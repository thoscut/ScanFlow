package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.URL != "http://localhost:8080" {
		t.Fatalf("expected URL http://localhost:8080, got %s", cfg.Server.URL)
	}
	if cfg.Defaults.Profile != "standard" {
		t.Fatalf("expected profile standard, got %s", cfg.Defaults.Profile)
	}
	if cfg.Defaults.Output != "paperless" {
		t.Fatalf("expected output paperless, got %s", cfg.Defaults.Output)
	}
}

func TestLoadFrom(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "client.toml")

	content := `
[server]
url = "http://192.168.1.100:9090"
api_key = "test-key"

[defaults]
profile = "oversize"
output = "smb"
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Server.URL != "http://192.168.1.100:9090" {
		t.Fatalf("expected URL http://192.168.1.100:9090, got %s", cfg.Server.URL)
	}
	if cfg.Server.APIKey != "test-key" {
		t.Fatalf("expected API key test-key, got %s", cfg.Server.APIKey)
	}
	if cfg.Defaults.Profile != "oversize" {
		t.Fatalf("expected profile oversize, got %s", cfg.Defaults.Profile)
	}
}

func TestLoadFromMissing(t *testing.T) {
	cfg, err := LoadFrom("/nonexistent/path.toml")
	if err != nil {
		t.Fatalf("should return default config for missing file, got error: %v", err)
	}
	if cfg.Server.URL != "http://localhost:8080" {
		t.Fatalf("expected default URL, got %s", cfg.Server.URL)
	}
}

func TestConfigSetGet(t *testing.T) {
	cfg := DefaultConfig()

	if err := cfg.Set("server.url", "http://test:8080"); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	val, err := cfg.Get("server.url")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if val != "http://test:8080" {
		t.Fatalf("expected http://test:8080, got %s", val)
	}

	if err := cfg.Set("unknown.key", "value"); err == nil {
		t.Fatal("expected error for unknown key")
	}

	if _, err := cfg.Get("unknown.key"); err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestConfigSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "client.toml")

	cfg := DefaultConfig()
	cfg.Server.URL = "http://saved:8080"
	cfg.Server.APIKey = "saved-key"

	if err := cfg.SaveTo(configPath); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if loaded.Server.URL != "http://saved:8080" {
		t.Fatalf("expected URL http://saved:8080, got %s", loaded.Server.URL)
	}
	if loaded.Server.APIKey != "saved-key" {
		t.Fatalf("expected API key saved-key, got %s", loaded.Server.APIKey)
	}
}
