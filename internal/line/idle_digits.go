package line

import (
	"strings"
	"sync"
	"time"
)

const DefaultIdleDigitTimeout = 1500 * time.Millisecond

// IdleDigitBuffer can discard a pending idle digit aggregation (optional on Line).
type IdleDigitBuffer interface {
	ClearIdleDigits()
}

// IdleDigitAggregator collects one-at-a-time DTMF (e.g. SIP INFO) and flushes
// after inter-digit silence so feature codes like *1 match.
type IdleDigitAggregator struct {
	mu      sync.Mutex
	timeout time.Duration
	buf     strings.Builder
	timer   *time.Timer
	onFlush func(digits string)
}

func NewIdleDigitAggregator(timeout time.Duration, onFlush func(digits string)) *IdleDigitAggregator {
	if timeout <= 0 {
		timeout = DefaultIdleDigitTimeout
	}
	return &IdleDigitAggregator{timeout: timeout, onFlush: onFlush}
}

func (a *IdleDigitAggregator) Add(digits string) {
	if a == nil || digits == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.timer != nil {
		a.timer.Stop()
	}
	a.buf.WriteString(digits)
	a.timer = time.AfterFunc(a.timeout, a.flushLocked)
}

func (a *IdleDigitAggregator) Clear() {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.timer != nil {
		a.timer.Stop()
		a.timer = nil
	}
	a.buf.Reset()
}

func (a *IdleDigitAggregator) flushLocked() {
	a.mu.Lock()
	digits := a.buf.String()
	a.buf.Reset()
	a.timer = nil
	onFlush := a.onFlush
	a.mu.Unlock()
	if digits != "" && onFlush != nil {
		onFlush(digits)
	}
}
