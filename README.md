# Agents Call Center

**Agent → You:** software agents (Cursor hooks, MCP clients, scripts) enqueue a **Call** that rings your phone, speaks a **Prompt**, and types your DTMF **Reply** into a chosen app window. The agent gets a call id immediately and never blocks on the handset.

See [CONTEXT.md](CONTEXT.md) for domain vocabulary and [docs/superpowers/specs/2026-10-13-agents-call-center-design.md](docs/superpowers/specs/2026-10-13-agents-call-center-design.md) for architecture.

## Install

Requires Go 1.25+ (see `go.mod`).

```bash
go install github.com/MRsagi/agents-call-center/cmd/acc@latest
```

Or build from a clone:

```bash
git clone https://github.com/MRsagi/agents-call-center.git
cd agents-call-center
make build    # or: go build -o acc ./cmd/acc
```

Run tests: `make test` or `go test ./...` (uses the **fake** line backend in unit tests).

## Quick start

1. Start the daemon (HTTP API + tray + line):

   ```bash
   acc serve
   ```

   Default API: `http://127.0.0.1:8741`. Set `ACC_LINE_BACKEND=virtual` or `sip` for real ringing (see below).

2. Enqueue a call from another terminal or a hook:

   ```bash
   acc call --prompt "Ship the fix?" --app Cursor --digits 1
   ```

   Output: `{"id":"<uuid>"}`. Check status with `acc status` or MCP `get_call`.

3. **Cursor hook (shell):** after `acc serve` is running, point a hook or automation at:

   ```bash
   acc call --prompt "Need your input" --app Cursor --title "agents-call-center"
   ```

4. **Cursor MCP:** add a stdio server that proxies to the local API (daemon must be running):

   ```json
   {
     "mcpServers": {
       "agents-call-center": {
         "command": "acc",
         "args": ["mcp"]
       }
     }
   }
   ```

   Tools: `place_call`, `get_call`, `get_queue`, `get_voicemail`. Override URL with `ACC_URL` or `acc mcp --url http://127.0.0.1:8741`.

## Trust model

- HTTP and virtual-line control bind to **localhost** only (`127.0.0.1:8741`, `127.0.0.1:8742` by default). There is **no authentication** — any local process can enqueue calls or read status.
- Intended for a single user on a trusted PC. Do not expose these ports to the LAN or internet without adding your own auth and TLS.

## PAP2T / embedded SIP line

Hardware ATA on the LAN (e.g. Linksys PAP2T). Full settings: [docs/pap2t.md](docs/pap2t.md).

Checklist:

- Set `ACC_LINE_BACKEND=sip` (and optional `ACC_SIP_*` vars) then run `acc serve`.
- Point the ATA proxy at the PC’s LAN IP, UDP 5060; open firewall for SIP + RTP.
- User ID / Auth ID `100`, password `acc` (defaults; match `ACC_SIP_EXTENSION` / `ACC_SIP_PASSWORD`).
- Codec G.711 μ-law; DTMF **SIP INFO** (see pap2t doc if digits are missed).
- Tray **Line up** after REGISTER; enqueue a call and confirm ring, audio, DTMF, hangup.

## Virtual line

Manual speaker/keyboard testing without SIP hardware. Full notes: [docs/virtual-line.md](docs/virtual-line.md).

```bash
ACC_LINE_BACKEND=virtual acc serve
```

On `127.0.0.1:8742` (TCP), send `ANSWER` when logs show `RINGING`, then `DTMF` digits as needed. Confirm prompt audio (or a skip log if no player is installed).

## Voicemail (`*1`)

If a call is not answered within the ring timeout (~90s), the prompt is stored as voicemail (JSONL under the data dir). While the line is **idle**, dial feature code **`*1`** (config default) to hear the oldest unheard message; DTMF still delivers to that call’s focus target. CLI summary: `acc voicemail`.

**v1 idle `*1` on SIP:** DTMF must use **SIP INFO** (see [docs/pap2t.md](docs/pap2t.md)). Digits are aggregated with a ~1.5s inter-digit timeout so `*` and `1` sent one-at-a-time still match. Picking up the handset without an active call may not deliver INFO on some ATAs — use **`acc voicemail`** as a reliable fallback.

## Phone Mic (macOS + BlackHole)

Route inbound SIP handset audio into a virtual mic for system dictation (BlackHole + sox). See [docs/phone-mic.md](docs/phone-mic.md).

## Focus target (OS)

`acc serve` uses the OS focuser to bring the agent app to the foreground and type DTMF digits into it after a call is answered.

**Linux:** OS focus is not supported (`FocusBackendOS` returns an error on focus/type). Use `focus_backend: fake` or run on macOS/Windows.

### macOS Accessibility (required for `acc serve`)

Grant **Accessibility** to the terminal or app that runs `acc` (System Settings → Privacy & Security → Accessibility). Without it, `osascript` / System Events keystrokes fail and the call completes with `delivery: focus_missed` (focus) or a failed status (typing).

Optional `focus_target.title_contains` narrows activation to a window whose title contains that substring. If no window matches, focus fails.

**Verify with a virtual call (manual checklist — macOS):**

1. Enable Accessibility for your terminal, then restart the terminal if macOS prompts you to.
2. Configure `focus_target` (app name and optional `title_contains`) for an app you can leave open in the foreground.
3. Run `ACC_LINE_BACKEND=virtual acc serve`. When the log shows `RINGING`, send `ANSWER` on `127.0.0.1:8742`.
4. Confirm the target app comes to the front (and the matching window, if `title_contains` is set).
5. Confirm DTMF digits from the virtual line appear in the focused app (or check logs for focus/type errors).

Automated GUI verification is not available on non-macOS CI hosts.
