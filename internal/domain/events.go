package domain

import (
	"context"
	"sync"
	"time"
)

const (
	EventCallUpdated       = "call.updated"
	EventVoicemailChanged  = "voicemail.changed"
	EventDictationStarted  = "dictation.started"
	EventDictationStopped  = "dictation.stopped"
	EventRestartSoon       = "restart.soon"
)

type Event struct {
	Type    string
	Payload any
	At      time.Time
}

type Bus struct {
	mu          sync.Mutex
	subscribers map[chan Event]struct{}
}

func NewBus() *Bus {
	return &Bus{subscribers: make(map[chan Event]struct{})}
}

func (b *Bus) Publish(ev Event) {
	if ev.At.IsZero() {
		ev.At = time.Now().UTC()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subscribers {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (b *Bus) Subscribe(ctx context.Context) <-chan Event {
	ch := make(chan Event, 16)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.mu.Lock()
		delete(b.subscribers, ch)
		b.mu.Unlock()
		close(ch)
	}()

	return ch
}
