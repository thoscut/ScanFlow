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
	if opts.Description != "ScanFlow Scanner Server" {
		t.Fatalf("expected default description, got %q", opts.Description)
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

func TestEnsureConfigSkipsExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.toml")

	original := []byte("existing-content")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	if err := ensureConfig(path); err != nil {
		t.Fatalf("ensureConfig failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "existing-content" {
		t.Fatal("ensureConfig overwrote existing file")
	}
}

func TestCopyExecutable(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")

	if err := os.WriteFile(src, []byte("binary-data"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if err := copyExecutable(src, dst); err != nil {
		t.Fatalf("copy failed: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(data) != "binary-data" {
		t.Fatal("copied content mismatch")
	}
}

func TestValidatePaths(t *testing.T) {
	opts := Options{
		BinaryPath: "/opt/scanflow/server",
		ConfigPath: "/etc/scanflow/server.toml",
		DataDir:    "/var/lib/scanflow",
		LogDir:     "/var/log/scanflow",
		TempDir:    "/tmp/scanflow",
	}

	if err := validatePaths("/usr/bin/scanflow", opts); err != nil {
		t.Fatalf("expected valid paths: %v", err)
	}

	opts.DataDir = "relative/path"
	if err := validatePaths("/usr/bin/scanflow", opts); err == nil {
		t.Fatal("expected error for relative path")
	}
}

func TestInstallDirs(t *testing.T) {
	opts := Options{
		BinaryPath: "/opt/scanflow/scanflow-server",
		ConfigPath: "/etc/scanflow/server.toml",
		DataDir:    "/var/lib/scanflow",
		LogDir:     "/var/log/scanflow",
		TempDir:    "/tmp/scanflow",
	}

	dirs := installDirs(opts)
	if len(dirs) != 6 {
		t.Fatalf("expected 6 directories, got %d", len(dirs))
	}

	// Should contain profiles subdirectory
	found := false
	for _, d := range dirs {
		if strings.Contains(d, "profiles") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected profiles directory in installDirs")
	}
}
