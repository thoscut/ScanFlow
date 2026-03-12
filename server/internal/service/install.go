package service

import (
	"fmt"
	"os"
	"path/filepath"
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
	UnitPath    string // Linux/systemd only
	DataDir     string
	LogDir      string
	TempDir     string
	StartNow    bool
}

// WithDefaults fills unset fields with platform-appropriate defaults.
func (o Options) WithDefaults() Options {
	name := strings.TrimSuffix(o.ServiceName, ".service")
	if name == "" {
		name = "scanflow"
	}
	o.ServiceName = name
	if o.Description == "" {
		o.Description = "ScanFlow Scanner Server"
	}
	return o.withPlatformDefaults()
}

func validatePaths(binarySource string, opts Options) error {
	paths := []string{binarySource, opts.BinaryPath, opts.ConfigPath, opts.DataDir, opts.LogDir, opts.TempDir}
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

func installDirs(opts Options) []string {
	return []string{
		filepath.Dir(opts.BinaryPath),
		filepath.Dir(opts.ConfigPath),
		filepath.Join(filepath.Dir(opts.ConfigPath), "profiles"),
		opts.DataDir,
		opts.LogDir,
		opts.TempDir,
	}
}
