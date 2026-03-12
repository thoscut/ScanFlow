//go:build windows

package service

import (
	"os"
	"strings"
	"testing"
)

func TestWindowsDefaults(t *testing.T) {
	opts := (Options{ServiceName: "scanflow"}).WithDefaults()

	programFiles := os.Getenv("ProgramFiles")
	programData := os.Getenv("ProgramData")

	if !strings.Contains(opts.BinaryPath, "ScanFlow") {
		t.Fatalf("unexpected binary path: %s", opts.BinaryPath)
	}
	if programFiles != "" && !strings.HasPrefix(opts.BinaryPath, programFiles) {
		t.Fatalf("binary path should start with ProgramFiles: %s", opts.BinaryPath)
	}
	if programData != "" && !strings.HasPrefix(opts.ConfigPath, programData) {
		t.Fatalf("config path should start with ProgramData: %s", opts.ConfigPath)
	}
	if programData != "" && !strings.HasPrefix(opts.DataDir, programData) {
		t.Fatalf("data dir should start with ProgramData: %s", opts.DataDir)
	}
}
