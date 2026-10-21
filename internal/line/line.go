package line

import "context"

type Line interface {
	Ring(ctx context.Context) error
	WaitAnswer(ctx context.Context) error
	Play(ctx context.Context, audio []byte) error
	CollectDigits(ctx context.Context, max int) (string, error)
	Hangup(ctx context.Context) error
	OnIdleDigits(handler func(digits string))
}

// ReadyLine reports whether the line can accept an outbound Ring. Optional; when
// absent, LineRecoverWait is not applied (deferred until SIP exposes readiness).
type ReadyLine interface {
	Ready() bool
}

// HangupWaiter waits until the human hangs up (BYE / on-hook). Optional on Line;
// Idle Dictation requires it so the runner can block until the session ends.
type HangupWaiter interface {
	WaitHangup(ctx context.Context) error
}
