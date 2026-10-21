package line

import (
	"context"
	"sync"
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

	mu       sync.Mutex
	hangupCh chan struct{}
}

func NewFake(script FakeScript) *Fake {
	return &Fake{script: script, hangupCh: make(chan struct{})}
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

// TriggerIdleDigits invokes the idle-digit handler asynchronously (like SIP/Virtual delivery).
func (f *Fake) TriggerIdleDigits(digits string) {
	h := f.idleHandler
	if h == nil {
		return
	}
	go h(digits)
}

// TriggerHangup closes the hangup channel so WaitHangup returns.
func (f *Fake) TriggerHangup() {
	f.mu.Lock()
	defer f.mu.Unlock()
	select {
	case <-f.hangupCh:
	default:
		close(f.hangupCh)
	}
}

func (f *Fake) WaitHangup(ctx context.Context) error {
	f.mu.Lock()
	ch := f.hangupCh
	f.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ch:
		f.mu.Lock()
		if f.hangupCh == ch {
			f.hangupCh = make(chan struct{})
		}
		f.mu.Unlock()
		return nil
	}
}

var _ HangupWaiter = (*Fake)(nil)
