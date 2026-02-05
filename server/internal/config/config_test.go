package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.Host != "0.0.0.0" {
		t.Fatalf("expected host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Fatalf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Scanner.Defaults.Resolution != 300 {
		t.Fatalf("expected resolution 300, got %d", cfg.Scanner.Defaults.Resolution)
	}
	if cfg.Scanner.Defaults.Mode != "color" {
		t.Fatalf("expected mode color, got %s", cfg.Scanner.Defaults.Mode)
	}
	if cfg.Processing.OCR.Language != "deu+eng" {
		t.Fatalf("expected language deu+eng, got %s", cfg.Processing.OCR.Language)
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "server.toml")

	content := `
[server]
host = "127.0.0.1"
port = 9090

[scanner]
device = "test:0"
auto_open = false

[scanner.defaults]
resolution = 600
mode = "gray"

[processing.ocr]
enabled = false
language = "eng"

[logging]
level = "debug"
format = "text"
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("expected host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Fatalf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Scanner.Device != "test:0" {
		t.Fatalf("expected device test:0, got %s", cfg.Scanner.Device)
	}
	if cfg.Scanner.AutoOpen != false {
		t.Fatal("expected auto_open=false")
	}
	if cfg.Scanner.Defaults.Resolution != 600 {
		t.Fatalf("expected resolution 600, got %d", cfg.Scanner.Defaults.Resolution)
	}
	if cfg.Processing.OCR.Enabled {
		t.Fatal("expected OCR disabled")
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("expected log level debug, got %s", cfg.Logging.Level)
	}
}

func TestLoadConfigMissing(t *testing.T) {
	_, err := Load("/nonexistent/path/config.toml")
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestProfileStore(t *testing.T) {
	store, err := NewProfileStore("")
	if err != nil {
		t.Fatalf("create profile store: %v", err)
	}

	// Should have built-in profiles
	profiles := store.List()
	if len(profiles) < 3 {
		t.Fatalf("expected at least 3 built-in profiles, got %d", len(profiles))
	}

	// Get standard profile
	p, ok := store.Get("standard")
	if !ok {
		t.Fatal("standard profile not found")
	}
	if p.Scanner.Resolution != 300 {
		t.Fatalf("expected resolution 300, got %d", p.Scanner.Resolution)
	}

	// Get oversize profile
	p, ok = store.Get("oversize")
	if !ok {
		t.Fatal("oversize profile not found")
	}
	if p.Scanner.PageHeight != 0 {
		t.Fatalf("expected unlimited page height (0), got %f", p.Scanner.PageHeight)
	}

	// Non-existent profile
	_, ok = store.Get("nonexistent")
	if ok {
		t.Fatal("should not find nonexistent profile")
	}
}

func TestProfileStoreFromDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	profileContent := `
[profile]
name = "Custom"
description = "Custom profile"

[scanner]
resolution = 150
mode = "lineart"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "custom.toml"), []byte(profileContent), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	store, err := NewProfileStore(tmpDir)
	if err != nil {
		t.Fatalf("create profile store: %v", err)
	}

	p, ok := store.Get("custom")
	if !ok {
		t.Fatal("custom profile not found")
	}
	if p.Profile.Name != "Custom" {
		t.Fatalf("expected name 'Custom', got %s", p.Profile.Name)
	}
	if p.Scanner.Resolution != 150 {
		t.Fatalf("expected resolution 150, got %d", p.Scanner.Resolution)
	}
}
