# Task 9 Report: CLI Surface

## Status

**DONE**

## Summary

Cobra-based `acc` binary with `serve` (App + HTTP, SIGINT/SIGTERM graceful shutdown), `call`, `status`, and `voicemail`. Client commands use `ACC_URL` (default `http://127.0.0.1:8741`) and JSON HTTP helpers in `internal/cli`.

## Tests

```text
go test ./internal/cli/ -v — PASS (TestClient_CallAndStatus)
go test ./... — PASS
```

## Files

| File | Purpose |
|------|---------|
| `cmd/acc/main.go` | Delegates to `cli.Execute()` |
| `internal/cli/root.go` | Root command wiring |
| `internal/cli/serve.go` | Daemon + HTTP lifecycle |
| `internal/cli/call.go` | POST `/v1/calls` |
| `internal/cli/status.go` | Health + queue JSON |
| `internal/cli/voicemail.go` | Voicemail summary |
| `internal/cli/client.go` | Shared HTTP client + `ACC_URL` |
| `internal/cli/cli_test.go` | In-process server + client exercise |

## Commit

- **SHA:** `1f35857`
- **Subject:** `feat: add acc CLI (serve, call, status, voicemail)`
- **Note:** `git commit -m` fails (`unknown option trailer`); used mingw64 `git-commit-tree` + `update-ref`.

## Concerns

- `serve` uses `config.Default()` only; no CLI flags for bind/data dir yet.
- Tray/MCP not wired (Tasks 10+).
- Client output is JSON; no human-formatted tables.
