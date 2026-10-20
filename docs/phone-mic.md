# Phone Mic

Feeds **inbound line audio** (handset → acc over SIP RTP) into a virtual microphone so apps (e.g. Cursor dictation) can hear the caller as a normal mic input.

## Windows (VB-Cable + sox)

1. Install [VB-Audio Virtual Cable](https://vb-audio.com/Cable/) (free).
2. Install **sox** (must be on `PATH`):
   - Scoop: `scoop install sox`
   - Chocolatey: `choco install sox`
   - Or [sox.sourceforge.net](https://sourceforge.net/projects/sox/)
3. Confirm devices in Windows Sound settings:
   - Playback: **CABLE Input** (acc writes here)
   - Recording: **CABLE Output** (apps use this as the mic)

```powershell
$env:ACC_LINE_BACKEND = "sip"
$env:ACC_SIP_PUBLIC_HOST = "192.168.10.1"
$env:ACC_PHONE_MIC_ENABLED = "1"
# optional (default on Windows):
# $env:ACC_PHONE_MIC_DEVICE = "CABLE Input"
.\acc.exe serve
```

In Cursor (or any app), choose **CABLE Output** as the microphone input.

## macOS (BlackHole + sox)

1. Install [BlackHole](https://existential.audio/blackhole/) (2ch is enough).
2. Install **sox**: `brew install sox`
3. In **Audio MIDI Setup**, optional: create a Multi-Output device if you also want local speakers.

```bash
export ACC_LINE_BACKEND=sip
export ACC_PHONE_MIC_ENABLED=1
# optional:
export ACC_PHONE_MIC_DEVICE="BlackHole 2ch"
acc serve
```

In Cursor (or any app), choose **BlackHole 2ch** as the microphone input.

## Behavior

- When enabled, `acc serve` starts a sox pipeline: 8 kHz mono PCM16 → virtual cable device.
- During an answered SIP call, inbound RTP μ-law is decoded and teed to the Phone Mic.
- Virtual Line has no handset RX path yet; use SIP/PAP2T for Phone Mic testing.
- Linux: Phone Mic is a no-op stub (not implemented yet).
- If sox is missing, Phone Mic logs and no-ops (calls still work).

## Verify

1. Register PAP2T (see [pap2t.md](pap2t.md)).
2. Enable Phone Mic env vars and run `acc serve`.
3. Place a test call; speak into the FXS phone.
4. Open a dictation target using the virtual mic and confirm audio levels move.
