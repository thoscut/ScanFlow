package acme

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/thoscut/scanflow/server/internal/config"
)

func TestNewManagerHTTP(t *testing.T) {
	dir := t.TempDir()

	cfg := config.ACMEConfig{
		Enabled:   true,
		Email:     "test@example.com",
		Domains:   []string{"scanner.example.com"},
		Challenge: "http",
		CertDir:   dir,
	}

	mgr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if mgr.httpMgr == nil {
		t.Fatal("expected httpMgr to be set for HTTP challenge")
	}
	if mgr.dnsSolver != nil {
		t.Fatal("expected dnsSolver to be nil for HTTP challenge")
	}

	// TLS config should be non-nil.
	tlsCfg := mgr.TLSConfig()
	if tlsCfg == nil {
		t.Fatal("expected non-nil TLS config")
	}

	// HTTP handler should be non-nil for HTTP-01.
	handler := mgr.HTTPHandler(nil)
	if handler == nil {
		t.Fatal("expected non-nil HTTP handler for HTTP-01 challenge")
	}
}

func TestNewManagerDNSCloudflare(t *testing.T) {
	dir := t.TempDir()

	// Write a fake token file.
	tokenFile := filepath.Join(dir, "cf_token")
	if err := os.WriteFile(tokenFile, []byte("fake-cf-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.ACMEConfig{
		Enabled:     true,
		Email:       "test@example.com",
		Domains:     []string{"scanner.example.com"},
		Challenge:   "dns",
		CertDir:     dir,
		DNSProvider: "cloudflare",
		Cloudflare: config.ACMECloudflareConfig{
			APITokenFile: tokenFile,
		},
	}

	mgr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if mgr.dnsSolver == nil {
		t.Fatal("expected dnsSolver to be set for DNS challenge")
	}
	if mgr.httpMgr != nil {
		t.Fatal("expected httpMgr to be nil for DNS challenge")
	}

	// HTTP handler should be nil for DNS-01.
	handler := mgr.HTTPHandler(nil)
	if handler != nil {
		t.Fatal("expected nil HTTP handler for DNS-01 challenge")
	}
}

func TestNewManagerDNSDuckDNS(t *testing.T) {
	dir := t.TempDir()

	tokenFile := filepath.Join(dir, "duck_token")
	if err := os.WriteFile(tokenFile, []byte("fake-duck-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.ACMEConfig{
		Enabled:     true,
		Email:       "test@example.com",
		Domains:     []string{"myscanner.duckdns.org"},
		Challenge:   "dns",
		CertDir:     dir,
		DNSProvider: "duckdns",
		DuckDNS: config.ACMEDuckDNSConfig{
			TokenFile: tokenFile,
		},
	}

	mgr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if mgr.dnsSolver == nil {
		t.Fatal("expected dnsSolver to be set")
	}
}

func TestNewManagerDNSRoute53(t *testing.T) {
	dir := t.TempDir()

	secretFile := filepath.Join(dir, "aws_secret")
	if err := os.WriteFile(secretFile, []byte("fake-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.ACMEConfig{
		Enabled:     true,
		Email:       "test@example.com",
		Domains:     []string{"scanner.example.com"},
		Challenge:   "dns",
		CertDir:     dir,
		DNSProvider: "route53",
		Route53: config.ACMERoute53Config{
			AccessKeyID:        "AKID123",
			SecretAccessKeyFile: secretFile,
			HostedZoneID:       "Z123",
			Region:             "eu-central-1",
		},
	}

	mgr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if mgr.dnsSolver == nil {
		t.Fatal("expected dnsSolver to be set")
	}
}

func TestNewManagerDNSExec(t *testing.T) {
	dir := t.TempDir()

	cfg := config.ACMEConfig{
		Enabled:     true,
		Email:       "test@example.com",
		Domains:     []string{"scanner.example.com"},
		Challenge:   "dns",
		CertDir:     dir,
		DNSProvider: "exec",
		Exec: config.ACMEExecConfig{
			CreateCommand:  "/usr/local/bin/dns-create",
			CleanupCommand: "/usr/local/bin/dns-cleanup",
		},
	}

	mgr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if mgr.dnsSolver == nil {
		t.Fatal("expected dnsSolver to be set")
	}
}

func TestNewManagerInvalidChallenge(t *testing.T) {
	dir := t.TempDir()
	cfg := config.ACMEConfig{
		Enabled:   true,
		Challenge: "invalid",
		CertDir:   dir,
	}

	_, err := New(cfg)
	if err == nil {
		t.Fatal("expected error for invalid challenge type")
	}
}

func TestNewManagerDNSMissingProvider(t *testing.T) {
	dir := t.TempDir()
	cfg := config.ACMEConfig{
		Enabled:   true,
		Challenge: "dns",
		CertDir:   dir,
	}

	_, err := New(cfg)
	if err == nil {
		t.Fatal("expected error for missing DNS provider")
	}
}

func TestNewManagerDNSUnsupportedProvider(t *testing.T) {
	dir := t.TempDir()
	cfg := config.ACMEConfig{
		Enabled:     true,
		Challenge:   "dns",
		CertDir:     dir,
		DNSProvider: "godaddy",
	}

	_, err := New(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported DNS provider")
	}
}

func TestLoadOrCreateKey(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "test.key")

	// Create a new key.
	key1, err := loadOrCreateKey(keyFile)
	if err != nil {
		t.Fatalf("loadOrCreateKey() error: %v", err)
	}
	if key1 == nil {
		t.Fatal("expected non-nil key")
	}

	// Verify key was written to disk.
	data, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("read key file: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "EC PRIVATE KEY" {
		t.Fatal("expected EC PRIVATE KEY PEM block")
	}

	// Load existing key.
	key2, err := loadOrCreateKey(keyFile)
	if err != nil {
		t.Fatalf("loadOrCreateKey() second call error: %v", err)
	}

	// Keys should match.
	if !key1.Equal(key2) {
		t.Fatal("expected same key on second load")
	}
}

func TestSaveCertificates(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")

	// Generate a self-signed cert for testing.
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{
		SerialNumber: bigInt(1),
		DNSNames:     []string{"test.example.com"},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create test cert: %v", err)
	}

	if err := saveCertificates(certFile, [][]byte{certDER}); err != nil {
		t.Fatalf("saveCertificates: %v", err)
	}

	// Verify the file contains a valid PEM cert.
	data, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatal("expected CERTIFICATE PEM block")
	}

	// Verify permissions are restrictive.
	info, _ := os.Stat(certFile)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600 permissions, got %o", info.Mode().Perm())
	}
}

func TestChallengeRecordName(t *testing.T) {
	if got := challengeRecordName("example.com"); got != "_acme-challenge.example.com" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractDuckDNSSubdomain(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"myscanner.duckdns.org", "myscanner"},
		{"sub.domain.duckdns.org", "sub"},
		{"nodot", "nodot"},
	}
	for _, tt := range tests {
		got := extractDuckDNSSubdomain(tt.input)
		if got != tt.want {
			t.Errorf("extractDuckDNSSubdomain(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestEnsureCertificateHTTP(t *testing.T) {
	dir := t.TempDir()

	cfg := config.ACMEConfig{
		Enabled:   true,
		Email:     "test@example.com",
		Domains:   []string{"scanner.example.com"},
		Challenge: "http",
		CertDir:   dir,
	}

	mgr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// For HTTP-01, EnsureCertificate should be a no-op (returns nil).
	if err := mgr.EnsureCertificate(context.Background()); err != nil {
		t.Fatalf("EnsureCertificate() error: %v", err)
	}
}

// bigInt returns a big.Int for test certificate serial numbers.
func bigInt(n int64) *big.Int {
	return new(big.Int).SetInt64(n)
}
