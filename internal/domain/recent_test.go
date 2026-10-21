package domain_test

import (
	"fmt"
	"testing"

	"github.com/MRsagi/agents-call-center/internal/domain"
)

func TestRecentCalls_Cap(t *testing.T) {
	r := domain.NewRecentCalls(20)
	for i := 0; i < 25; i++ {
		c, err := domain.NewCall(fmt.Sprintf("p%d", i), domain.ModeDTMF, domain.FocusTarget{App: "X"}, 1)
		if err != nil {
			t.Fatal(err)
		}
		c.Status = domain.StatusCompleted
		r.Add(c)
	}
	if got := len(r.Snapshot()); got != 20 {
		t.Fatalf("len=%d", got)
	}
}
