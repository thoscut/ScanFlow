package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/thoscut/scanflow/server/internal/config"
)

// Options configures service installation paths and metadata.
type Options struct {
	ServiceName string
	Description string
	User        string
	Group       string
	BinaryPath  string
	ConfigPath  string
	UnitPath    string
	DataDir     string
	LogDir      string
	TempDir     string
	StartNow    bool
}

// WithDefaults fills unset fields with the project's standard Linux/systemd locations.
func (o Options) WithDefaults() Options {
	name := strings.TrimSuffix(o.ServiceName, ".service")
	if name == "" {
		name = "scanflow"
	}
	o.ServiceName = name
	if o.Description == "" {
		o.Description = "ScanFlow Scanner Server"
	}
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

// Install installs the current binary as a Linux systemd service.
func Install(binarySource string, opts Options) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("service installation is currently supported on Linux/systemd only")
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("service installation requires root privileges")
	}

	opts = opts.WithDefaults()
	if err := validatePaths(binarySource, opts); err != nil {
		return err
	}

	for _, dir := range []string{
		filepath.Dir(opts.BinaryPath),
		filepath.Dir(opts.ConfigPath),
		filepath.Join(filepath.Dir(opts.ConfigPath), "profiles"),
		opts.DataDir,
		opts.LogDir,
		opts.TempDir,
	} {
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
	if err := os.WriteFile(opts.UnitPath, []byte(renderSystemdUnit(opts)), 0o644); err != nil {
		return fmt.Errorf("write service unit: %w", err)
	}

	if err := runSystemctl("daemon-reload"); err != nil {
		return fmt.Errorf("reload systemd: %w", err)
	}

	enableArgs := []string{"enable", opts.ServiceName}
	if opts.StartNow {
		enableArgs = []string{"enable", "--now", opts.ServiceName}
	}
	if err := runSystemctl(enableArgs...); err != nil {
		return fmt.Errorf("enable service: %w", err)
	}

	return nil
}

// Uninstall removes the Linux systemd service unit and installed binary.
func Uninstall(opts Options) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("service removal is currently supported on Linux/systemd only")
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("service removal requires root privileges")
	}

	opts = opts.WithDefaults()

	if _, err := os.Stat(opts.UnitPath); err == nil {
		if err := runSystemctl("disable", "--now", opts.ServiceName); err != nil {
			return fmt.Errorf("disable service: %w", err)
		}
	}

	if err := os.Remove(opts.UnitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove service unit: %w", err)
	}
	if err := os.Remove(opts.BinaryPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove installed binary: %w", err)
	}

	if err := runSystemctl("daemon-reload"); err != nil {
		return fmt.Errorf("reload systemd: %w", err)
	}

	return nil
}

func validatePaths(binarySource string, opts Options) error {
	paths := []string{binarySource, opts.BinaryPath, opts.ConfigPath, opts.UnitPath, opts.DataDir, opts.LogDir, opts.TempDir}
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("path must be absolute: %s", path)
		}
	}
	return nil
}

func ensureConfig(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	data, err := toml.Marshal(config.DefaultConfig())
	if err != nil {
		return fmt.Errorf("marshal default config: %w", err)
	}
	return os.WriteFile(path, data, 0o640)
}

func copyExecutable(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755)
}

func renderSystemdUnit(opts Options) string {
	opts = opts.WithDefaults()
	return fmt.Sprintf(`[Unit]
Description=%s
After=network.target
Documentation=https://github.com/thoscut/ScanFlow

[Service]
Type=simple
User=%s
Group=%s
ExecStart=%s -config %s
Restart=always
RestartSec=5

# Environment
Environment=SCANFLOW_LOG_LEVEL=info

# Security
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=%s %s %s

# Resource limits
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
`, opts.Description, opts.User, opts.Group, opts.BinaryPath, opts.ConfigPath, opts.DataDir, opts.LogDir, opts.TempDir)
}

func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
