package line_test

import (
	"testing"
	"time"

	"github.com/MRsagi/agents-call-center/internal/line"
)

func TestIdleDigitAggregator_FlushesAfterPause(t *testing.T) {
	var got []string
	agg := line.NewIdleDigitAggregator(50*time.Millisecond, func(digits string) {
		got = append(got, digits)
	})
	agg.Add("*")
	agg.Add("1")
	time.Sleep(80 * time.Millisecond)
	if len(got) != 1 || got[0] != "*1" {
		t.Fatalf("got=%v want [*1]", got)
	}
}

func TestIdleDigitAggregator_ClearDropsPending(t *testing.T) {
	var got []string
	agg := line.NewIdleDigitAggregator(50*time.Millisecond, func(digits string) {
		got = append(got, digits)
	})
	agg.Add("*")
	agg.Clear()
	time.Sleep(100 * time.Millisecond)
	if len(got) != 0 {
		t.Fatalf("expected no flush after clear, got %v", got)
	}
}
