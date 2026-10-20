//go:build !darwin && !windows

package phonemic

// New builds a no-op PhoneMic on platforms without a virtual-mic adapter.
func New(cfg Config) (PhoneMic, error) {
	return &Stub{}, nil
}

// Stub is a no-op PhoneMic on unsupported platforms.
type Stub struct{}

func (s *Stub) Start() error { return nil }

func (s *Stub) WritePCM([]byte) {}

func (s *Stub) Stop() {}
