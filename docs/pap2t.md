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
| User ID / Auth ID | `100` (digest username; matches `SIPExtension` / `SIPUser`, default `100`) |
| Password | `acc` (match `SIPPassword`, default `acc`) |
| Preferred codec | G.711 μ-law (PCMU) only if possible |
| DTMF Tx Method | **SIP INFO** (required for digit collection today) |

### DTMF (SIP INFO)

The embedded SIP line collects DTMF from **SIP INFO** requests (`Signal=` header or `application/dtmf-relay` body). RFC2833 RTP telephone-event is offered in SDP but **not decoded** yet.

On the PAP2T, set **DTMF Tx Method** to **SIP INFO** (not in-band or Audio). If tones are ignored, use the ATA web UI to force INFO per FXS port and reboot the line.

## Run acc with SIP line

Environment variables (read by `acc serve`):

| Variable | Purpose |
|----------|---------|
| `ACC_LINE_BACKEND` | `sip` to use embedded SIP (default `fake`) |
| `ACC_SIP_BIND` | UDP listen, e.g. `0.0.0.0:5060` |
| `ACC_SIP_EXTENSION` | Extension the ATA registers as (default `100`) |
| `ACC_SIP_USER` | Digest username (default `100`; also accepts `SIPExtension`) |
| `ACC_SIP_PASSWORD` | Digest password (default `acc`) |
| `ACC_SIP_REALM` | Digest realm (default `agents-call-center`) |
| `ACC_SIP_PUBLIC_HOST` | IPv4 advertised in SDP (auto-detect if unset) |

Example:

```powershell
$env:ACC_LINE_BACKEND = "sip"
acc serve
```

**Play audio:** RTP expects 8 kHz G.711 μ-law. TTS should produce WAV; macOS AIFF is converted when small. Unsupported formats log an error instead of silent comfort noise.

## Readiness

Tray/status **Line up** is true when at least one REGISTER contact is stored for `SIPExtension`. Until then, queued calls wait up to `LineRecoverWait` then fail.

## Manual test checklist

1. Start `acc` with `LineBackendSIP` on the PC LAN IP.
2. Configure PAP2T as above; confirm registration (Line up in tray).
3. Enqueue a call via API/CLI; phone should ring.
4. Answer; prompt audio should play (TTS as WAV/μ-law over RTP).
5. Enter DTMF for reply; hang up cleanly (BYE).

## Limitations (v1)

- DTMF: **SIP INFO only**; in-band and RFC2833 RTP events are not decoded.
- Play: WAV (PCM/μ-law) and small AIFF PCM; other containers return an error (fake synth still uses comfort noise in tests).
- TLS/SRTP and multiple simultaneous registrations are not supported.
