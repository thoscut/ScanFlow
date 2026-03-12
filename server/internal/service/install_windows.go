//go:build windows

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func (o Options) withPlatformDefaults() Options {
	if o.BinaryPath == "" {
		o.BinaryPath = filepath.Join(os.Getenv("ProgramFiles"), "ScanFlow", "scanflow-server.exe")
	}
	if o.ConfigPath == "" {
		o.ConfigPath = filepath.Join(os.Getenv("ProgramData"), "ScanFlow", "server.toml")
	}
	if o.DataDir == "" {
		o.DataDir = filepath.Join(os.Getenv("ProgramData"), "ScanFlow", "data")
	}
	if o.LogDir == "" {
		o.LogDir = filepath.Join(os.Getenv("ProgramData"), "ScanFlow", "logs")
	}
	if o.TempDir == "" {
		o.TempDir = filepath.Join(os.TempDir(), "scanflow")
	}
	return o
}

// Install registers the current binary as a Windows service using sc.exe.
func Install(binarySource string, opts Options) error {
	opts = opts.WithDefaults()
	if err := validatePaths(binarySource, opts); err != nil {
		return err
	}

	for _, dir := range installDirs(opts) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	if err := copyExecutable(binarySource, opts.BinaryPath); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	if err := ensureConfig(opts.ConfigPath); err != nil {
		return fmt.Errorf("install config: %w", err)
	}

	binPath := fmt.Sprintf(`"%s" -config "%s"`, opts.BinaryPath, opts.ConfigPath)
	cmd := exec.Command("sc.exe", "create", opts.ServiceName,
		"binPath=", binPath,
		"DisplayName=", opts.Description,
		"start=", "auto")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create Windows service: %w", err)
	}

	// Set service description.
	descCmd := exec.Command("sc.exe", "description", opts.ServiceName, opts.Description)
	descCmd.Stdout = os.Stdout
	descCmd.Stderr = os.Stderr
	descCmd.Run() // best-effort

	if opts.StartNow {
		startCmd := exec.Command("sc.exe", "start", opts.ServiceName)
		startCmd.Stdout = os.Stdout
		startCmd.Stderr = os.Stderr
		if err := startCmd.Run(); err != nil {
			return fmt.Errorf("start service: %w", err)
		}
	}

	return nil
}

// Uninstall stops and removes the Windows service and installed binary.
func Uninstall(opts Options) error {
	opts = opts.WithDefaults()

	// Stop service (best-effort).
	stopCmd := exec.Command("sc.exe", "stop", opts.ServiceName)
	stopCmd.Stdout = os.Stdout
	stopCmd.Stderr = os.Stderr
	stopCmd.Run()

	// Delete service.
	cmd := exec.Command("sc.exe", "delete", opts.ServiceName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("delete Windows service: %w", err)
	}

	if err := os.Remove(opts.BinaryPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove installed binary: %w", err)
	}

	return nil
}
