package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfigValues(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Scanner.AutoOpen != true {
		t.Fatal("expected auto_open=true")
	}
	if cfg.Scanner.Defaults.Source != "adf_duplex" {
		t.Fatalf("expected source adf_duplex, got %s", cfg.Scanner.Defaults.Source)
	}
	if cfg.Processing.MaxConcurrentJobs != 2 {
		t.Fatalf("expected max_concurrent_jobs=2, got %d", cfg.Processing.MaxConcurrentJobs)
	}
	if cfg.Processing.PDF.JPEGQuality != 85 {
		t.Fatalf("expected jpeg_quality=85, got %d", cfg.Processing.PDF.JPEGQuality)
	}
	if cfg.Processing.OCR.Enabled != true {
		t.Fatal("expected OCR enabled by default")
	}
	if cfg.Button.ShortPressProfile != "standard" {
		t.Fatalf("expected short press profile 'standard', got %s", cfg.Button.ShortPressProfile)
	}
	if cfg.Button.LongPressProfile != "oversize" {
		t.Fatalf("expected long press profile 'oversize', got %s", cfg.Button.LongPressProfile)
	}
	if cfg.Logging.Level != "info" {
		t.Fatalf("expected log level 'info', got %s", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Fatalf("expected log format 'json', got %s", cfg.Logging.Format)
	}
}

func TestDurationUnmarshalText(t *testing.T) {
	var d duration
	if err := d.UnmarshalText([]byte("50ms")); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if d.Duration() != 50*time.Millisecond {
		t.Fatalf("expected 50ms, got %v", d.Duration())
	}
}

func TestDurationUnmarshalInvalid(t *testing.T) {
	var d duration
	if err := d.UnmarshalText([]byte("not-a-duration")); err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestDurationMarshalText(t *testing.T) {
	d := duration(500 * time.Millisecond)
	text, err := d.MarshalText()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(text) != "500ms" {
		t.Fatalf("expected '500ms', got %s", string(text))
	}
}

func TestLoadConfigInvalidToml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	os.WriteFile(path, []byte("this is not [valid toml"), 0o644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestLoadConfigWithButtons(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[button]
enabled = true
poll_interval = "100ms"
long_press_duration = "2s"
short_press_profile = "standard"
long_press_profile = "oversize"
output = "paperless"
beep_on_long_press = true

[button.metadata]
title_pattern = "Scan_{date}"
correspondent = 5
document_type = 3
tags = [1, 2]
`
	os.WriteFile(path, []byte(content), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if !cfg.Button.Enabled {
		t.Fatal("expected button enabled")
	}
	if cfg.Button.PollInterval.Duration() != 100*time.Millisecond {
		t.Fatalf("expected poll interval 100ms, got %v", cfg.Button.PollInterval.Duration())
	}
	if cfg.Button.LongPressDuration.Duration() != 2*time.Second {
		t.Fatalf("expected long press 2s, got %v", cfg.Button.LongPressDuration.Duration())
	}
	if !cfg.Button.BeepOnLongPress {
		t.Fatal("expected beep on long press")
	}
	if cfg.Button.Metadata.Correspondent != 5 {
		t.Fatalf("expected correspondent 5, got %d", cfg.Button.Metadata.Correspondent)
	}
}

func TestLoadConfigWithSecrets(t *testing.T) {
	dir := t.TempDir()

	tokenFile := filepath.Join(dir, "token")
	os.WriteFile(tokenFile, []byte("  secret-token  \n"), 0o644)

	configPath := filepath.Join(dir, "config.toml")
	content := `
[output.paperless]
enabled = true
url = "http://localhost:8000"
token_file = "` + tokenFile + `"
`
	os.WriteFile(configPath, []byte(content), 0o644)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if cfg.Output.Paperless.Token != "secret-token" {
		t.Fatalf("expected token 'secret-token', got %q", cfg.Output.Paperless.Token)
	}
}

func TestLoadConfigSecretFileMissing(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	content := `
[output.paperless]
enabled = true
url = "http://localhost:8000"
token_file = "/nonexistent/token"
`
	os.WriteFile(configPath, []byte(content), 0o644)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for missing secret file with enabled output")
	}
}

func TestLoadConfigSecretFileIgnoredWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	content := `
[output.paperless]
enabled = false
url = "http://localhost:8000"
token_file = "/nonexistent/token"
`
	os.WriteFile(configPath, []byte(content), 0o644)

	_, err := Load(configPath)
	if err != nil {
		t.Fatalf("expected no error when disabled: %v", err)
	}
}

func TestLoadConfigWithEmail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[output.email]
enabled = true
smtp_host = "smtp.example.com"
smtp_port = 587
smtp_user = "user@example.com"
from_address = "scan@example.com"
default_recipient = "admin@example.com"
`
	os.WriteFile(path, []byte(content), 0o644)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if !cfg.Output.Email.Enabled {
		t.Fatal("expected email enabled")
	}
	if cfg.Output.Email.SMTPHost != "smtp.example.com" {
		t.Fatalf("unexpected smtp host: %s", cfg.Output.Email.SMTPHost)
	}
	if cfg.Output.Email.SMTPPort != 587 {
		t.Fatalf("expected smtp port 587, got %d", cfg.Output.Email.SMTPPort)
	}
}

func TestProfileStoreSet(t *testing.T) {
	store, _ := NewProfileStore("")

	custom := &Profile{
		Profile: ProfileInfo{Name: "Custom"},
		Scanner: ProfileScanner{Resolution: 1200},
	}

	store.Set("custom", custom)

	got, ok := store.Get("custom")
	if !ok {
		t.Fatal("expected to find custom profile")
	}
	if got.Scanner.Resolution != 1200 {
		t.Fatalf("expected resolution 1200, got %d", got.Scanner.Resolution)
	}
}

func TestProfileStoreOverwrite(t *testing.T) {
	store, _ := NewProfileStore("")

	// Overwrite built-in profile
	custom := &Profile{
		Profile: ProfileInfo{Name: "Modified Standard"},
		Scanner: ProfileScanner{Resolution: 600},
	}
	store.Set("standard", custom)

	got, ok := store.Get("standard")
	if !ok {
		t.Fatal("expected to find standard profile")
	}
	if got.Scanner.Resolution != 600 {
		t.Fatalf("expected resolution 600, got %d", got.Scanner.Resolution)
	}
}

func TestProfileStoreFromDirectoryIgnoresSubdirs(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory (should be ignored)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)

	// Create a non-TOML file (should be ignored)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0o644)

	store, err := NewProfileStore(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still have the 3 built-in profiles
	profiles := store.List()
	if len(profiles) != 3 {
		t.Fatalf("expected 3 built-in profiles, got %d", len(profiles))
	}
}

func TestProfileStoreFromNonexistentDir(t *testing.T) {
	store, err := NewProfileStore("/nonexistent/dir")
	if err != nil {
		t.Fatalf("expected no error for nonexistent dir, got: %v", err)
	}

	// Should still have built-in profiles
	profiles := store.List()
	if len(profiles) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(profiles))
	}
}

func TestDefaultStandardProfile(t *testing.T) {
	store, _ := NewProfileStore("")
	p, ok := store.Get("standard")
	if !ok {
		t.Fatal("standard profile not found")
	}
	if p.Scanner.Mode != "color" {
		t.Fatalf("expected mode 'color', got %s", p.Scanner.Mode)
	}
	if p.Scanner.Source != "adf_duplex" {
		t.Fatalf("expected source 'adf_duplex', got %s", p.Scanner.Source)
	}
	if !p.Processing.OCR.Enabled {
		t.Fatal("expected OCR enabled for standard")
	}
	if p.Output.DefaultTarget != "paperless" {
		t.Fatalf("expected default target 'paperless', got %s", p.Output.DefaultTarget)
	}
}

func TestDefaultPhotoProfile(t *testing.T) {
	store, _ := NewProfileStore("")
	p, ok := store.Get("photo")
	if !ok {
		t.Fatal("photo profile not found")
	}
	if p.Scanner.Resolution != 600 {
		t.Fatalf("expected resolution 600, got %d", p.Scanner.Resolution)
	}
	if p.Scanner.Source != "flatbed" {
		t.Fatalf("expected source 'flatbed', got %s", p.Scanner.Source)
	}
	if p.Processing.OCR.Enabled {
		t.Fatal("expected OCR disabled for photo")
	}
}
