//go:build darwin

package phonemic

import (
	"io"
	"log"
	"os/exec"
	"sync"
)

type darwinMic struct {
	cfg    Config
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	noop   bool
	started bool
}

// New builds a macOS PhoneMic (sox → Core Audio / BlackHole).
func New(cfg Config) (PhoneMic, error) {
	if cfg.Device == "" {
		cfg.Device = defaultDevice
	}
	if cfg.Logf == nil {
		cfg.Logf = func(format string, args ...any) { log.Printf(format, args...) }
	}
	d := &darwinMic{cfg: cfg}
	if _, err := exec.LookPath("sox"); err != nil {
		cfg.Logf("phone mic: sox not found — install sox and BlackHole; see docs/phone-mic.md")
		d.noop = true
	}
	return d, nil
}

func (d *darwinMic) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.started {
		return nil
	}
	if d.noop {
		d.cfg.Logf("phone mic: disabled (missing sox); RX audio will not reach BlackHole")
		d.started = true
		return nil
	}
	cmd := exec.Command("sox",
		"-t", "raw", "-r", "8000", "-e", "signed-integer", "-b", "16", "-c", "1", "-",
		"-t", "coreaudio", d.cfg.Device,
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return err
	}
	d.cmd = cmd
	d.stdin = stdin
	d.started = true
	d.cfg.Logf("phone mic: streaming RX PCM to Core Audio %q (select BlackHole as mic in apps)", d.cfg.Device)
	return nil
}

func (d *darwinMic) WritePCM(pcm []byte) {
	if len(pcm) == 0 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.started || d.stdin == nil {
		return
	}
	_, _ = d.stdin.Write(pcm)
}

func (d *darwinMic) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stdin != nil {
		_ = d.stdin.Close()
		d.stdin = nil
	}
	if d.cmd != nil && d.cmd.Process != nil {
		_ = d.cmd.Process.Kill()
		_ = d.cmd.Wait()
		d.cmd = nil
	}
	d.started = false
}
