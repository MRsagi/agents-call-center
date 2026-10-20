# Agents Call Center

A personal channel that lets software agents reach a human over a phone line to deliver messages and collect replies.

## Language

**Call Center**:
The local service that connects agents to one human over SIP/PSTN (or a virtual phone). Not a multi-agent contact center for customers.
_Avoid_: Contact center, help desk, support queue

**Human**:
The single person who answers the phone and responds by speech or keypad.
_Avoid_: Operator, agent (for the person), customer, user (when contrasting with software agents)

**Agent**:
A software actor (IDE hook, bot, script, MCP client) that requests a Call to the Human.
_Avoid_: Bot (as the only term), client, caller (ambiguous with the Human)

**Call**:
One queued interaction requested by an Agent: the Surface enqueues it and returns immediately; the Call Center later rings the Line, speaks the Prompt, and delivers the Human’s Reply into the Focus Target.
_Avoid_: Session, conversation, job (when meaning the phone interaction), RPC (Calls are not blocking request/response)

**Queue**:
The ordered waiting line of Calls for the single phone line. Strict FIFO — no barge-in or priority in the core model.
_Avoid_: Priority queue, stack

**Call Mode**:
How a Call expects the Human to respond: `notify` (message only), `dtmf` (keypad digits), or `speech` (spoken reply). The API names all three; the first shipped implementation is `dtmf` only.
_Avoid_: Interaction type, prompt type

**Reply**:
The Human’s answer to a Call — for `dtmf`, the collected digits, typed into the Focus Target followed by Enter; later, transcribed speech for `speech`.
_Avoid_: Response (when meaning the phone answer), input, HTTP return value

**Focus Target**:
The window/tab/chat identity an Agent supplies when enqueueing a Call — used on answer to bring that context forward before delivering the Reply.
_Avoid_: Callback URL (unless literally used), window handle alone (too low-level as the domain name)

**Phone Mic**:
A virtual microphone exposed to the OS whose audio source is the handset (or virtual phone) when off-hook. Apps such as Cursor use it as a normal input device (e.g. system dictation).
_Avoid_: Inbound prompt, softphone mic, app bridge

**Surface**:
A way an Agent submits or observes Calls: HTTP API, CLI, or MCP. All Surfaces talk to the same Call Center core.
_Avoid_: Frontend, integration, binding

**Line**:
The phone endpoint the Call Center rings and talks to — physical (SIP/ATA) or Virtual. Exactly one Line is active for the Queue.
_Avoid_: Channel, trunk, extension (unless meaning SIP config)

**Virtual Line**:
A software Line that plays audio locally and accepts keyboard (as DTMF) instead of a handset — for manual testing without hardware.
_Avoid_: Softphone (as the product name), emulator

**Fake Line**:
A non-audio Line that records prompts and returns scripted Replies — for automated tests.
_Avoid_: Mock (as the domain name), stub Line

**Status Tray**:
A small always-available UI showing Line health, Queue depth, and the latest Call, plus Open Console and Quit — not a full CRM.
_Avoid_: Dashboard, admin UI, control panel (use Console for those Human-facing pages)

**Console**:
The local web UI for the Human: live Call text, Voicemail list, and configuration. Opened from the Status Tray into the default browser; served by the Call Center on localhost.
_Avoid_: Dashboard, admin UI, control panel, frontend (when meaning Agents’ Surfaces)

**Prompt**:
The text an Agent wants the Human to hear on a Call, rendered to audio before or during ringing/answer.
_Avoid_: Script, message body (when meaning the spoken text)

**Synthesizer**:
The component that turns a Prompt into audio for the Line. Default is OS speech; other engines may be plugged in later.
_Avoid_: TTS service (as the domain name), voice API

**Voicemail**:
A Call the Human did not answer within the ring timeout. Unheard Voicemails persist (JSONL); once heard or deleted from the Console, they are removed. Ordinary completed Call history stays in-memory only. Audio access is by dialing a Feature Code on an idle Line; the Console lists Prompt text without playing audio.
_Avoid_: Missed call log, inbox, notification history

**Feature Code**:
A digit sequence the Human dials on an idle Line to invoke a Call Center action (e.g. play Voicemail, start Idle Dictation).
_Avoid_: Star code (as the only term), DTMF command, shortcode

**Idle Dictation**:
A Human-started session on an idle Line via Feature Code: Phone Mic carries handset audio, and a configured hotkey toggles dictation in the already-focused app. Ends on hang-up; does not steal window focus; does not yield to a ringing Agent Call.
_Avoid_: Auto-dictate on pickup, speech Call Mode, Phone Mic alone

**Dialplan**:
The mapping of Feature Codes (and Call progress behavior) on the Line — what ringing, idle dialing, and in-Call digits mean. Idle pickup is dial tone only; Voicemail and Idle Dictation Feature Codes apply on an idle Line; in-Call digits are the Call’s Reply.
_Avoid_: IVR menu tree, router
