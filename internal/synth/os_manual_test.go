//go:build manual

package synth_test

import (
	"context"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/MRsagi/agents-call-center/internal/synth"
)

func TestNewOS_SynthesizeSmoke(t *testing.T) {
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("say"); err != nil {
			t.Skip("say not installed")
		}
	}
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("espeak-ng"); err != nil {
			if _, err := exec.LookPath("espeak"); err != nil {
				t.Skip("espeak-ng/espeak not installed")
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	audio, err := synth.NewOS().Synthesize(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(audio) == 0 {
		t.Fatal("expected non-empty audio")
	}
}
