package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/MRsagi/agents-call-center/internal/domain"
)

func TestBus_PublishSubscribe(t *testing.T) {
	b := domain.NewBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := b.Subscribe(ctx)
	b.Publish(domain.Event{Type: domain.EventCallUpdated})
	select {
	case ev := <-ch:
		if ev.Type != domain.EventCallUpdated {
			t.Fatalf("%v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}
