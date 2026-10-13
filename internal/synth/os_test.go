package synth_test

import (
	"testing"

	"github.com/MRsagi/agents-call-center/internal/synth"
)

func TestNewOS_ImplementsSynthesizer(t *testing.T) {
	s := synth.NewOS()
	if s == nil {
		t.Fatal("NewOS returned nil")
	}
	var _ synth.Synthesizer = s
}
