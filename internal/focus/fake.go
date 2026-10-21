package focus

import (
	"context"
	"sync"

	"github.com/MRsagi/agents-call-center/internal/domain"
)

type Fake struct {
	mu        sync.Mutex
	Focuses   []domain.FocusTarget
	Typed     [][]string
	FailFocus bool
	FailType  bool
}

func (f *Fake) Focus(ctx context.Context, target domain.FocusTarget) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Focuses = append(f.Focuses, target)
	if f.FailFocus {
		return context.Canceled
	}
	return nil
}

func (f *Fake) Type(ctx context.Context, keys []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := append([]string(nil), keys...)
	f.Typed = append(f.Typed, cp)
	if f.FailType {
		return context.Canceled
	}
	return nil
}

// TypedSnapshot returns a copy of Typed under the fake mutex.
func (f *Fake) TypedSnapshot() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([][]string, len(f.Typed))
	for i, keys := range f.Typed {
		out[i] = append([]string(nil), keys...)
	}
	return out
}

// FocusSnapshot returns a copy of Focuses under the fake mutex.
func (f *Fake) FocusSnapshot() []domain.FocusTarget {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.FocusTarget(nil), f.Focuses...)
}
