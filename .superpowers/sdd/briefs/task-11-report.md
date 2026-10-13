# Task 11 Report: Virtual Line

## Status

**DONE**

## Summary

`line.Virtual` implements `Line` with `Ready() == true`, TCP control on `127.0.0.1:8742` (config/env override), optional stdin via `ACC_VIRTUAL_STDIN=1`, best-effort OS audio playback, and app wiring for `LineBackendVirtual`.

## Tests

```text
=== RUN   TestVirtualLine_TCPControl
--- PASS: TestVirtualLine_TCPControl
PASS
ok  	github.com/MRsagi/agents-call-center/internal/line
```

Full suite: `go test ./...` — pass.

## Files

| File | Change |
|------|--------|
| `internal/line/virtual.go` | Virtual line + control server |
| `internal/line/virtual_test.go` | TCP-driven Ring/Answer/Play/DTMF test |
| `internal/config/config.go` | `VirtualControlBind` default |
| `internal/app/app.go` | Construct virtual backend |
| `docs/virtual-line.md` | Manual testing notes |

## Commit

`feat: add Virtual Line for speaker/keyboard testing`

## Notes

- Idle DTMF (feature codes) delivered when line is idle and not collecting.
- Non-WAV synth bytes may not play on Windows `SoundPlayer`; playback failure is logged, not fatal.

## Fix pass

- `Virtual.Ring`: best-effort OS ring beep (async; does not fail Ring).
- `README.md`: Virtual line section + 3-bullet manual checklist → `docs/virtual-line.md`.
- `docs/virtual-line.md`: control plane documented as **TCP only** (no UDP).

Tests: `go test ./internal/line/ -v` PASS; `go test ./...` PASS.

Commit: `fix: Virtual Line ring beep and README checklist`
