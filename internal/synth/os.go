package synth

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type osSynth struct {
	synthesize func(ctx context.Context, prompt string) ([]byte, error)
}

func (o *osSynth) Synthesize(ctx context.Context, prompt string) ([]byte, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, nil
	}
	return o.synthesize(ctx, prompt)
}

// NewOS returns a Synthesizer that uses the host OS text-to-speech (say, SAPI, espeak-ng).
func NewOS() Synthesizer {
	switch runtime.GOOS {
	case "darwin":
		return &osSynth{synthesize: synthesizeDarwin}
	case "windows":
		return &osSynth{synthesize: synthesizeWindows}
	case "linux":
		return &osSynth{synthesize: synthesizeLinux}
	default:
		return &osSynth{synthesize: func(context.Context, string) ([]byte, error) {
			return nil, fmt.Errorf("OS TTS not supported on %s", runtime.GOOS)
		}}
	}
}

func synthesizeToTempFile(ctx context.Context, ext string, run func(ctx context.Context, path string) error) ([]byte, error) {
	f, err := os.CreateTemp("", "acc-tts-*"+ext)
	if err != nil {
		return nil, err
	}
	path := f.Name()
	_ = f.Close()
	defer os.Remove(path)

	if err := run(ctx, path); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func synthesizeDarwin(ctx context.Context, prompt string) ([]byte, error) {
	if _, err := exec.LookPath("say"); err != nil {
		return nil, fmt.Errorf("darwin TTS: say not found: %w", err)
	}
	return synthesizeToTempFile(ctx, ".aiff", func(ctx context.Context, path string) error {
		cmd := exec.CommandContext(ctx, "say", "-o", path, prompt)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("say: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	})
}

func synthesizeWindows(ctx context.Context, prompt string) ([]byte, error) {
	return synthesizeToTempFile(ctx, ".wav", func(ctx context.Context, path string) error {
		escapedPath := strings.ReplaceAll(path, "'", "''")
		escapedPrompt := strings.ReplaceAll(prompt, "'", "''")
		ps := fmt.Sprintf(
			"Add-Type -AssemblyName System.Speech; "+
				"$s = New-Object System.Speech.Synthesis.SpeechSynthesizer; "+
				"$s.SetOutputToWaveFile('%s'); "+
				"$s.Speak('%s'); "+
				"$s.Dispose()",
			escapedPath, escapedPrompt,
		)
		cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("System.Speech: %w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	})
}

func synthesizeLinux(ctx context.Context, prompt string) ([]byte, error) {
	bin := "espeak-ng"
	if _, err := exec.LookPath(bin); err != nil {
		bin = "espeak"
		if _, err := exec.LookPath(bin); err != nil {
			return nil, fmt.Errorf("linux TTS: espeak-ng/espeak not found")
		}
	}
	return synthesizeToTempFile(ctx, ".wav", func(ctx context.Context, path string) error {
		cmd := exec.CommandContext(ctx, bin, "-w", path, prompt)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w: %s", filepath.Base(bin), err, strings.TrimSpace(string(out)))
		}
		return nil
	})
}
