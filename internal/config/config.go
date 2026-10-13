package config

import (
	"os"
	"path/filepath"
	"time"
)

type LineBackend string

const (
	LineBackendFake    LineBackend = "fake"
	LineBackendSIP     LineBackend = "sip"
	LineBackendVirtual LineBackend = "virtual"
)

type Config struct {
	HTTPBind        string
	DataDir         string
	RingTimeout     time.Duration
	LineRecoverWait time.Duration
	FeatureCode     string
	LineBackend         LineBackend
	VirtualControlBind  string
}

func DefaultDataDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "agents-call-center")
	}
	return filepath.Join(dir, "agents-call-center")
}

func Default() Config {
	return Config{
		HTTPBind:        "127.0.0.1:8741",
		DataDir:         DefaultDataDir(),
		RingTimeout:     90 * time.Second,
		LineRecoverWait: 3 * time.Second,
		FeatureCode:     "*1",
		LineBackend:        LineBackendFake,
		VirtualControlBind: "127.0.0.1:8742",
	}
}
