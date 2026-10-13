# Task 8 Report: Config + HTTP API Surface

## Status

**DONE**

## Summary

Extended `config.Config` with HTTP bind (`127.0.0.1:8741`), data dir, feature code, and line backend. `app.App` wires Queue, Runner, Voicemail, and adapters with `Start`/`Stop`. `api.Server` exposes localhost-only `/v1/calls`, `/v1/queue`, `/v1/voicemail`, and `/v1/health`.

## Tests

```text
go test ./internal/api/ -v — PASS (2 tests)
go test ./... — PASS
```

## Files

| File | Purpose |
|------|---------|
| `internal/config/config.go` | Bind, DataDir, FeatureCode, LineBackend defaults |
| `internal/app/app.go` | Core wiring, runner lifecycle |
| `internal/api/server.go` | HTTP handlers + localhost bind check |
| `internal/api/server_test.go` | POST + poll GET; health/voicemail |

## Commit

- **SHA:** `ddc0b43`
- **Subject:** `feat: add localhost HTTP API for enqueue and status`
- **Note:** `git commit -m` fails (`unknown option trailer`); used `git-commit-tree.exe` + `git-update-ref.exe`.

## Concerns

- SIP/Virtual line backends return error from `app.New` until implemented.
- `ListenAndServe` validates localhost; `httptest` bypasses bind (tests use `Handler()` only).
- No graceful HTTP shutdown yet (Task 9 `acc serve` may add it).
