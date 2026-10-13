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
