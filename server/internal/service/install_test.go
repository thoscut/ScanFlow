package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOptionsWithDefaults(t *testing.T) {
	opts := (Options{ServiceName: "scanflow.service"}).WithDefaults()

	if opts.ServiceName != "scanflow" {
		t.Fatalf("expected normalized service name, got %q", opts.ServiceName)
	}
	if opts.UnitPath != "/etc/systemd/system/scanflow.service" {
		t.Fatalf("unexpected unit path: %s", opts.UnitPath)
	}
	if opts.BinaryPath != "/opt/scanflow/scanflow-server" {
		t.Fatalf("unexpected binary path: %s", opts.BinaryPath)
	}
}

func TestRenderSystemdUnitUsesConfiguredPaths(t *testing.T) {
	unit := renderSystemdUnit(Options{
		ServiceName: "scanflow",
		User:        "scanner",
		Group:       "scanner",
		BinaryPath:  "/opt/scanflow/scanflow-server",
		ConfigPath:  "/etc/scanflow/server.toml",
		DataDir:     "/var/lib/scanflow",
		LogDir:      "/var/log/scanflow",
		TempDir:     "/tmp/scanflow",
	})

	for _, want := range []string{
		"ExecStart=/opt/scanflow/scanflow-server -config /etc/scanflow/server.toml",
		"User=scanner",
		"Group=scanner",
		"ReadWritePaths=/var/lib/scanflow /var/log/scanflow /tmp/scanflow",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("expected unit to contain %q\n%s", want, unit)
		}
	}
}

func TestEnsureConfigWritesDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.toml")

	if err := ensureConfig(path); err != nil {
		t.Fatalf("ensureConfig failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "port = 8080") {
		t.Fatalf("expected default config content, got:\n%s", content)
	}
	if !strings.Contains(content, "long_press_profile = 'oversize'") {
		t.Fatalf("expected button defaults in config, got:\n%s", content)
	}
	if !strings.Contains(content, "poll_interval = '50ms'") {
		t.Fatalf("expected human-readable durations in config, got:\n%s", content)
	}
}
