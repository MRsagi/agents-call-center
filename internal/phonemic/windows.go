//go:build windows

package phonemic

import (
	"io"
	"log"
	"os/exec"
	"sync"
)

const windowsDefaultDevice = "CABLE Input"

type windowsMic struct {
	cfg     Config
	mu      sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	noop    bool
	started bool
}

// New builds a Windows PhoneMic (sox → waveaudio / VB-Cable).
func New(cfg Config) (PhoneMic, error) {
	if cfg.Device == "" {
		cfg.Device = windowsDefaultDevice
	}
	if cfg.Logf == nil {
		cfg.Logf = func(format string, args ...any) { log.Printf(format, args...) }
	}
	w := &windowsMic{cfg: cfg}
	if _, err := exec.LookPath("sox"); err != nil {
		cfg.Logf("phone mic: sox not found — install sox and VB-Audio Virtual Cable; see docs/phone-mic.md")
		w.noop = true
	}
	return w, nil
}

func (w *windowsMic) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started {
		return nil
	}
	if w.noop {
		w.cfg.Logf("phone mic: disabled (missing sox); RX audio will not reach %q", w.cfg.Device)
		w.started = true
		return nil
	}
	// Resample 8 kHz mono → 48 kHz stereo for VB-Cable waveaudio compatibility.
	cmd := exec.Command("sox",
		"-t", "raw", "-r", "8000", "-e", "signed-integer", "-b", "16", "-c", "1", "-",
		"-r", "48000", "-c", "2",
		"-t", "waveaudio", w.cfg.Device,
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return err
	}
	w.cmd = cmd
	w.stdin = stdin
	w.started = true
	w.cfg.Logf("phone mic: streaming RX PCM to waveaudio %q (select CABLE Output as mic in apps)", w.cfg.Device)
	return nil
}

func (w *windowsMic) WritePCM(pcm []byte) {
	if len(pcm) == 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.started || w.stdin == nil {
		return
	}
	_, _ = w.stdin.Write(pcm)
}

func (w *windowsMic) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stdin != nil {
		_ = w.stdin.Close()
		w.stdin = nil
	}
	if w.cmd != nil && w.cmd.Process != nil {
		_ = w.cmd.Process.Kill()
		_ = w.cmd.Wait()
		w.cmd = nil
	}
	w.started = false
}
