package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Config holds the client configuration.
type Config struct {
	Server   ServerConfig   `toml:"server"`
	Defaults DefaultsConfig `toml:"defaults"`
	TUI      TUIConfig      `toml:"tui"`
}

type ServerConfig struct {
	URL        string `toml:"url"`
	APIKey     string `toml:"api_key"`
	APIKeyFile string `toml:"api_key_file"`
}

type DefaultsConfig struct {
	Profile     string `toml:"profile"`
	Output      string `toml:"output"`
	Interactive bool   `toml:"interactive"`
}

type TUIConfig struct {
	Theme          string `toml:"theme"`
	PreviewQuality string `toml:"preview_quality"`
}

// Load reads the client configuration from the default location.
func Load() (*Config, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}

	path := filepath.Join(configDir, "scanflow", "client.toml")
	return LoadFrom(path)
}

// LoadFrom reads the client configuration from a specific file.
func LoadFrom(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Load API key from file if specified
	if cfg.Server.APIKeyFile != "" && cfg.Server.APIKey == "" {
		keyData, err := os.ReadFile(expandPath(cfg.Server.APIKeyFile))
		if err == nil {
			cfg.Server.APIKey = strings.TrimSpace(string(keyData))
		}
	}

	return cfg, nil
}

// DefaultConfig returns the default client configuration.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			URL: "http://localhost:8080",
		},
		Defaults: DefaultsConfig{
			Profile: "standard",
			Output:  "paperless",
		},
		TUI: TUIConfig{
			Theme:          "dark",
			PreviewQuality: "medium",
		},
	}
}

// Save writes the configuration to the default location.
func (c *Config) Save() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}

	dir := filepath.Join(configDir, "scanflow")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	path := filepath.Join(dir, "client.toml")
	return c.SaveTo(path)
}

// SaveTo writes the configuration to a specific file.
func (c *Config) SaveTo(path string) error {
	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// Set updates a configuration value by dotted key path.
func (c *Config) Set(key, value string) error {
	switch key {
	case "server.url":
		c.Server.URL = value
	case "server.api_key":
		c.Server.APIKey = value
	case "defaults.profile":
		c.Defaults.Profile = value
	case "defaults.output":
		c.Defaults.Output = value
	case "tui.theme":
		c.TUI.Theme = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	return nil
}

// Get returns a configuration value by dotted key path.
func (c *Config) Get(key string) (string, error) {
	switch key {
	case "server.url":
		return c.Server.URL, nil
	case "server.api_key":
		return c.Server.APIKey, nil
	case "defaults.profile":
		return c.Defaults.Profile, nil
	case "defaults.output":
		return c.Defaults.Output, nil
	case "tui.theme":
		return c.TUI.Theme, nil
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
