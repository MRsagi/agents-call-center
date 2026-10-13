# Agents Call Center

Local service that connects software agents to one human over a phone line (SIP/PSTN or a virtual line for development). See [CONTEXT.md](CONTEXT.md) for domain vocabulary.

## Virtual line

Manual speaker/keyboard testing without SIP hardware. Full notes: [docs/virtual-line.md](docs/virtual-line.md).

Manual checklist:

- Run `acc serve` with the **virtual** line backend.
- On `127.0.0.1:8742` (TCP), send `ANSWER` when the log shows `RINGING`, then `DTMF` digits as needed.
- Confirm you hear the prompt audio (or a skip log if no player is installed).

## Focus target (macOS)

`acc serve` uses the OS focuser to bring the agent app to the foreground and type DTMF digits into it after a call is answered.

On **macOS**, grant **Accessibility** permission to the terminal or app that runs `acc` (System Settings → Privacy & Security → Accessibility). Without it, `osascript` / System Events keystrokes fail and the call completes with `delivery: focus_missed` (focus) or a failed status (typing).

Optional `focus_target.title_contains` narrows activation to a window whose title contains that substring.
