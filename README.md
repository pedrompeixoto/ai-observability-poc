# Claude Code → Go relay → Server-Sent Events → browser

> [!WARNING]
> **Proof of concept, 100% vibe coded.** Built entirely with AI, just to learn how Claude Code's
> OpenTelemetry export works. It is not production software: no auth, no tests, no persistence,
> and minimal error handling. Run it only on a trusted machine.

Claude Code's built-in OpenTelemetry **log events** (`user_prompt`, `api_request`,
`tool_result`, ...) are exported as OTLP HTTP/JSON to a tiny Go relay, which pushes
each event over Server-Sent Events to a single HTML page.

```
claude ──OTLP/HTTP JSON──▶ relay :4318/v1/logs ──SSE /events──▶ web/index.html
```

## Run

```sh
go run .                      # relay + dashboard on http://localhost:4318
open http://localhost:4318
```

Then start Claude Code **from this folder**:

```sh
claude
```

The telemetry config lives only in `.claude/settings.json` (project scope).
Claude Code doesn't apply telemetry env vars from project settings until you approve them;
on first launch it shows *"This project's settings set telemetry environment variables. Run /status to…"*.
Run `/status`, approve, and restart `claude`.

Alternative (no approval needed, nothing global changed) — set the vars just for one run:

```sh
CLAUDE_CODE_ENABLE_TELEMETRY=1 OTEL_LOGS_EXPORTER=otlp \
OTEL_EXPORTER_OTLP_PROTOCOL=http/json OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 \
OTEL_LOGS_EXPORT_INTERVAL=1000 OTEL_LOG_USER_PROMPTS=1 OTEL_LOG_TOOL_DETAILS=1 claude
```

### Seeing `<REDACTED>`?

By default Claude Code redacts prompt and response text. `OTEL_LOG_USER_PROMPTS=1` and
`OTEL_LOG_TOOL_DETAILS=1` turn that off (prompt text, response text, tool parameters).
If they're in `.claude/settings.json` but you still see redacted values, the project's
telemetry vars haven't been approved yet (see `/status` above) or `claude` wasn't restarted.

## Files

- `main.go` — relay (standard library only)
- `web/index.html` — dashboard (plain JS, no build)
- `.claude/settings.json` — project-local telemetry env

## POC limitations

- **No auth, all interfaces.** The relay listens on `:4318` on every network interface, and anyone
  who can reach it can post to `/v1/logs` or read the `/events` stream (which can include prompt text).
- **Logs only.** OTLP metrics and traces are ignored; token and cost stats come from fields inside log events.
- **In-memory only.** Nothing is stored; events reach only browsers connected at the time.
- **`.claude/settings.json` is currently empty (`{}`).** Until the telemetry env is added there,
  use the one-run env-var command above.
- No tests, CI, or packaging.
