package app_test

import (
	"testing"

	"github.com/MRsagi/agents-call-center/internal/app"
	"github.com/MRsagi/agents-call-center/internal/config"
	"github.com/MRsagi/agents-call-center/internal/synth"
)

func TestNew_DefaultSynthIsFake(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	a, err := app.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a.Synth.(*synth.Fake); !ok {
		t.Fatalf("Synth type=%T want *synth.Fake", a.Synth)
	}
}
