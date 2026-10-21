package config

import (
	"path/filepath"
	"testing"
)

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := Default()
	cfg.FeatureCode = "*9"
	cfg.IdleDictationCode = "*8"
	cfg.DictationHotkey = "alt+d"
	cfg.TrayEnabled = false
	if err := SaveFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.FeatureCode != "*9" || got.IdleDictationCode != "*8" {
		t.Fatalf("%+v", got)
	}
	if got.DictationHotkey != "alt+d" {
		t.Fatalf("dictation_hotkey=%q", got.DictationHotkey)
	}
	if got.TrayEnabled {
		t.Fatal("expected tray_enabled false")
	}
}

func TestEnvOverridesWin(t *testing.T) {
	t.Setenv("ACC_LINE_BACKEND", "sip")
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := Default()
	cfg.LineBackend = LineBackendFake
	_ = SaveFile(path, cfg)
	effective, overrides, err := LoadEffective(dir)
	if err != nil {
		t.Fatal(err)
	}
	if effective.LineBackend != LineBackendSIP {
		t.Fatalf("backend=%s", effective.LineBackend)
	}
	if !overrides["line_backend"] {
		t.Fatal("expected line_backend override mark")
	}
}
