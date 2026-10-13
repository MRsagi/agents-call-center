//go:build !darwin

package phonemic

// New builds a no-op PhoneMic on non-macOS platforms.
func New(cfg Config) (PhoneMic, error) {
	return &Stub{}, nil
}

// Stub is a no-op PhoneMic on non-macOS platforms.
type Stub struct{}

func (s *Stub) Start() error { return nil }

func (s *Stub) WritePCM([]byte) {}

func (s *Stub) Stop() {}
