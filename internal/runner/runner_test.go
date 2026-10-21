package runner_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/MRsagi/agents-call-center/internal/domain"
	"github.com/MRsagi/agents-call-center/internal/focus"
	"github.com/MRsagi/agents-call-center/internal/line"
	"github.com/MRsagi/agents-call-center/internal/runner"
	"github.com/MRsagi/agents-call-center/internal/synth"
)

func waitStatus(t *testing.T, q *domain.Queue, id string, want domain.Status, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c, ok := q.Get(id)
		if ok && c.Status == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	c, _ := q.Get(id)
	t.Fatalf("status=%v want=%v", c.Status, want)
}

func TestRunner_CompletedDeliversKeystrokes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := domain.NewQueue()
	vmPath := filepath.Join(t.TempDir(), "vm.jsonl")
	vm, _ := domain.OpenVoicemailStore(vmPath)
	foc := &focus.Fake{}
	ln := line.NewFake(line.FakeScript{AnswerAfter: time.Millisecond, Digits: "1"})
	syn := &synth.Fake{}
	r := runner.New(runner.Deps{
		Queue: q, Line: ln, Synth: syn, Focus: foc, Voicemail: vm,
		Dialplan: domain.NewDialplan("*1", "*2"),
		RingTimeout: time.Second,
	})
	go r.Start(ctx)
	c, _ := domain.NewCall("approve?", domain.ModeDTMF, domain.FocusTarget{App: "Cursor"}, 1)
	_ = q.Enqueue(c)
	waitStatus(t, q, c.ID, domain.StatusCompleted, 2*time.Second)
	if len(foc.Typed) != 1 || foc.Typed[0][0] != "1" || foc.Typed[0][1] != "Enter" {
		t.Fatalf("typed=%v", foc.Typed)
	}
}

func TestRunner_NoAnswerLeavesVoicemail(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := domain.NewQueue()
	vmPath := filepath.Join(t.TempDir(), "vm.jsonl")
	vm, _ := domain.OpenVoicemailStore(vmPath)
	foc := &focus.Fake{}
	ln := line.NewFake(line.FakeScript{NoAnswer: true})
	syn := &synth.Fake{}
	r := runner.New(runner.Deps{
		Queue: q, Line: ln, Synth: syn, Focus: foc, Voicemail: vm,
		Dialplan: domain.NewDialplan("*1", "*2"),
		RingTimeout: 50 * time.Millisecond,
	})
	go r.Start(ctx)
	c, _ := domain.NewCall("hello", domain.ModeDTMF, domain.FocusTarget{App: "Cursor"}, 1)
	_ = q.Enqueue(c)
	waitStatus(t, q, c.ID, domain.StatusVoicemail, 2*time.Second)
	n, err := vm.Count()
	if err != nil || n != 1 {
		t.Fatalf("count=%d err=%v", n, err)
	}
}

func TestRunner_TypeFailStillCompletes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := domain.NewQueue()
	vmPath := filepath.Join(t.TempDir(), "vm.jsonl")
	vm, _ := domain.OpenVoicemailStore(vmPath)
	foc := &focus.Fake{FailType: true}
	ln := line.NewFake(line.FakeScript{AnswerAfter: time.Millisecond, Digits: "3"})
	syn := &synth.Fake{}
	r := runner.New(runner.Deps{
		Queue: q, Line: ln, Synth: syn, Focus: foc, Voicemail: vm,
		Dialplan: domain.NewDialplan("*1", "*2"),
		RingTimeout: time.Second,
	})
	go r.Start(ctx)
	c, _ := domain.NewCall("pick", domain.ModeDTMF, domain.FocusTarget{App: "Cursor"}, 1)
	_ = q.Enqueue(c)
	waitStatus(t, q, c.ID, domain.StatusCompleted, 2*time.Second)
	got, _ := q.Get(c.ID)
	if got.Delivery != domain.DeliveryFocusMissed {
		t.Fatalf("delivery=%q", got.Delivery)
	}
	if got.Digits != "3" {
		t.Fatalf("digits=%q", got.Digits)
	}
}

func TestRunner_FocusFailStillCompletes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q := domain.NewQueue()
	vmPath := filepath.Join(t.TempDir(), "vm.jsonl")
	vm, _ := domain.OpenVoicemailStore(vmPath)
	foc := &focus.Fake{FailFocus: true}
	ln := line.NewFake(line.FakeScript{AnswerAfter: time.Millisecond, Digits: "2"})
	syn := &synth.Fake{}
	r := runner.New(runner.Deps{
		Queue: q, Line: ln, Synth: syn, Focus: foc, Voicemail: vm,
		Dialplan: domain.NewDialplan("*1", "*2"),
		RingTimeout: time.Second,
	})
	go r.Start(ctx)
	c, _ := domain.NewCall("pick", domain.ModeDTMF, domain.FocusTarget{App: "Cursor"}, 1)
	_ = q.Enqueue(c)
	waitStatus(t, q, c.ID, domain.StatusCompleted, 2*time.Second)
	got, _ := q.Get(c.ID)
	if got.Delivery != domain.DeliveryFocusMissed {
		t.Fatalf("delivery=%q", got.Delivery)
	}
	if got.Digits != "2" {
		t.Fatalf("digits=%q", got.Digits)
	}
	if len(foc.Typed) != 0 {
		t.Fatalf("typed=%v want none when focus fails", foc.Typed)
	}
}
