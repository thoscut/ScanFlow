//go:build linux

package service

import (
	"strings"
	"testing"
)

func TestLinuxDefaults(t *testing.T) {
	opts := (Options{ServiceName: "scanflow.service"}).WithDefaults()

	if opts.UnitPath != "/etc/systemd/system/scanflow.service" {
		t.Fatalf("unexpected unit path: %s", opts.UnitPath)
	}
	if opts.BinaryPath != "/opt/scanflow/scanflow-server" {
		t.Fatalf("unexpected binary path: %s", opts.BinaryPath)
	}
	if opts.User != "scanner" {
		t.Fatalf("unexpected user: %s", opts.User)
	}
	if opts.Group != "scanner" {
		t.Fatalf("unexpected group: %s", opts.Group)
	}
	if opts.ConfigPath != "/etc/scanflow/server.toml" {
		t.Fatalf("unexpected config path: %s", opts.ConfigPath)
	}
	if opts.DataDir != "/var/lib/scanflow" {
		t.Fatalf("unexpected data dir: %s", opts.DataDir)
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

func TestRenderSystemdUnitIncludesSecurityDirectives(t *testing.T) {
	unit := renderSystemdUnit(Options{
		ServiceName: "scanflow",
		BinaryPath:  "/opt/scanflow/scanflow-server",
		ConfigPath:  "/etc/scanflow/server.toml",
		DataDir:     "/var/lib/scanflow",
		LogDir:      "/var/log/scanflow",
		TempDir:     "/tmp/scanflow",
	})

	for _, want := range []string{
		"NoNewPrivileges=true",
		"ProtectSystem=strict",
		"ProtectHome=true",
		"LimitNOFILE=65536",
		"Restart=always",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("expected unit to contain %q\n%s", want, unit)
		}
	}
}
