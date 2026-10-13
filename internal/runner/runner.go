package runner

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/MRsagi/agents-call-center/internal/config"
	"github.com/MRsagi/agents-call-center/internal/domain"
	"github.com/MRsagi/agents-call-center/internal/focus"
	"github.com/MRsagi/agents-call-center/internal/line"
	"github.com/MRsagi/agents-call-center/internal/synth"
)

type Deps struct {
	Queue           *domain.Queue
	Line            line.Line
	Synth           synth.Synthesizer
	Focus           focus.Focuser
	Voicemail       domain.VoicemailStore
	Dialplan        domain.Dialplan
	RingTimeout     time.Duration
	LineRecoverWait time.Duration
}

type Runner struct {
	deps Deps

	mu        sync.Mutex
	processing bool
}

func New(deps Deps) *Runner {
	def := config.Default()
	if deps.RingTimeout == 0 {
		deps.RingTimeout = def.RingTimeout
	}
	if deps.LineRecoverWait == 0 {
		deps.LineRecoverWait = def.LineRecoverWait
	}
	return &Runner{deps: deps}
}

func (r *Runner) Start(ctx context.Context) {
	r.deps.Line.OnIdleDigits(func(digits string) {
		if !r.trySetIdle() {
			return
		}
		defer r.clearIdle()
		r.handleIdleDigits(ctx, digits)
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !r.tryAcquireProcessing() {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		c, ok := r.deps.Queue.Dequeue()
		if !ok {
			r.clearProcessing()
			time.Sleep(10 * time.Millisecond)
			continue
		}
		r.processCall(ctx, c)
	}
}

func (r *Runner) tryAcquireProcessing() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.processing {
		return false
	}
	r.processing = true
	return true
}

func (r *Runner) trySetIdle() bool {
	return r.tryAcquireProcessing()
}

func (r *Runner) clearProcessing() {
	r.mu.Lock()
	r.processing = false
	r.mu.Unlock()
}

func (r *Runner) clearIdle() {
	r.clearProcessing()
}

func (r *Runner) processCall(ctx context.Context, c *domain.Call) {
	defer r.clearProcessing()

	c.Status = domain.StatusRinging

	audio, err := r.deps.Synth.Synthesize(ctx, c.Prompt)
	if err != nil {
		c.Status = domain.StatusFailed
		c.Error = err.Error()
		return
	}

	if err := r.waitLineReady(ctx); err != nil {
		c.Status = domain.StatusFailed
		c.Error = err.Error()
		return
	}

	ringCtx, cancel := context.WithTimeout(ctx, r.deps.RingTimeout)
	defer cancel()

	if err := r.deps.Line.Ring(ringCtx); err != nil && !errors.Is(err, context.Canceled) && ringCtx.Err() == nil {
		c.Status = domain.StatusFailed
		c.Error = err.Error()
		return
	}

	if err := r.deps.Line.WaitAnswer(ringCtx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			_ = r.deps.Voicemail.Leave(c)
			c.Status = domain.StatusVoicemail
			_ = r.deps.Line.Hangup(context.Background())
			return
		}
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			c.Status = domain.StatusFailed
			if ctx.Err() != nil {
				c.Error = ctx.Err().Error()
			} else {
				c.Error = err.Error()
			}
			_ = r.deps.Line.Hangup(context.Background())
			return
		}
		c.Status = domain.StatusFailed
		c.Error = err.Error()
		_ = r.deps.Line.Hangup(context.Background())
		return
	}

	c.Status = domain.StatusInProgress
	delivery := domain.DeliveryOK
	if err := r.deps.Focus.Focus(ctx, c.FocusTarget); err != nil {
		delivery = domain.DeliveryFocusMissed
	}

	if err := r.deps.Line.Play(ctx, audio); err != nil {
		c.Status = domain.StatusFailed
		c.Error = err.Error()
		_ = r.deps.Line.Hangup(context.Background())
		return
	}

	digits, err := r.deps.Line.CollectDigits(ctx, c.MaxDigits)
	if err != nil {
		c.Status = domain.StatusFailed
		c.Error = err.Error()
		_ = r.deps.Line.Hangup(context.Background())
		return
	}
	c.Digits = digits

	if err := r.deps.Focus.Type(ctx, focus.KeysForDigits(digits)); err != nil {
		c.Status = domain.StatusFailed
		c.Error = err.Error()
		_ = r.deps.Line.Hangup(context.Background())
		return
	}

	_ = r.deps.Line.Hangup(context.Background())
	c.Delivery = delivery
	c.Status = domain.StatusCompleted
}

func (r *Runner) handleIdleDigits(ctx context.Context, digits string) {
	switch r.deps.Dialplan.MatchIdle(digits) {
	case domain.IdleVoicemail:
		entry, err := r.deps.Voicemail.OldestUnheard()
		if err != nil {
			return
		}
		audio, err := r.deps.Synth.Synthesize(ctx, entry.Prompt)
		if err != nil {
			return
		}
		_ = r.deps.Line.Play(ctx, audio)
		collected, err := r.deps.Line.CollectDigits(ctx, entry.MaxDigits)
		if err != nil {
			return
		}
		_ = r.deps.Focus.Focus(ctx, entry.FocusTarget)
		_ = r.deps.Focus.Type(ctx, focus.KeysForDigits(collected))
		_ = r.deps.Voicemail.MarkHeard(entry.CallID)
	case domain.IdleError, domain.IdleNone:
	}
}

func (r *Runner) waitLineReady(ctx context.Context) error {
	wait := r.deps.LineRecoverWait
	if wait <= 0 {
		return nil
	}
	rl, ok := r.deps.Line.(line.ReadyLine)
	if !ok {
		// deferred until SIP Line exposes readiness
		return nil
	}
	if rl.Ready() {
		return nil
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if rl.Ready() {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return errors.New("line not ready")
}
