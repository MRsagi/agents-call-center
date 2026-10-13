package line_test

import (
	"context"
	"testing"
	"time"

	"github.com/MRsagi/agents-call-center/internal/line"
)

func TestFakeLine_AnswerAndDigits(t *testing.T) {
	ctx := context.Background()
	l := line.NewFake(line.FakeScript{AnswerAfter: 10 * time.Millisecond, Digits: "1"})
	if err := l.Ring(ctx); err != nil {
		t.Fatal(err)
	}
	if err := l.WaitAnswer(ctx); err != nil {
		t.Fatal(err)
	}
	if err := l.Play(ctx, []byte("audio")); err != nil {
		t.Fatal(err)
	}
	d, err := l.CollectDigits(ctx, 1)
	if err != nil || d != "1" {
		t.Fatalf("digits=%q err=%v", d, err)
	}
}
