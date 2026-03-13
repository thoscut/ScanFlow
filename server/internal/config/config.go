package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// ocrLangPattern validates OCR language strings like "deu", "deu+eng", "chi_sim+eng".
var ocrLangPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+(\+[a-zA-Z0-9_]+)*$`)

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
	Enabled  bool       `toml:"enabled"`
	CertFile string     `toml:"cert_file"`
	KeyFile  string     `toml:"key_file"`
	ACME     ACMEConfig `toml:"acme"`
}

// ACMEConfig configures automatic certificate management via Let's Encrypt.
type ACMEConfig struct {
	Enabled            bool     `toml:"enabled"`
	Email              string   `toml:"email"`
	Domains            []string `toml:"domains"`
	Challenge          string   `toml:"challenge"`            // "http" or "dns"
	CertDir            string   `toml:"cert_dir"`
	DirectoryURL       string   `toml:"directory_url"`        // empty = Let's Encrypt production
	DNSProvider        string   `toml:"dns_provider"`         // "cloudflare", "duckdns", "route53", "exec"
	DNSPropagationWait duration `toml:"dns_propagation_wait"` // time to wait for DNS propagation

	// Provider-specific DNS settings.
	Cloudflare ACMECloudflareConfig `toml:"cloudflare"`
	DuckDNS    ACMEDuckDNSConfig    `toml:"duckdns"`
	Route53    ACMERoute53Config    `toml:"route53"`
	Exec       ACMEExecConfig       `toml:"exec"`
}

// ACMECloudflareConfig holds Cloudflare DNS settings.
type ACMECloudflareConfig struct {
	APITokenFile string `toml:"api_token_file"`
	ZoneID       string `toml:"zone_id"` // optional, auto-detected if empty
}

// ACMEDuckDNSConfig holds DuckDNS settings.
type ACMEDuckDNSConfig struct {
	TokenFile string `toml:"token_file"`
}

// ACMERoute53Config holds AWS Route 53 settings.
type ACMERoute53Config struct {
	AccessKeyID        string `toml:"access_key_id"`
	SecretAccessKeyFile string `toml:"secret_access_key_file"`
	HostedZoneID       string `toml:"hosted_zone_id"`
	Region             string `toml:"region"`
}

// ACMEExecConfig holds settings for an external DNS challenge script.
type ACMEExecConfig struct {
	CreateCommand  string `toml:"create_command"`  // called with: <domain> <token> <key_auth>
	CleanupCommand string `toml:"cleanup_command"` // called with: <domain> <token> <key_auth>
}

type ScannerConfig struct {
	Device   string          `toml:"device"`
	AutoOpen bool            `toml:"auto_open"`
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
	TempDirectory     string          `toml:"temp_directory"`
	MaxConcurrentJobs int             `toml:"max_concurrent_jobs"`
	JobTimeout        duration        `toml:"job_timeout"`
	PDF               PDFConfig       `toml:"pdf"`
	OCR               OCRConfig       `toml:"ocr"`
	ImageFilters      ImageFilterConfig `toml:"image_filters"`
}

// ImageFilterConfig controls optional image filters applied during the
// post-processing pipeline before PDF creation.
type ImageFilterConfig struct {
	AutoRotate         bool    `toml:"auto_rotate"`
	ColorToGrayscale   bool    `toml:"color_to_grayscale"`
	BrightnessAdjust   float64 `toml:"brightness_adjust"`
	ContrastAdjust     float64 `toml:"contrast_adjust"`
	NormalizeExposure  bool    `toml:"normalize_exposure"`
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

func (d duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// Validate checks the configuration for invalid or missing values and returns
// all detected problems joined into a single error so the operator sees every
// issue at once.
func (c *Config) Validate() error {
	var errs []error

	// Server.Port
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Errorf("server.port must be between 1 and 65535, got %d", c.Server.Port))
	}

	// Processing.TempDirectory
	if strings.TrimSpace(c.Processing.TempDirectory) == "" {
		errs = append(errs, fmt.Errorf("processing.temp_directory must not be empty"))
	}

	// Processing.MaxConcurrentJobs
	if c.Processing.MaxConcurrentJobs < 1 {
		errs = append(errs, fmt.Errorf("processing.max_concurrent_jobs must be >= 1, got %d", c.Processing.MaxConcurrentJobs))
	}

	// Processing.PDF.JPEGQuality
	if c.Processing.PDF.JPEGQuality < 1 || c.Processing.PDF.JPEGQuality > 100 {
		errs = append(errs, fmt.Errorf("processing.pdf.jpeg_quality must be between 1 and 100, got %d", c.Processing.PDF.JPEGQuality))
	}

	// Processing.OCR.Language (only when OCR is enabled)
	if c.Processing.OCR.Enabled && c.Processing.OCR.Language != "" {
		if !ocrLangPattern.MatchString(c.Processing.OCR.Language) {
			errs = append(errs, fmt.Errorf("processing.ocr.language contains invalid characters: %q", c.Processing.OCR.Language))
		}
	}

	// TLS without ACME requires cert and key files
	if c.Server.TLS.Enabled && !c.Server.TLS.ACME.Enabled {
		if strings.TrimSpace(c.Server.TLS.CertFile) == "" {
			errs = append(errs, fmt.Errorf("server.tls.cert_file must not be empty when TLS is enabled without ACME"))
		}
		if strings.TrimSpace(c.Server.TLS.KeyFile) == "" {
			errs = append(errs, fmt.Errorf("server.tls.key_file must not be empty when TLS is enabled without ACME"))
		}
	}

	// ACME requires at least one domain and a non-empty email
	if c.Server.TLS.ACME.Enabled {
		if len(c.Server.TLS.ACME.Domains) == 0 {
			errs = append(errs, fmt.Errorf("server.tls.acme.domains must contain at least one domain when ACME is enabled"))
		}
		if strings.TrimSpace(c.Server.TLS.ACME.Email) == "" {
			errs = append(errs, fmt.Errorf("server.tls.acme.email must not be empty when ACME is enabled"))
		}
	}

	// Logging.Level
	switch c.Logging.Level {
	case "debug", "info", "warn", "error":
		// valid
	default:
		errs = append(errs, fmt.Errorf("logging.level must be one of debug, info, warn, error; got %q", c.Logging.Level))
	}

	// Logging.Format
	switch c.Logging.Format {
	case "json", "text":
		// valid
	default:
		errs = append(errs, fmt.Errorf("logging.format must be one of json, text; got %q", c.Logging.Format))
	}

	return errors.Join(errs...)
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

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

// DefaultConfig returns the configuration with sensible defaults.
func DefaultConfig() *Config {
	tempDir := "/tmp/scanflow"
	tesseractPath := "/usr/bin/tesseract"
	localDir := "/var/lib/scanflow/documents"
	if runtime.GOOS == "windows" {
		tempDir = os.TempDir() + `\scanflow`
		tesseractPath = "tesseract"
		localDir = os.Getenv("ProgramData") + `\ScanFlow\documents`
	}

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
			TempDirectory:     tempDir,
			MaxConcurrentJobs: 2,
			JobTimeout:        duration(10 * time.Minute),
			PDF: PDFConfig{
				Format:      "PDF/A-2b",
				Compression: "jpeg",
				JPEGQuality: 85,
			},
			OCR: OCRConfig{
				Enabled:       true,
				Language:      "deu+eng",
				TesseractPath: tesseractPath,
			},
		},
		Storage: StorageConfig{
			LocalDirectory: localDir,
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
