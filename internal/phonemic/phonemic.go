package phonemic

import (
	"sync"
)

// PhoneMic feeds off-hook line RX audio into the host virtual microphone (macOS + BlackHole).
type PhoneMic interface {
	Start() error
	WritePCM(pcm []byte)
	Stop()
}

// Config configures the Phone Mic adapter.
type Config struct {
	Device string // Core Audio device name, default "BlackHole 2ch"
	Logf   func(format string, args ...any)
}

const defaultDevice = "BlackHole 2ch"

// Fake records PCM for tests.
type Fake struct {
	mu      sync.Mutex
	started bool
	pcm     []byte
}

func (f *Fake) Start() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = true
	return nil
}

func (f *Fake) WritePCM(pcm []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pcm = append(f.pcm, pcm...)
}

func (f *Fake) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = false
}

func (f *Fake) Started() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.started
}

func (f *Fake) PCM() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]byte(nil), f.pcm...)
}
