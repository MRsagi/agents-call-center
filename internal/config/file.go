package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type fileDTO struct {
	HTTPBind           string `json:"http_bind"`
	DataDir            string `json:"data_dir"`
	RingTimeoutMS      int64  `json:"ring_timeout_ms"`
	LineRecoverWaitMS  int64  `json:"line_recover_wait_ms"`
	FeatureCode        string `json:"feature_code"`
	IdleDictationCode  string `json:"idle_dictation_code"`
	DictationHotkey    string `json:"dictation_hotkey"`
	LineBackend        string `json:"line_backend"`
	SynthBackend       string `json:"synth_backend"`
	FocusBackend       string `json:"focus_backend"`
	VirtualControlBind string `json:"virtual_control_bind"`
	SIPBind            string `json:"sip_bind"`
	SIPUser            string `json:"sip_user"`
	SIPPassword        string `json:"sip_password"`
	SIPRealm           string `json:"sip_realm"`
	SIPExtension       string `json:"sip_extension"`
	SIPPublicHost      string `json:"sip_public_host"`
	PhoneMicEnabled    bool   `json:"phone_mic_enabled"`
	PhoneMicDevice     string `json:"phone_mic_device"`
	TrayEnabled        *bool  `json:"tray_enabled,omitempty"` // nil = default on
}

func toDTO(cfg Config) fileDTO {
	dto := fileDTO{
		HTTPBind:           cfg.HTTPBind,
		DataDir:            cfg.DataDir,
		RingTimeoutMS:      cfg.RingTimeout.Milliseconds(),
		LineRecoverWaitMS:  cfg.LineRecoverWait.Milliseconds(),
		FeatureCode:        cfg.FeatureCode,
		IdleDictationCode:  cfg.IdleDictationCode,
		DictationHotkey:    cfg.DictationHotkey,
		LineBackend:        string(cfg.LineBackend),
		SynthBackend:       string(cfg.SynthBackend),
		FocusBackend:       string(cfg.FocusBackend),
		VirtualControlBind: cfg.VirtualControlBind,
		SIPBind:            cfg.SIPBind,
		SIPUser:            cfg.SIPUser,
		SIPPassword:        cfg.SIPPassword,
		SIPRealm:           cfg.SIPRealm,
		SIPExtension:       cfg.SIPExtension,
		SIPPublicHost:      cfg.SIPPublicHost,
		PhoneMicEnabled:    cfg.PhoneMicEnabled,
		PhoneMicDevice:     cfg.PhoneMicDevice,
	}
	if !cfg.TrayEnabled {
		f := false
		dto.TrayEnabled = &f
	}
	return dto
}

func fromDTO(dto fileDTO) Config {
	cfg := Default()
	if dto.HTTPBind != "" {
		cfg.HTTPBind = dto.HTTPBind
	}
	if dto.DataDir != "" {
		cfg.DataDir = dto.DataDir
	}
	if dto.RingTimeoutMS > 0 {
		cfg.RingTimeout = time.Duration(dto.RingTimeoutMS) * time.Millisecond
	}
	if dto.LineRecoverWaitMS > 0 {
		cfg.LineRecoverWait = time.Duration(dto.LineRecoverWaitMS) * time.Millisecond
	}
	if dto.FeatureCode != "" {
		cfg.FeatureCode = dto.FeatureCode
	}
	if dto.IdleDictationCode != "" {
		cfg.IdleDictationCode = dto.IdleDictationCode
	}
	if dto.DictationHotkey != "" {
		cfg.DictationHotkey = dto.DictationHotkey
	}
	if dto.LineBackend != "" {
		cfg.LineBackend = LineBackend(dto.LineBackend)
	}
	if dto.SynthBackend != "" {
		cfg.SynthBackend = SynthBackend(dto.SynthBackend)
	}
	if dto.FocusBackend != "" {
		cfg.FocusBackend = FocusBackend(dto.FocusBackend)
	}
	if dto.VirtualControlBind != "" {
		cfg.VirtualControlBind = dto.VirtualControlBind
	}
	if dto.SIPBind != "" {
		cfg.SIPBind = dto.SIPBind
	}
	if dto.SIPUser != "" {
		cfg.SIPUser = dto.SIPUser
	}
	if dto.SIPPassword != "" {
		cfg.SIPPassword = dto.SIPPassword
	}
	if dto.SIPRealm != "" {
		cfg.SIPRealm = dto.SIPRealm
	}
	if dto.SIPExtension != "" {
		cfg.SIPExtension = dto.SIPExtension
	}
	if dto.SIPPublicHost != "" {
		cfg.SIPPublicHost = dto.SIPPublicHost
	}
	cfg.PhoneMicEnabled = dto.PhoneMicEnabled
	if dto.PhoneMicDevice != "" {
		cfg.PhoneMicDevice = dto.PhoneMicDevice
	}
	if dto.TrayEnabled != nil {
		cfg.TrayEnabled = *dto.TrayEnabled
	}
	return cfg
}

// SaveFile writes cfg to path as snake_case JSON.
func SaveFile(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(toDTO(cfg), "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// LoadFile reads a config JSON file from path.
func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var dto fileDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return Config{}, err
	}
	return fromDTO(dto), nil
}

// LoadEffective loads Default, merges {dataDir}/config.json when present, then ApplyEnv.
// envOverrides keys are JSON field names overridden by ACC_* environment variables.
func LoadEffective(dataDir string) (cfg Config, envOverrides map[string]bool, err error) {
	cfg = Default()
	path := filepath.Join(dataDir, "config.json")
	if _, statErr := os.Stat(path); statErr == nil {
		loaded, loadErr := LoadFile(path)
		if loadErr != nil {
			return Config{}, nil, loadErr
		}
		cfg = loaded
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return Config{}, nil, statErr
	}
	envOverrides = ApplyEnv(&cfg)
	return cfg, envOverrides, nil
}
