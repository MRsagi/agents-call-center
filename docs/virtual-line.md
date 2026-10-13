# Virtual line (manual testing)

Use the **virtual** line backend to exercise real call flow without SIP hardware.

## Enable

Set `LineBackend` to `virtual` in app config (default control bind `127.0.0.1:8742`).

Environment:

| Variable | Meaning |
|----------|---------|
| `ACC_VIRTUAL_CONTROL_ADDR` | TCP bind address (default `127.0.0.1:8742`) |
| `ACC_VIRTUAL_STDIN=1` | Answer on Enter; type `0`–`9`, `*`, `#` for DTMF |

## Control commands (TCP, one line each)

The control plane is **TCP only** (plain text, one command per line). There is no UDP control port.

Connect with any TCP client (e.g. `nc 127.0.0.1 8742` or PowerShell `Test-NetConnection` + a TCP client):

- `ANSWER` — pick up the ringing call
- `DTMF 1` — send digit(s); `DTMF 12` sends `12`
- `HANGUP` — end the call

While a call is **ringing**, send `ANSWER`. After prompt playback, send `DTMF` for digit collection.

## Audio

`Play` writes synthesized audio to a temp file and tries `afplay` (macOS), `ffplay`, or PowerShell `SoundPlayer`. If none are installed, playback is skipped and a log line is emitted (no error).

## Manual checklist

1. Start `acc serve` with virtual line backend.
2. Create a call via API/CLI; confirm log shows `RINGING`.
3. Send `ANSWER` on the control port; call proceeds.
4. Send `DTMF` digits; confirm they appear on the completed call.
5. Optional: set `ACC_VIRTUAL_STDIN=1` and repeat using keyboard.
