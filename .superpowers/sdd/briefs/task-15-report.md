# Task 15 Report: Embedded SIP Line (PAP2T)

## Status
Complete.

## Changes
- Added `github.com/emiago/sipgo` dependency.
- New `internal/line/sip` implementing `line.Line` + `ReadyLine`: UDP registrar, digest REGISTER, INVITE/ACK/BYE via dialog client, RTP PCMU playback (WAV → μ-law), DTMF via SIP INFO.
- Config: `SIPBind`, `SIPUser`, `SIPPassword`, `SIPRealm`, `SIPExtension`, `SIPPublicHost`.
- Wired `LineBackendSIP` in `internal/app/app.go`.
- `docs/pap2t.md` PAP2T checklist and firewall notes.

## Tests
- `go test ./...` — pass.
- `TestLine_RegisterReady`: ephemeral UDP port, sipgo client REGISTER + digest → `Ready()` true.

## Manual
Not run (no PAP2T hardware in session).

## Concerns
- DTMF: RFC2833 RTP events not decoded yet; INFO/`application/dtmf-relay` only.
- `SIPPublicHost` auto-detect may pick wrong NIC on multi-homed PCs; set explicitly for production.
- SIP binds LAN (`0.0.0.0:5060`); firewall documentation in `docs/pap2t.md`.

## Fix pass (Task 15 gaps)
- Digest auth accepts username `SIPUser` or `SIPExtension`; default `SIPUser` = `100` (PAP2T Auth ID).
- `TestLine_RegisterReady` registers with digest user `100`.
- `config.ApplyEnv` + `acc serve` honor `ACC_LINE_BACKEND`, `ACC_SIP_*`.
- `app.New` merges zero SIP fields when `LineBackendSIP`.
- README PAP2T checklist → `docs/pap2t.md`; DTMF INFO documented.
- Play: AIFF PCM → μ-law (≤8 MiB); invalid WAV/AIFF returns error.
