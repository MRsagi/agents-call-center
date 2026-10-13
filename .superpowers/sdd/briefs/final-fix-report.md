# Final fix report (whole-branch review)

Applied 2026-10-13.

## Changes

1. **Idle digit aggregation** — `internal/line/idle_digits.go` with 1.5s inter-digit timeout; wired in SIP `onInfo` idle path and Virtual `deliverDTMF`. Runner clears buffer after `MatchIdle` handling via `IdleDigitBuffer`.
2. **Empty mailbox** — `handleIdleDigits` synthesizes and plays “No voicemail messages” when `OldestUnheard` errors instead of silent return.
3. **Type failure delivery** — Focus `Type` errors after answer set `StatusCompleted` + `DeliveryFocusMissed` (digits retained); test `TestRunner_TypeFailStillCompletes`.
4. **Serve localhost bind** — `acc serve` calls `api.Server.ValidateBind()` before starting HTTP.
5. **Docs** — README + `docs/pap2t.md` note SIP INFO, aggregation, ATA idle limits, `acc voicemail` fallback.
6. **Serve line hint** — stderr log of line backend and `ACC_LINE_BACKEND` at serve start.

## Verification

```
go test ./... -count=1 -timeout 120s
```

All packages OK (2026-10-13).
