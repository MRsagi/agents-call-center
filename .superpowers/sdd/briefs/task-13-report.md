# Task 13 Report: OS Synthesizer

## Status
Complete.

## Changes
- Added `internal/synth/os.go` with `NewOS()` dispatching on `runtime.GOOS`:
  - **darwin:** `say -o` → AIFF bytes
  - **windows:** PowerShell `System.Speech.Synthesis` → WAV bytes
  - **linux:** `espeak-ng -w` (fallback `espeak`) → WAV bytes
- Added `config.SynthBackend` (`fake` | `os`); default remains `fake`.
- `app.NewWithAdapters` selects synth from config when no adapter injected.
- `acc serve` sets `SynthBackend=os` so production uses host TTS.

## Tests
- `TestNewOS_ImplementsSynthesizer` — interface smoke (CI-safe).
- `TestNew_DefaultSynthIsFake` — runner/API tests keep fake default via `config.Default()`.
- `os_manual_test.go` (`//go:build manual`) — optional end-to-end TTS smoke.

## Concerns
- Windows/Linux virtual line playback prefers WAV; darwin accepts AIFF via `afplay`.
- PowerShell TTS escapes single quotes only; exotic prompts may need hardening later.
- Unsupported GOOS returns error from `Synthesize`, not at `NewOS()`.

## Verification
```bash
go test ./...
```
