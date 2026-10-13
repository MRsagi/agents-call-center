# Linksys PAP2T with agents-call-center (embedded SIP)

The call-center daemon can act as a minimal SIP registrar on your LAN. A PAP2T (or similar ATA) registers to the PC; outbound calls use INVITE + RTP (G.711 μ-law).

## Network

- **SIP bind:** default `0.0.0.0:5060` (LAN). HTTP API stays on `127.0.0.1:8741`.
- Allow **UDP 5060** (and ephemeral RTP UDP) through the host firewall for the ATA subnet.
- Set **`SIPPublicHost`** (or rely on auto-detected LAN IPv4) so SDP advertises an address the ATA can reach.

## PAP2T line settings (per FXS port)

| Field | Value |
|--------|--------|
| Line Enable | Yes |
| SIP Port | 5060 |
| Proxy / Outbound Proxy | `<PC LAN IP>` (same host running `acc`) |
| Register | Yes |
| Register Expires | 3600 |
| User ID / Auth ID | `100` (match `SIPExtension`, default `100`) |
| Password | `acc` (match `SIPPassword`, default `acc`) |
| Preferred codec | G.711 μ-law (PCMU) only if possible |
| DTMF Tx Method | SIP INFO or RFC2833 (INFO is parsed; RFC2833 telephone-event in SDP is offered) |

## Run acc with SIP line

Set config (env or future flags):

- `LineBackend`: `sip`
- `SIPBind`: `0.0.0.0:5060`
- `SIPExtension`: extension the ATA registers as (default `100`)
- `SIPUser` / `SIPPassword` / `SIPRealm`: digest credentials for REGISTER

Example (Go config in code today): use `config.Config{LineBackend: config.LineBackendSIP, ...}` when constructing the app.

## Readiness

Tray/status **Line up** is true when at least one REGISTER contact is stored for `SIPExtension`. Until then, queued calls wait up to `LineRecoverWait` then fail.

## Manual test checklist

1. Start `acc` with `LineBackendSIP` on the PC LAN IP.
2. Configure PAP2T as above; confirm registration (Line up in tray).
3. Enqueue a call via API/CLI; phone should ring.
4. Answer; prompt audio should play (TTS as WAV/μ-law over RTP).
5. Enter DTMF for reply; hang up cleanly (BYE).

## Limitations (v1)

- DTMF collection uses **SIP INFO** (`Signal=` / `application/dtmf-relay`) when the ATA sends it; in-band DTMF is not decoded.
- Non-WAV synth output is sent as short comfort noise (fake synth in tests).
- TLS/SRTP and multiple simultaneous registrations are not supported.
