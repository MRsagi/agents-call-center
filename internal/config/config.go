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

type SynthBackend string

const (
	SynthBackendFake SynthBackend = "fake"
	SynthBackendOS   SynthBackend = "os"
)

type FocusBackend string

const (
	FocusBackendFake FocusBackend = "fake"
	FocusBackendOS   FocusBackend = "os"
)

type Config struct {
	HTTPBind        string
	DataDir         string
	RingTimeout     time.Duration
	LineRecoverWait time.Duration
	FeatureCode     string
	LineBackend         LineBackend
	SynthBackend        SynthBackend
	FocusBackend        FocusBackend
	VirtualControlBind  string
	SIPBind             string
	SIPUser             string
	SIPPassword         string
	SIPRealm            string
	SIPExtension        string
	SIPPublicHost       string
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
		SynthBackend:       SynthBackendFake,
		FocusBackend:       FocusBackendFake,
		VirtualControlBind: "127.0.0.1:8742",
		SIPBind:            "0.0.0.0:5060",
		SIPUser:            "acc",
		SIPPassword:        "acc",
		SIPRealm:           "agents-call-center",
		SIPExtension:       "100",
	}
}
