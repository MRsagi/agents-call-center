# Task 14 Report: OS Focuser

## Status
Complete.

## Changes
- Added `internal/focus/os.go` with `NewOS()` delegating to build-tagged `newOSImpl()`.
- **darwin:** `osascript` activates `focus_target.app`; optional `title_contains` raises matching window via System Events; `Type` sends keystrokes / key code 36 for Enter.
- **windows:** best-effort `WScript.Shell` `AppActivate` and `SendKeys`.
- **other:** returns `focuser not supported on this OS`.
- Config `FocusBackend` (`fake`|`os`); default `fake`; `acc serve` sets `os` (mirrors `SynthBackend`).
- README documents macOS Accessibility permission.

## Tests
- `go test ./...` (unit tests use `focus.Fake`; `TestNewOS_*` asserts non-nil Focuser).

## Manual
Not run in this session (requires answered virtual call + Accessibility on macOS).

## Concerns
- Windows `AppActivate` matches process or title loosely; may need HWND-based focus later.
- Linux has no focuser yet (`FocusBackendOS` will error on focus/type).
