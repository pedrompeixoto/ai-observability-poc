// Minimal relay: Claude Code OTLP logs (HTTP/JSON) -> Server-Sent Events -> browser.
package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
)

// --- OTLP JSON (only the fields we need) ---

type kv struct {
	Key   string         `json:"key"`
	Value map[string]any `json:"value"` // e.g. {"stringValue":"x"} or {"intValue":"3"}
}

type otlpLogs struct {
	ResourceLogs []struct {
		ScopeLogs []struct {
			LogRecords []struct {
				TimeUnixNano string         `json:"timeUnixNano"`
				Body         map[string]any `json:"body"`
				Attributes   []kv           `json:"attributes"`
			} `json:"logRecords"`
		} `json:"scopeLogs"`
	} `json:"resourceLogs"`
}

// Event is what we push to the dashboard.
type Event struct {
	Time  string         `json:"time"`
	Name  string         `json:"name"`
	Attrs map[string]any `json:"attrs"`
}

// --- SSE fan-out ---

var (
	mu      sync.Mutex
	clients = map[chan Event]struct{}{}
)

// broadcast never blocks: a slow dashboard drops events instead of
// stalling the OTLP response to Claude Code.
func broadcast(e Event) {
	mu.Lock()
	defer mu.Unlock()
	for ch := range clients {
		select {
		case ch <- e:
		default:
		}
	}
}

func eventsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	rc := http.NewResponseController(w)

	ch := make(chan Event, 64)
	mu.Lock()
	clients[ch] = struct{}{}
	mu.Unlock()
	defer func() {
		mu.Lock()
		delete(clients, ch)
		mu.Unlock()
		log.Println("dashboard disconnected")
	}()

	if err := rc.Flush(); err != nil {
		return
	}
	log.Println("dashboard connected")

	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			b, err := json.Marshal(e)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", b)
			if err := rc.Flush(); err != nil {
				return
			}
		}
	}
}

// --- OTLP receiver ---

func firstValue(v map[string]any) any {
	for _, x := range v {
		return x
	}
	return nil
}

func logsHandler(w http.ResponseWriter, r *http.Request) {
	var body io.Reader = r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		body = gz
	}

	var req otlpLogs
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		http.Error(w, "expected OTLP JSON: "+err.Error(), 400)
		return
	}

	for _, rl := range req.ResourceLogs {
		for _, sl := range rl.ScopeLogs {
			for _, lr := range sl.LogRecords {
				e := Event{Time: lr.TimeUnixNano, Attrs: map[string]any{}}
				for _, a := range lr.Attributes {
					e.Attrs[a.Key] = firstValue(a.Value)
				}
				e.Name, _ = firstValue(lr.Body).(string) // e.g. "claude_code.tool_result"
				log.Println("event:", e.Name)
				broadcast(e)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("{}"))
}

func main() {
	http.HandleFunc("/v1/logs", logsHandler)
	http.HandleFunc("/events", eventsHandler)
	http.Handle("/", http.FileServer(http.Dir("web")))

	log.Println("relay on http://localhost:4318  (OTLP: /v1/logs, SSE: /events)")
	log.Fatal(http.ListenAndServe(":4318", nil))
}
