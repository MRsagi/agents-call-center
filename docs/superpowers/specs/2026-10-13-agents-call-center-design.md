# Agents Call Center — Design

**Date:** 2026-10-13  
**Status:** Approved for implementation planning  
**Domain language:** [`CONTEXT.md`](../../../CONTEXT.md)

## Goal

A personal **Call Center** on the Human’s PC: software **Agents** (Cursor hooks, bots, MCP clients, scripts) enqueue a **Call** that rings a landline (via LINKSYS PAP2T SIP/FXS) or a **Virtual Line**, speaks a **Prompt**, and delivers the Human’s **Reply** into a specific IDE/chat **Focus Target** as keystrokes. Agents never block waiting for the phone.

## Non-goals (v1)

- Multi-Human / contact-center queues, CRM, agent seats
- Blocking RPC-style “wait for digits on the HTTP response”
- `speech` Call Mode (API may name it; not implemented)
- Cloud TTS, hosted SIP trunks as the primary path
- Asterisk/FreeSWITCH sidecar
- Phone Mic on Windows/Linux
- Dialplan milestones B/C (pickup focuses last chat; `N*` chat bank + dictate)
- Auth tokens or LAN bind (localhost only, no auth)
- Priority / barge-in on the Queue

## Architecture

**Approach:** single Go daemon (`acc serve`) — one process for core + adapters + Status Tray.

```
Agents → Surfaces (HTTP / MCP / CLI)
              ↓ enqueue (returns Call id immediately)
           Queue (FIFO, one active Call)
              ↓
         Call runner ──► Synthesizer (OS TTS)
                      ──► Line (SIP | Virtual | Fake)
                      ──► Focuser (OS focus + keystrokes)
              ↓ (no answer)
         Voicemail JSONL
```

| Layer | Responsibility |
|---|---|
| Surfaces | Local HTTP, MCP, CLI |
| Core | Queue, Call state machine, Dialplan, Voicemail |
| Adapters | Line, Synthesizer, Focuser, Phone Mic (macOS) |
| Status Tray | Native systray: Line health, Queue depth, last Call, Voicemail count, Quit |

**Config** (file + env): SIP registrar settings for PAP2T, active Line backend, ring timeout (default ~90s), Voicemail Feature Code (default `*1`), data directory. Bind `127.0.0.1` only.

**Binary UX:** `acc serve` runs daemon + tray; `acc call …` and other commands talk to local HTTP; quitting the tray stops the daemon.

## Domain model (summary)

See `CONTEXT.md` for canonical terms. Key behaviors:

- **Call** — async work item; Surface returns `{id}` immediately
- **Queue** — strict FIFO; one Line
- **Call Mode** — API: `notify` \| `dtmf` \| `speech`; **v1 implements `dtmf` only**
- **Reply (dtmf)** — digits typed into Focus Target, then **Enter**
- **Focus Target** — structured identity supplied by the Agent/hook (application + window/chat reference). Exact fields follow what Cursor (or other hosts) can expose; unresolved targets do not abort the audio Call
- **Voicemail** — no answer within ring timeout; unheard entries persist as JSONL; deleted once heard; accessed only via Feature Code on idle Line
- **Phone Mic** — macOS virtual microphone fed from off-hook Line audio (for system dictation); not wired to Dialplan B/C yet

## Call lifecycle

**Payload:** `id`, `prompt`, `mode`, `max_digits` (default 1), `focus_target`, `created_at`, `status`.

**Statuses:** `queued` → `ringing` → `in_progress` → `completed` \| `voicemail` \| `failed`.

**Runner (one Call at a time):**

1. Synthesize Prompt → audio; on TTS failure → `failed`
2. `Line.Ring`; start ring timer
3. On answer: `Focuser.Focus(focus_target)`; play Prompt; collect DTMF up to `max_digits`
4. `Focuser.Type(digits + Enter)`; status `completed` (if focus/type failed: still `completed` with `delivery=focus_missed`)
5. On ring timeout: append Voicemail JSONL; status `voicemail`; start next queued Call

## Dialplan (v1)

| Situation | Behavior |
|---|---|
| Idle off-hook | Dial tone only — no auto Voicemail, no auto focus |
| Idle + Voicemail Feature Code (`*1`) | Play oldest unheard Voicemail; DTMF still delivers to that Call’s Focus Target; delete when finished; if empty, “no messages” |
| Other idle digits | Error tone / ignore |
| In-Call DTMF | Collected as Reply for the active Call |

**Later (not v1):** pickup focuses last Agent chat; `N*` focuses chat N and starts Phone Mic dictation.

## Line backends

Shared interface: `Ring`, `Play`, `CollectDigits`, `Hangup`, idle digit handling.

| Backend | Use |
|---|---|
| **SIP** | Embedded SIP UA/registrar; PAP2T registers to the daemon |
| **Virtual Line** | Local speakers + keyboard as DTMF — manual testing |
| **Fake Line** | Scripted prompts/replies — unit/CI |

## Surfaces (v1)

All three, thin over the same core:

- **HTTP** — e.g. `POST /v1/calls` → `{id}`; `GET /v1/calls/{id}`; `GET /v1/queue`; Voicemail inspection helpers for CLI/tray
- **CLI** — `acc serve`, `acc call`, `acc status`, …
- **MCP** — tools mirroring enqueue + status (no blocking wait-for-reply tool as the primary UX)

## Status Tray

Native Go systray menu: Line up/down, Queue depth, last Call summary, unheard Voicemail count, Quit.

## Persistence

| Data | Store |
|---|---|
| Active/recent Call history | In-memory only (lost on restart) |
| Unheard Voicemail | Append-only JSONL under app data dir; remove entry when heard |

## Errors & permissions

| Failure | Behavior |
|---|---|
| Line down / SIP register fail | Tray shows Line down; runner waits briefly for Line recovery, then marks Call `failed` if still down |
| TTS fail | `failed`; do not ring |
| Focus unresolved / Accessibility denied | Audio Call continues; skip keystrokes; `delivery=focus_missed`; tray warns once |
| Daemon not running | CLI/MCP instruct to run `acc serve` |

**macOS permissions:** Accessibility (Focuser); audio as needed for Virtual Line / Phone Mic; local network to PAP2T.

**Trust model:** localhost, no auth — personal desk tool.

## Phone Mic (macOS v1)

When enabled and Line is off-hook, feed handset/Virtual audio into an OS virtual microphone so apps (e.g. Cursor dictation) can use the phone as mic input. Independent of Call runner Dialplan B/C. Absent on Windows/Linux in v1 (Calls still work).

## Testing

| Level | Scope |
|---|---|
| Unit | Queue, state machine, Feature Codes, Voicemail JSONL, keystroke recipe |
| Integration | HTTP + Fake Line + fake Focuser |
| Manual | Virtual Line; then PAP2T + landline |
| CI | Unit + Fake Line only |

## Platform matrix (v1)

| Capability | macOS | Windows | Linux |
|---|---|---|---|
| Daemon, Queue, Surfaces | ✓ | ✓ | ✓ |
| SIP Line (PAP2T) | ✓ | ✓ | ✓ |
| Virtual / Fake Line | ✓ | ✓ | ✓ |
| OS TTS Synthesizer | ✓ | ✓ | ✓ |
| Status Tray | ✓ | ✓ | ✓ |
| Focuser | ✓ (primary) | best-effort | best-effort |
| Phone Mic | ✓ | — | — |

## Tech choices

| Choice | Decision |
|---|---|
| Language | Go |
| Process model | Single daemon |
| SIP | Embedded in-process (swappable behind `Line`) |
| TTS | OS TTS via `Synthesizer`; Piper-class engines later |
| Tray | Native Go systray |
| MCP / HTTP / CLI | In-process Surfaces |

## Milestone hints (post-v1)

1. Dialplan B — idle pickup focuses last Focus Target  
2. Dialplan C — `N*` chat bank + Phone Mic dictate  
3. `speech` Call Mode  
4. Phone Mic on Windows/Linux  
5. Optional neural TTS; Asterisk Line backend only if embedded SIP proves insufficient  

## Open implementation details (non-blocking)

These do not change the product contract; resolve during implementation planning:

- Concrete `focus_target` JSON schema once Cursor hook capabilities are probed
- Exact SIP stack library within Go
- macOS virtual-mic mechanism (driver vs supported user-space approach)
- Ring timeout and Feature Code as config defaults (`90s`, `*1`)
