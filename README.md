# Agents Call Center

Local service that connects software agents to one human over a phone line (SIP/PSTN or a virtual line for development). See [CONTEXT.md](CONTEXT.md) for domain vocabulary.

## Virtual line

Manual speaker/keyboard testing without SIP hardware. Full notes: [docs/virtual-line.md](docs/virtual-line.md).

Manual checklist:

- Run `acc serve` with the **virtual** line backend.
- On `127.0.0.1:8742` (TCP), send `ANSWER` when the log shows `RINGING`, then `DTMF` digits as needed.
- Confirm you hear the prompt audio (or a skip log if no player is installed).

## Focus target (OS)

`acc serve` uses the OS focuser to bring the agent app to the foreground and type DTMF digits into it after a call is answered.

**Linux:** OS focus is not supported (`FocusBackendOS` returns an error on focus/type). Use `focus_backend: fake` or run on macOS/Windows.

### macOS Accessibility (required for `acc serve`)

Grant **Accessibility** to the terminal or app that runs `acc` (System Settings → Privacy & Security → Accessibility). Without it, `osascript` / System Events keystrokes fail and the call completes with `delivery: focus_missed` (focus) or a failed status (typing).

Optional `focus_target.title_contains` narrows activation to a window whose title contains that substring. If no window matches, focus fails.

**Verify with a Virtual Call (manual checklist — run on macOS):**

1. Enable Accessibility for your terminal, then restart the terminal if macOS prompts you to.
2. Configure `focus_target` (app name and optional `title_contains`) for an app you can leave open in the foreground.
3. Run `acc serve` (virtual line). When the log shows `RINGING`, send `ANSWER` on `127.0.0.1:8742`.
4. Confirm the target app comes to the front (and the matching window, if `title_contains` is set).
5. Confirm DTMF digits from the virtual line appear in the focused app (or check logs for focus/type errors).

Step 3 of the OS focuser rollout is this checklist; automated GUI verification is not available on non-macOS CI hosts.
