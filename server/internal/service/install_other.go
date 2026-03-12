//go:build !linux && !windows

package service

import (
	"fmt"
	"path/filepath"
)

func (o Options) withPlatformDefaults() Options {
	if o.User == "" {
		o.User = "scanner"
	}
	if o.Group == "" {
		o.Group = o.User
	}
	if o.BinaryPath == "" {
		o.BinaryPath = "/opt/scanflow/scanflow-server"
	}
	if o.ConfigPath == "" {
		o.ConfigPath = "/etc/scanflow/server.toml"
	}
	if o.UnitPath == "" {
		o.UnitPath = filepath.Join("/etc/systemd/system", o.ServiceName+".service")
	}
	if o.DataDir == "" {
		o.DataDir = "/var/lib/scanflow"
	}
	if o.LogDir == "" {
		o.LogDir = "/var/log/scanflow"
	}
	if o.TempDir == "" {
		o.TempDir = "/tmp/scanflow"
	}
	return o
}

// Install is not supported on this platform.
func Install(_ string, _ Options) error {
	return fmt.Errorf("service installation is not supported on this platform")
}

// Uninstall is not supported on this platform.
func Uninstall(_ Options) error {
	return fmt.Errorf("service removal is not supported on this platform")
}
