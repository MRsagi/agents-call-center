# Task 16 Report: Phone Mic (macOS + BlackHole)

## Status
Complete.

## Changes
- New `internal/phonemic`: `PhoneMic` interface, `Fake`, darwin sox→CoreAudio pipeline, `!darwin` stub.
- Config: `PhoneMicEnabled`, `PhoneMicDevice` (default `BlackHole 2ch`); env `ACC_PHONE_MIC_*`.
- `internal/app`: construct/start/stop Phone Mic; SIP `SetRXPCMHook` when enabled.
- SIP RTP: inbound μ-law decode + RX loop tees PCM to Phone Mic after answer.
- `docs/phone-mic.md` + README link (BlackHole + sox install).

## Tests
- `go test ./...` — pass.
- `phonemic` Fake + stub New; `sip` μ-law roundtrip test.

## Manual
Not run (Windows CI host; BlackHole/sox macOS only).

## Concerns
- Virtual Line has no RX PCM path (documented).
- macOS without sox: Phone Mic logs and no-ops (no BlackHole output).
- μ-law decode is best-effort; no AEC/level normalization.
- Core Audio device name must match `ACC_PHONE_MIC_DEVICE` exactly.

## Fix pass (RX tee lifetime)
- SIP `startRXPCM` uses call-scoped `context.Background()` cancel on `rtpSender.close()` (not `ringCtx`).
- Test `TestStartRXPCMSurvivesRingContextCancel`; removed unused `ErrToolsMissing`; log when Phone Mic on non-SIP line.
- `go test ./internal/line/sip/ ./internal/phonemic/ ./...` — pass.
