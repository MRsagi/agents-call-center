package domain_test

import (
	"testing"

	"github.com/MRsagi/agents-call-center/internal/domain"
)

func TestDialplan_VoicemailCode(t *testing.T) {
	d := domain.NewDialplan("*1")
	if d.MatchIdle("*1") != domain.IdleVoicemail {
		t.Fatal("want voicemail")
	}
	if d.MatchIdle("1") != domain.IdleError {
		t.Fatal("want error for unknown")
	}
	if d.MatchIdle("") != domain.IdleNone {
		t.Fatal("want none for empty")
	}
}
