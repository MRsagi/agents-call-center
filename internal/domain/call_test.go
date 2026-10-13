package domain_test

import (
	"testing"
	"time"

	"github.com/MRsagi/agents-call-center/internal/domain"
)

func TestNewCall_Defaults(t *testing.T) {
	ft := domain.FocusTarget{App: "Cursor", TitleContains: "agent"}
	c, err := domain.NewCall("approve deploy", domain.ModeDTMF, ft, 0)
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == "" {
		t.Fatal("expected non-empty id")
	}
	if c.Mode != domain.ModeDTMF {
		t.Fatalf("mode=%s", c.Mode)
	}
	if c.MaxDigits != 1 {
		t.Fatalf("max_digits=%d want 1", c.MaxDigits)
	}
	if c.Status != domain.StatusQueued {
		t.Fatalf("status=%s", c.Status)
	}
	if c.CreatedAt.After(time.Now().Add(time.Second)) {
		t.Fatal("CreatedAt in future")
	}
}

func TestNewCall_RejectsEmptyPrompt(t *testing.T) {
	_, err := domain.NewCall("", domain.ModeDTMF, domain.FocusTarget{}, 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewCall_RejectsSpeechModeInV1(t *testing.T) {
	_, err := domain.NewCall("hi", domain.ModeSpeech, domain.FocusTarget{}, 1)
	if err == nil {
		t.Fatal("expected error for unimplemented mode")
	}
}
