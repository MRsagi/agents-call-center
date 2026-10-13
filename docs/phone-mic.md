# Phone Mic (macOS + BlackHole)

Feeds **inbound line audio** (handset → acc over SIP RTP) into a virtual microphone so macOS apps (e.g. Cursor dictation) can hear the caller.

## Install

1. Install [BlackHole](https://existential.audio/blackhole/) (2ch is enough).
2. Install **sox**: `brew install sox`
3. In **Audio MIDI Setup**, optional: create a Multi-Output device if you also want local speakers.

## Configure acc

```bash
export ACC_LINE_BACKEND=sip
export ACC_PHONE_MIC_ENABLED=1
# optional:
export ACC_PHONE_MIC_DEVICE="BlackHole 2ch"
acc serve
```

In Cursor (or any app), choose **BlackHole 2ch** as the microphone input.

## Behavior

- When enabled, `acc serve` starts a sox pipeline: 8 kHz mono PCM16 → Core Audio device (BlackHole).
- During an answered SIP call, inbound RTP μ-law is decoded and teed to the Phone Mic.
- Virtual Line has no handset RX path yet; use SIP/PAP2T for Phone Mic testing.
- Non-macOS: Phone Mic is a no-op stub.

## Verify

1. Register PAP2T (see [pap2t.md](pap2t.md)).
2. Enable Phone Mic env vars and run `acc serve`.
3. Place a test call; speak into the FXS phone.
4. Open a dictation target using BlackHole as input and confirm audio levels move.
