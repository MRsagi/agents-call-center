package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	FeatureCode        string
	IdleDictationCode  string
	DictationHotkey    string
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
	PhoneMicEnabled     bool
	PhoneMicDevice      string
}

func DefaultDataDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "agents-call-center")
	}
	return filepath.Join(dir, "agents-call-center")
}

// ApplyEnv overrides cfg fields when the corresponding ACC_* environment variables are set.
func ApplyEnv(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("ACC_LINE_BACKEND")); v != "" {
		cfg.LineBackend = LineBackend(v)
	}
	if v := strings.TrimSpace(os.Getenv("ACC_SIP_BIND")); v != "" {
		cfg.SIPBind = v
	}
	if v := strings.TrimSpace(os.Getenv("ACC_SIP_USER")); v != "" {
		cfg.SIPUser = v
	}
	if v := strings.TrimSpace(os.Getenv("ACC_SIP_PASSWORD")); v != "" {
		cfg.SIPPassword = v
	}
	if v := strings.TrimSpace(os.Getenv("ACC_SIP_REALM")); v != "" {
		cfg.SIPRealm = v
	}
	if v := strings.TrimSpace(os.Getenv("ACC_SIP_EXTENSION")); v != "" {
		cfg.SIPExtension = v
	}
	if v := strings.TrimSpace(os.Getenv("ACC_SIP_PUBLIC_HOST")); v != "" {
		cfg.SIPPublicHost = v
	}
	if v := strings.TrimSpace(os.Getenv("ACC_PHONE_MIC_ENABLED")); v != "" {
		cfg.PhoneMicEnabled = v == "1" || strings.EqualFold(v, "true")
	}
	if v := strings.TrimSpace(os.Getenv("ACC_PHONE_MIC_DEVICE")); v != "" {
		cfg.PhoneMicDevice = v
	}
}

func Default() Config {
	return Config{
		HTTPBind:        "127.0.0.1:8741",
		DataDir:         DefaultDataDir(),
		RingTimeout:     90 * time.Second,
		LineRecoverWait: 3 * time.Second,
		FeatureCode:       "*1",
		IdleDictationCode: "*2",
		DictationHotkey:   "ctrl+m",
		LineBackend:        LineBackendFake,
		SynthBackend:       SynthBackendFake,
		FocusBackend:       FocusBackendFake,
		VirtualControlBind: "127.0.0.1:8742",
		SIPBind:            "0.0.0.0:5060",
		SIPUser:            "100",
		SIPPassword:        "acc",
		SIPRealm:           "agents-call-center",
		SIPExtension:       "100",
		PhoneMicDevice:     defaultPhoneMicDevice(),
	}
}

func defaultPhoneMicDevice() string {
	if runtime.GOOS == "windows" {
		return "CABLE Input"
	}
	return "BlackHole 2ch"
}
