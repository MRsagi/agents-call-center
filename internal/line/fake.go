package line

import (
	"context"
	"time"
)

type FakeScript struct {
	AnswerAfter time.Duration
	Digits      string
	NoAnswer    bool
}

type Fake struct {
	script      FakeScript
	idleHandler func(digits string)
}

func NewFake(script FakeScript) *Fake {
	return &Fake{script: script}
}

func (f *Fake) Ready() bool {
	return true
}

func (f *Fake) Ring(_ context.Context) error {
	return nil
}

func (f *Fake) WaitAnswer(ctx context.Context) error {
	if f.script.NoAnswer {
		<-ctx.Done()
		return ctx.Err()
	}
	if f.script.AnswerAfter <= 0 {
		return nil
	}
	timer := time.NewTimer(f.script.AnswerAfter)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (f *Fake) Play(_ context.Context, _ []byte) error {
	return nil
}

func (f *Fake) CollectDigits(_ context.Context, max int) (string, error) {
	d := f.script.Digits
	if max > 0 && len(d) > max {
		d = d[:max]
	}
	return d, nil
}

func (f *Fake) Hangup(_ context.Context) error {
	return nil
}

func (f *Fake) OnIdleDigits(handler func(digits string)) {
	f.idleHandler = handler
}
