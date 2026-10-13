package line

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

// VirtualConfig configures the manual-testing line (TCP control + optional stdin).
type VirtualConfig struct {
	ControlAddr string
	Stdin       bool
	Logf        func(format string, args ...any)
}

// VirtualControlFromEnv builds config from ACC_VIRTUAL_CONTROL_ADDR and ACC_VIRTUAL_STDIN.
func VirtualControlFromEnv() VirtualConfig {
	addr := strings.TrimSpace(os.Getenv("ACC_VIRTUAL_CONTROL_ADDR"))
	if addr == "" {
		addr = "127.0.0.1:8742"
	}
	stdin := os.Getenv("ACC_VIRTUAL_STDIN") == "1"
	return VirtualConfig{ControlAddr: addr, Stdin: stdin}
}

type callPhase int

const (
	phaseIdle callPhase = iota
	phaseRinging
	phaseActive
)

// Virtual implements Line for local speaker/keyboard testing. Control plane: plain-text
// TCP commands ANSWER, DTMF <digits>, HANGUP (one line each). See docs/virtual-line.md.
type Virtual struct {
	cfg VirtualConfig

	mu          sync.Mutex
	ln          net.Listener
	idleHandler func(digits string)
	idleAgg     *IdleDigitAggregator

	phase      callPhase
	answerCh   chan struct{}
	hangupCh   chan struct{}
	digitCh    chan string
	collecting bool
}

func NewVirtual(cfg VirtualConfig) (*Virtual, error) {
	if cfg.ControlAddr == "" {
		cfg.ControlAddr = "127.0.0.1:8742"
	}
	if cfg.Logf == nil {
		cfg.Logf = func(format string, args ...any) { log.Printf(format, args...) }
	}
	ln, err := net.Listen("tcp", cfg.ControlAddr)
	if err != nil {
		return nil, fmt.Errorf("virtual line control listen: %w", err)
	}
	v := &Virtual{
		cfg:      cfg,
		ln:       ln,
		digitCh:  make(chan string, 32),
		answerCh: make(chan struct{}, 1),
		hangupCh: make(chan struct{}, 1),
	}
	go v.acceptControl()
	if cfg.Stdin {
		go v.serveStdin()
	}
	cfg.Logf("virtual line: control listening on %s", ln.Addr().String())
	return v, nil
}

func (v *Virtual) ControlAddr() string {
	if v.ln == nil {
		return v.cfg.ControlAddr
	}
	return v.ln.Addr().String()
}

func (v *Virtual) Close() error {
	if v.ln != nil {
		return v.ln.Close()
	}
	return nil
}

func (v *Virtual) Ready() bool {
	return true
}

func (v *Virtual) Ring(_ context.Context) error {
	v.mu.Lock()
	v.resetCallLocked()
	v.phase = phaseRinging
	v.mu.Unlock()
	v.cfg.Logf("RINGING")
	go virtualRingBeep(v.cfg.Logf)
	return nil
}

func (v *Virtual) WaitAnswer(ctx context.Context) error {
	v.mu.Lock()
	answerCh := v.answerCh
	hangupCh := v.hangupCh
	v.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-answerCh:
		v.mu.Lock()
		if v.phase == phaseRinging {
			v.phase = phaseActive
		}
		v.mu.Unlock()
		return nil
	case <-hangupCh:
		return errors.New("call hung up")
	}
}

func (v *Virtual) Play(ctx context.Context, audio []byte) error {
	if len(audio) == 0 {
		return nil
	}
	f, err := os.CreateTemp("", "acc-virtual-*.wav")
	if err != nil {
		return err
	}
	path := f.Name()
	if _, err := f.Write(audio); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	defer os.Remove(path)
	if err := virtualPlayFile(ctx, path, v.cfg.Logf); err != nil && !errors.Is(err, context.Canceled) {
		v.cfg.Logf("virtual line: play: %v", err)
	}
	return nil
}

func (v *Virtual) CollectDigits(ctx context.Context, max int) (string, error) {
	if max <= 0 {
		return "", nil
	}
	v.mu.Lock()
	v.collecting = true
	v.mu.Unlock()
	defer func() {
		v.mu.Lock()
		v.collecting = false
		v.mu.Unlock()
	}()

	var buf strings.Builder
	for buf.Len() < max {
		v.mu.Lock()
		hangupCh := v.hangupCh
		digitCh := v.digitCh
		v.mu.Unlock()
		select {
		case <-ctx.Done():
			return trimDigits(buf.String(), max), ctx.Err()
		case <-hangupCh:
			return trimDigits(buf.String(), max), errors.New("call hung up")
		case d := <-digitCh:
			buf.WriteString(d)
			if buf.Len() >= max {
				return trimDigits(buf.String(), max), nil
			}
		}
	}
	return trimDigits(buf.String(), max), nil
}

func (v *Virtual) Hangup(_ context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.signalHangupLocked()
	v.phase = phaseIdle
	v.answerCh = make(chan struct{}, 1)
	v.hangupCh = make(chan struct{}, 1)
	return nil
}

func (v *Virtual) OnIdleDigits(handler func(digits string)) {
	v.mu.Lock()
	v.idleHandler = handler
	v.idleAgg = NewIdleDigitAggregator(DefaultIdleDigitTimeout, handler)
	v.mu.Unlock()
}

func (v *Virtual) ClearIdleDigits() {
	v.mu.Lock()
	agg := v.idleAgg
	v.mu.Unlock()
	if agg != nil {
		agg.Clear()
	}
}

func (v *Virtual) resetCallLocked() {
	v.answerCh = make(chan struct{}, 1)
	v.hangupCh = make(chan struct{}, 1)
}

func (v *Virtual) signalHangupLocked() {
	select {
	case <-v.hangupCh:
	default:
		close(v.hangupCh)
	}
}

func (v *Virtual) acceptControl() {
	for {
		conn, err := v.ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			v.cfg.Logf("virtual line: accept: %v", err)
			continue
		}
		go v.handleControlConn(conn)
	}
}

func (v *Virtual) handleControlConn(conn net.Conn) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		v.handleCommand(sc.Text())
	}
}

func (v *Virtual) serveStdin() {
	r := bufio.NewReader(os.Stdin)
	for {
		b, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			v.cfg.Logf("virtual line: stdin: %v", err)
			return
		}
		switch b {
		case '\r', '\n':
			v.handleCommand("ANSWER")
		default:
			if b >= '0' && b <= '9' || b == '*' || b == '#' {
				v.handleCommand("DTMF " + string(b))
			}
		}
	}
}

func (v *Virtual) handleCommand(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	upper := strings.ToUpper(line)
	switch {
	case upper == "ANSWER":
		v.mu.Lock()
		ch := v.answerCh
		v.mu.Unlock()
		select {
		case ch <- struct{}{}:
		default:
		}
	case upper == "HANGUP":
		v.mu.Lock()
		v.signalHangupLocked()
		v.phase = phaseIdle
		v.mu.Unlock()
	case strings.HasPrefix(upper, "DTMF"):
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return
		}
		digits := strings.Join(parts[1:], "")
		v.deliverDTMF(digits)
	}
}

func (v *Virtual) deliverDTMF(digits string) {
	if digits == "" {
		return
	}
	v.mu.Lock()
	collecting := v.collecting
	phase := v.phase
	handler := v.idleHandler
	agg := v.idleAgg
	ch := v.digitCh
	v.mu.Unlock()

	if collecting {
		select {
		case ch <- digits:
		default:
			v.cfg.Logf("virtual line: digit buffer full, dropped %q", digits)
		}
		return
	}
	if phase == phaseActive {
		return
	}
	if phase == phaseIdle {
		if agg != nil {
			agg.Add(digits)
		} else if handler != nil {
			handler(digits)
		}
	}
}

func trimDigits(s string, max int) string {
	if max > 0 && len(s) > max {
		return s[:max]
	}
	return s
}

func virtualRingBeep(logf func(format string, args ...any)) {
	type beeper struct {
		name string
		args []string
	}
	var beepers []beeper
	switch runtime.GOOS {
	case "windows":
		beepers = append(beepers, beeper{"powershell", []string{"-NoProfile", "-NonInteractive", "-Command", "[console]::beep(800,200)"}})
	case "darwin":
		beepers = append(beepers, beeper{"afplay", []string{"/System/Library/Sounds/Tink.aiff"}})
		beepers = append(beepers, beeper{"osascript", []string{"-e", "beep"}})
	default:
		beepers = append(beepers, beeper{"sh", []string{"-c", "printf '\\a'"}})
		beepers = append(beepers, beeper{"paplay", []string{"/usr/share/sounds/freedesktop/stereo/bell.oga"}})
	}
	for _, b := range beepers {
		if _, err := exec.LookPath(b.name); err != nil {
			continue
		}
		cmd := exec.Command(b.name, b.args...)
		if err := cmd.Run(); err == nil {
			return
		}
	}
	logf("virtual line: ring beep skipped (no player available)")
}

func virtualPlayFile(ctx context.Context, path string, logf func(format string, args ...any)) error {
	type player struct {
		name string
		args []string
	}
	var players []player
	switch runtime.GOOS {
	case "darwin":
		players = append(players, player{"afplay", []string{path}})
	case "windows":
		ps := fmt.Sprintf("(New-Object Media.SoundPlayer '%s').PlaySync();", strings.ReplaceAll(path, "'", "''"))
		players = append(players, player{"powershell", []string{"-NoProfile", "-NonInteractive", "-Command", ps}})
	}
	players = append(players, player{"ffplay", []string{"-nodisp", "-autoexit", "-loglevel", "quiet", path}})

	for _, p := range players {
		if _, err := exec.LookPath(p.name); err != nil {
			continue
		}
		cmd := exec.CommandContext(ctx, p.name, p.args...)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	logf("virtual line: no audio player available (skipped playback)")
	return nil
}
