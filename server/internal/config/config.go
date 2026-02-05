package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Config holds the complete server configuration.
type Config struct {
	Server     ServerConfig     `toml:"server"`
	Scanner    ScannerConfig    `toml:"scanner"`
	Button     ButtonConfig     `toml:"button"`
	Processing ProcessingConfig `toml:"processing"`
	Storage    StorageConfig    `toml:"storage"`
	Output     OutputConfig     `toml:"output"`
	Logging    LoggingConfig    `toml:"logging"`
}

type ServerConfig struct {
	Host    string     `toml:"host"`
	Port    int        `toml:"port"`
	BaseURL string     `toml:"base_url"`
	Auth    AuthConfig `toml:"auth"`
	TLS     TLSConfig  `toml:"tls"`
}

type AuthConfig struct {
	Enabled           bool     `toml:"enabled"`
	APIKeys           []string `toml:"api_keys"`
	BasicAuthUser     string   `toml:"basic_auth_user"`
	BasicAuthPassHash string   `toml:"basic_auth_password_hash"`
}

type TLSConfig struct {
	Enabled  bool   `toml:"enabled"`
	CertFile string `toml:"cert_file"`
	KeyFile  string `toml:"key_file"`
}

type ScannerConfig struct {
	Device   string         `toml:"device"`
	AutoOpen bool           `toml:"auto_open"`
	Defaults ScannerDefaults `toml:"defaults"`
}

type ScannerDefaults struct {
	Resolution int     `toml:"resolution"`
	Mode       string  `toml:"mode"`
	Source     string  `toml:"source"`
	PageWidth  float64 `toml:"page_width"`
	PageHeight float64 `toml:"page_height"`
}

type ButtonConfig struct {
	Enabled           bool           `toml:"enabled"`
	PollInterval      duration       `toml:"poll_interval"`
	LongPressDuration duration       `toml:"long_press_duration"`
	ShortPressProfile string         `toml:"short_press_profile"`
	LongPressProfile  string         `toml:"long_press_profile"`
	Output            string         `toml:"output"`
	BeepOnLongPress   bool           `toml:"beep_on_long_press"`
	Metadata          MetadataConfig `toml:"metadata"`
}

type MetadataConfig struct {
	TitlePattern  string `toml:"title_pattern"`
	Correspondent int    `toml:"correspondent"`
	DocumentType  int    `toml:"document_type"`
	Tags          []int  `toml:"tags"`
}

type ProcessingConfig struct {
	TempDirectory     string    `toml:"temp_directory"`
	MaxConcurrentJobs int       `toml:"max_concurrent_jobs"`
	PDF               PDFConfig `toml:"pdf"`
	OCR               OCRConfig `toml:"ocr"`
}

type PDFConfig struct {
	Format      string `toml:"format"`
	Compression string `toml:"compression"`
	JPEGQuality int    `toml:"jpeg_quality"`
}

type OCRConfig struct {
	Enabled       bool   `toml:"enabled"`
	Language      string `toml:"language"`
	TesseractPath string `toml:"tesseract_path"`
}

type StorageConfig struct {
	LocalDirectory string `toml:"local_directory"`
	RetentionDays  int    `toml:"retention_days"`
}

type OutputConfig struct {
	Paperless        PaperlessConfig        `toml:"paperless"`
	SMB              SMBConfig              `toml:"smb"`
	PaperlessConsume PaperlessConsumeConfig `toml:"paperless_consume"`
	Email            EmailConfig            `toml:"email"`
}

type PaperlessConfig struct {
	Enabled              bool   `toml:"enabled"`
	URL                  string `toml:"url"`
	TokenFile            string `toml:"token_file"`
	Token                string `toml:"token"`
	VerifySSL            bool   `toml:"verify_ssl"`
	DefaultCorrespondent int    `toml:"default_correspondent"`
	DefaultDocumentType  int    `toml:"default_document_type"`
	DefaultTags          []int  `toml:"default_tags"`
}

type SMBConfig struct {
	Enabled         bool   `toml:"enabled"`
	Server          string `toml:"server"`
	Share           string `toml:"share"`
	Username        string `toml:"username"`
	PasswordFile    string `toml:"password_file"`
	Directory       string `toml:"directory"`
	FilenamePattern string `toml:"filename_pattern"`
}

type PaperlessConsumeConfig struct {
	Enabled   bool   `toml:"enabled"`
	Path      string `toml:"path"`
	SMBServer string `toml:"smb_server"`
}

type EmailConfig struct {
	Enabled          bool   `toml:"enabled"`
	SMTPHost         string `toml:"smtp_host"`
	SMTPPort         int    `toml:"smtp_port"`
	SMTPUser         string `toml:"smtp_user"`
	SMTPPasswordFile string `toml:"smtp_password_file"`
	FromAddress      string `toml:"from_address"`
	DefaultRecipient string `toml:"default_recipient"`
}

type LoggingConfig struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
	File   string `toml:"file"`
}

// duration wraps time.Duration for TOML unmarshaling.
type duration time.Duration

func (d *duration) UnmarshalText(text []byte) error {
	dur, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = duration(dur)
	return nil
}

func (d duration) Duration() time.Duration {
	return time.Duration(d)
}

// Load reads and parses the server configuration from a TOML file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.loadSecrets(); err != nil {
		return nil, fmt.Errorf("load secrets: %w", err)
	}

	return cfg, nil
}

// DefaultConfig returns the configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Scanner: ScannerConfig{
			AutoOpen: true,
			Defaults: ScannerDefaults{
				Resolution: 300,
				Mode:       "color",
				Source:     "adf_duplex",
				PageWidth:  210.0,
				PageHeight: 297.0,
			},
		},
		Button: ButtonConfig{
			PollInterval:      duration(50 * time.Millisecond),
			LongPressDuration: duration(1 * time.Second),
			ShortPressProfile: "standard",
			LongPressProfile:  "oversize",
			Output:            "paperless",
			Metadata: MetadataConfig{
				TitlePattern: "Scan_{date}_{time}",
			},
		},
		Processing: ProcessingConfig{
			TempDirectory:     "/tmp/scanflow",
			MaxConcurrentJobs: 2,
			PDF: PDFConfig{
				Format:      "PDF/A-2b",
				Compression: "jpeg",
				JPEGQuality: 85,
			},
			OCR: OCRConfig{
				Enabled:       true,
				Language:      "deu+eng",
				TesseractPath: "/usr/bin/tesseract",
			},
		},
		Storage: StorageConfig{
			LocalDirectory: "/var/lib/scanflow/documents",
			RetentionDays:  30,
		},
		Output: OutputConfig{
			Paperless: PaperlessConfig{
				VerifySSL: true,
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

// loadSecrets reads secret values from files.
func (c *Config) loadSecrets() error {
	if c.Output.Paperless.TokenFile != "" && c.Output.Paperless.Token == "" {
		token, err := readSecretFile(c.Output.Paperless.TokenFile)
		if err != nil && c.Output.Paperless.Enabled {
			return fmt.Errorf("paperless token: %w", err)
		}
		c.Output.Paperless.Token = token
	}
	return nil
}

func readSecretFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
