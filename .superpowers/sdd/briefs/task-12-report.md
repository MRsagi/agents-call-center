# Task 12 Report: Status Tray

## Status

**DONE**

## Summary

Added `internal/tray` with `StatusSnapshot`, `StatusProvider`, `FormatTooltip` / `UpdateTooltip`, and `Run` (getlantern/systray on Windows and CGO desktop Unix; stub elsewhere). `acc serve` starts the tray in a goroutine unless `ACC_TRAY=0`; Quit cancels the serve context. `app.App.StatusSnapshot()` and `domain.Queue.LastFinishedSummary()` feed queue depth, voicemail count, line health, and last finished call.

## Tests

```text
go test ./internal/tray/ -v — PASS (FormatTooltip, UpdateTooltip, ACC_TRAY=0 no-op)
go test ./... — PASS
```

## Files

| File | Purpose |
|------|---------|
| `internal/tray/status.go` | Snapshot, formatting helpers |
| `internal/tray/tray.go` | `Run`, `ACC_TRAY` gate |
| `internal/tray/systray_enabled.go` | Systray loop, 1s poll, Quit |
| `internal/tray/systray_stub.go` | Headless / no-CGO builds |
| `internal/cli/serve.go` | Wire tray + shared shutdown context |
| `internal/cli/tray_serve.go` | Provider + `TrayQuit` |
| `internal/app/app.go` | `StatusSnapshot()` |
| `internal/domain/queue.go` | `LastFinishedSummary()` |

## Commit

- **SHA:** (see `git log -1` on branch)
- **Subject:** `feat: add Status Tray for queue and Line health`
- **Note:** `git commit -m` fails (`unknown option trailer`); used `git-commit-tree.exe` + `git update-ref`.

## Concerns

- Linux tray needs CGO + AppIndicator/Ayatana; CI uses stub unless tagged desktop build.
- Placeholder ICO may need replacement for crisp icons on HiDPI.
- Manual verify on Windows/macOS recommended; not run in this session.
