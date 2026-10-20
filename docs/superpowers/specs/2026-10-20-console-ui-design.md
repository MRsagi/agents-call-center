# Agents Call Center — Console UI Design

**Date:** 2026-10-20  
**Status:** Approved for implementation planning  
**Domain language:** [`CONTEXT.md`](../../../CONTEXT.md)  
**Parent design:** [`2026-10-13-agents-call-center-design.md`](./2026-10-13-agents-call-center-design.md)

## Goal

Give the Human a friendly **Console** for day-to-day use of the Call Center: live Call text, a readable Voicemail list, and full configuration — while keeping the **Status Tray** as the always-on glance and Quit control. Also add **Idle Dictation** via Feature Code so the handset can drive OS dictation in the already-focused app (e.g. Cursor Ctrl+M).

## Non-goals

- Native webview wrapper (Wails / similar) — optional later; UI API stays wrap-friendly
- Linux Console polish (Status Tray + CLI only on Linux for this drop)
- Auto-start dictation on bare idle pickup
- PC-speaker Voicemail playback
- Auth tokens or non-localhost bind
- Preempting Idle Dictation when an Agent Call rings (Agent Calls wait)
- Changing Dialplan B (“pickup focuses last Agent”) from the original roadmap

## Decisions (locked)

| Topic | Choice |
|---|---|
| Shell | Hybrid: Status Tray + **Open Console** |
| Console delivery | Browser UI served by `acc serve` at `/console` |
| Platforms | Windows + macOS Console; Linux tray/CLI only |
| Config scope | Full runtime mirror, including secrets in the form |
| Config apply | Save → write file → daemon auto-restart → UI reconnects |
| Voicemail Console | Browse full Prompt text + delete / clear all; audio still via phone `*1` |
| Live Call view | Prompt + state + recent in-memory Call history (~20) |
| Idle Dictation start | Feature Code only (default `*2`), not bare pickup |
| Idle Dictation focus | Hotkey to **current foreground** app — no focus steal |
| Idle Dictation end | Hang up only |
| Agent vs dictation | Ringing Call does **not** interrupt Idle Dictation |

## Architecture

```
Agents → Surfaces (HTTP / MCP / CLI)
              ↓
         Call Center core (Queue, Dialplan, Voicemail, Runner)
              ↓
    Line / Synth / Focus / Phone Mic
              ↑
Status Tray ──Open Console──► Browser → /console (static UI)
                              ↕ REST + SSE
                         Config file (data dir) → save → auto-restart
```

**Approach:** tray + external browser Console. Same localhost trust model as today’s API. Static assets embedded or served from the daemon; live data over REST + Server-Sent Events (SSE), with 1s poll fallback.

### Status Tray

- Keep existing glance: Line up/down, queue depth, voicemail count, last Call summary
- Add menu item **Open Console** → open default browser to `http://127.0.0.1:<http-port>/console`
- **Quit** stops the daemon (unchanged)
- Linux: omit Open Console (or show unsupported); tray otherwise unchanged

### Console

Local web UI, three pages:

1. **Call** — live Prompt/state + Idle Dictation banner + recent history  
2. **Voicemail** — list, read full text, delete / clear  
3. **Config** — full editable form  

Nav: **Call | Voicemail | Config**.

## Console pages (detail)

### Call

- Large Prompt text for the active or ringing Call; idle copy: “No active call”
- Status: queued / ringing / in_progress / idle
- Meta: queue depth, Focus Target, digits collected, delivery/error
- When Idle Dictation active: banner “Dictating to foreground app — hang up to stop”
- Recent Calls (last ~20, in-memory only): prompt snippet, status, timestamp — lost on restart

### Voicemail

- List unheard entries: created time, full Prompt (expandable), Focus Target, call id
- Actions: delete one, clear all
- No PC playback; phone Feature Code (default `*1`) remains the audio path
- Deleting from Console removes the JSONL entry (same end state as “heard”)

### Config

- Editable fields mirror full runtime config: Line/synth/focus backends, HTTP and virtual-control binds, SIP settings (including password), Phone Mic, Feature Codes (Voicemail + Idle Dictation), dictation hotkey, ring/recover timeouts, data dir, tray enable
- UI warns that secrets are visible and any local process can read localhost
- **Save** validates → writes config file under data dir → triggers daemon auto-restart → UI shows “Reconnecting…” until health succeeds
- Load order: config file, then `ACC_*` env overrides (env always wins for that process). Console GET returns effective (post-merge) config and marks env-overridden fields so the Human knows Save to file won’t stick until that env var is unset

## Dialplan: Idle Dictation

| Situation | Behavior |
|---|---|
| Idle off-hook | Dial tone only |
| Idle + Voicemail Feature Code (default `*1`) | Play oldest unheard Voicemail (unchanged) |
| Idle + Idle Dictation Feature Code (default `*2`) | Start Idle Dictation: ensure Phone Mic path if enabled; send dictation hotkey to current foreground window; set dictating state for Console/SSE |
| Hang up during Idle Dictation | Stop Phone Mic tee for the session; send hotkey again (best-effort toggle off); clear dictating state |
| Agent Call rings during Idle Dictation | Do **not** interrupt; Call remains queued until Line is free |
| Other idle digits | Error tone / ignore |

**New config:** Idle Dictation Feature Code (default `*2`); dictation hotkey string (default `ctrl+m` on Windows and macOS — mapped to the OS key event by the Focuser). If Phone Mic is off or sox/device missing, still send the hotkey and warn in Console.

## Data flow & API

### Config file

- Path: `{DataDir}/config.json`
- Load on startup; `ACC_*` env overrides applied after file load
- `PUT /v1/config` writes file then performs an in-process serve restart (tear down HTTP/Line/tray listeners, reload config, bring them back up in the same OS process) so binds/backends apply without requiring a manual relaunch

### HTTP (localhost, no auth)

| Method | Path | Role |
|---|---|---|
| GET | `/console` (+ static assets) | Console UI |
| GET | `/v1/config` | Read effective config (post-env-merge) |
| PUT | `/v1/config` | Validate, write file, restart |
| GET | `/v1/voicemail` | List unheard (existing) |
| DELETE | `/v1/voicemail/{id}` | Delete one entry |
| DELETE | `/v1/voicemail` | Clear all |
| GET | `/v1/status` | Line up, queue depth, active Call, Idle Dictation flag, recent Calls |
| GET | `/v1/events` | SSE: Call transitions, voicemail count, dictation start/stop, restart-soon |
| GET | `/v1/health` | Existing; used for reconnect |

Existing call/queue endpoints unchanged.

### Live UI

- Call page prefers SSE; polls `/v1/status` every 1s if SSE unavailable
- After config restart: reconnect loop on `/v1/health`

## Errors & permissions

| Failure | Behavior |
|---|---|
| Browser open fails | Log + tray-accessible URL for manual open |
| Invalid config on save | HTTP 400 with field errors; no restart |
| Restart / bind failure | Loud log and exit the process with a non-zero code after failed re-bind so the tray disappears rather than hanging in a half-dead state; Human relaunches `acc serve` |
| Hotkey send fails | Console warning; Phone Mic may still run |
| Phone Mic unavailable on `*2` | Hotkey still sent; Console warns mic inactive |
| Delete Voicemail during phone `*1` play | Allowed; phone path may skip/no-op — prefer Console delete when Line idle |
| Linux Open Console | Omit menu item or show unsupported page |

**Trust model:** unchanged — localhost only, no auth.

## Testing

| Level | Scope |
|---|---|
| Unit | Dialplan Idle Dictation match; config load/save validation; voicemail DELETE |
| API | Config PUT (+ restart hook testable via fake), voicemail delete/clear, status/recent, SSE smoke |
| Manual | Windows + macOS: Open Console, live Call text, voicemail delete, config restart, `*2` + hotkey + Phone Mic |
| CI | Unit + Fake Line; no browser E2E required for v1 |

## Platform matrix (this drop)

| Capability | Windows | macOS | Linux |
|---|---|---|---|
| Status Tray | ✓ | ✓ | ✓ |
| Console UI | ✓ | ✓ | — |
| Idle Dictation Feature Code | ✓ | ✓ | ✓ (hotkey/Phone Mic best-effort) |
| Config file + auto-restart | ✓ | ✓ | ✓ |

## Relationship to prior design

Supersedes Status Tray “menu-only” UX with Console for Human-facing management. Extends Dialplan with Idle Dictation Feature Code (replaces the vague “later N* dictate” milestone for the desk workflow chosen here). Does not implement auto-focus-on-pickup.
