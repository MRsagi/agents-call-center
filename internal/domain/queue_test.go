package domain_test

import (
	"testing"

	"github.com/MRsagi/agents-call-center/internal/domain"
)

func TestQueue_FIFO(t *testing.T) {
	q := domain.NewQueue()
	a, _ := domain.NewCall("a", domain.ModeDTMF, domain.FocusTarget{}, 1)
	b, _ := domain.NewCall("b", domain.ModeDTMF, domain.FocusTarget{}, 1)
	if err := q.Enqueue(a); err != nil {
		t.Fatal(err)
	}
	if err := q.Enqueue(b); err != nil {
		t.Fatal(err)
	}
	if q.Len() != 2 {
		t.Fatalf("len=%d", q.Len())
	}
	got, ok := q.Dequeue()
	if !ok || got.ID != a.ID {
		t.Fatalf("first dequeue want %s got %#v", a.ID, got)
	}
	got, ok = q.Dequeue()
	if !ok || got.ID != b.ID {
		t.Fatalf("second dequeue want %s", b.ID)
	}
	_, ok = q.Dequeue()
	if ok {
		t.Fatal("expected empty")
	}
}

func TestQueue_GetByID(t *testing.T) {
	q := domain.NewQueue()
	c, _ := domain.NewCall("x", domain.ModeDTMF, domain.FocusTarget{}, 1)
	_ = q.Enqueue(c)
	got, ok := q.Get(c.ID)
	if !ok || got.Prompt != "x" {
		t.Fatalf("get failed: %#v", got)
	}
}
