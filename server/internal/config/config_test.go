package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestLoadConfigACME(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "server.toml")

	content := `
[server]
host = "0.0.0.0"
port = 443

[server.tls]
enabled = true

[server.tls.acme]
enabled = true
email = "admin@example.com"
domains = ["scanflow.example.com", "scanner.example.com"]
challenge = "dns"
cert_dir = "/var/lib/scanflow/certs"
dns_provider = "cloudflare"
dns_propagation_wait = "90s"

[server.tls.acme.cloudflare]
api_token_file = "/etc/scanflow/cf_token"
zone_id = "zone123"
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	acmeCfg := cfg.Server.TLS.ACME
	if !acmeCfg.Enabled {
		t.Fatal("expected ACME enabled")
	}
	if acmeCfg.Email != "admin@example.com" {
		t.Fatalf("expected email admin@example.com, got %s", acmeCfg.Email)
	}
	if len(acmeCfg.Domains) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(acmeCfg.Domains))
	}
	if acmeCfg.Domains[0] != "scanflow.example.com" {
		t.Fatalf("expected first domain scanflow.example.com, got %s", acmeCfg.Domains[0])
	}
	if acmeCfg.Challenge != "dns" {
		t.Fatalf("expected challenge dns, got %s", acmeCfg.Challenge)
	}
	if acmeCfg.DNSProvider != "cloudflare" {
		t.Fatalf("expected dns_provider cloudflare, got %s", acmeCfg.DNSProvider)
	}
	if acmeCfg.DNSPropagationWait.Duration().Seconds() != 90 {
		t.Fatalf("expected propagation wait 90s, got %v", acmeCfg.DNSPropagationWait.Duration())
	}
	if acmeCfg.Cloudflare.APITokenFile != "/etc/scanflow/cf_token" {
		t.Fatalf("expected cloudflare token file, got %s", acmeCfg.Cloudflare.APITokenFile)
	}
	if acmeCfg.Cloudflare.ZoneID != "zone123" {
		t.Fatalf("expected cloudflare zone_id zone123, got %s", acmeCfg.Cloudflare.ZoneID)
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

func TestValidateDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should be valid, got: %v", err)
	}
}

func TestValidateInvalidPort(t *testing.T) {
	for _, port := range []int{0, 70000} {
		cfg := DefaultConfig()
		cfg.Server.Port = port
		err := cfg.Validate()
		if err == nil {
			t.Fatalf("expected error for port %d", port)
		}
		if !strings.Contains(err.Error(), "server.port") {
			t.Fatalf("error should mention server.port, got: %v", err)
		}
	}
}

func TestValidateInvalidOCRLang(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Processing.OCR.Enabled = true
	cfg.Processing.OCR.Language = "eng;rm -rf"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid OCR language")
	}
	if !strings.Contains(err.Error(), "processing.ocr.language") {
		t.Fatalf("error should mention processing.ocr.language, got: %v", err)
	}
}

func TestValidateInvalidTLS(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.TLS.Enabled = true
	cfg.Server.TLS.ACME.Enabled = false
	cfg.Server.TLS.CertFile = ""
	cfg.Server.TLS.KeyFile = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for TLS without cert/key")
	}
	if !strings.Contains(err.Error(), "cert_file") {
		t.Fatalf("error should mention cert_file, got: %v", err)
	}
	if !strings.Contains(err.Error(), "key_file") {
		t.Fatalf("error should mention key_file, got: %v", err)
	}
}
