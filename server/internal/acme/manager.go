// Package acme provides automatic TLS certificate management via Let's Encrypt.
// It supports both HTTP-01 and DNS-01 challenges.
package acme

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"github.com/thoscut/scanflow/server/internal/config"
)

// LetsEncryptProductionURL is the default ACME directory.
const LetsEncryptProductionURL = "https://acme-v02.api.letsencrypt.org/directory"

// Manager handles automatic certificate provisioning and renewal.
type Manager struct {
	cfg        config.ACMEConfig
	httpMgr    *autocert.Manager  // used for HTTP-01 challenge
	dnsSolver  DNSSolver          // used for DNS-01 challenge
	acmeClient *acme.Client
	certDir    string
	mu         sync.RWMutex
	cert       *tls.Certificate
}

// New creates a new ACME manager from the provided configuration.
func New(cfg config.ACMEConfig) (*Manager, error) {
	certDir := cfg.CertDir
	if certDir == "" {
		certDir = "/var/lib/scanflow/certs"
	}

	if err := os.MkdirAll(certDir, 0o700); err != nil {
		return nil, fmt.Errorf("create cert directory: %w", err)
	}

	m := &Manager{
		cfg:     cfg,
		certDir: certDir,
	}

	challenge := cfg.Challenge
	if challenge == "" {
		challenge = "http"
	}

	switch challenge {
	case "http":
		if err := m.setupHTTP(); err != nil {
			return nil, err
		}
	case "dns":
		if err := m.setupDNS(); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported ACME challenge type: %q (use \"http\" or \"dns\")", challenge)
	}

	return m, nil
}

// setupHTTP configures the autocert manager for HTTP-01 challenges.
func (m *Manager) setupHTTP() error {
	m.httpMgr = &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Email:      m.cfg.Email,
		HostPolicy: autocert.HostWhitelist(m.cfg.Domains...),
		Cache:      autocert.DirCache(m.certDir),
	}

	if m.cfg.DirectoryURL != "" {
		m.httpMgr.Client = &acme.Client{DirectoryURL: m.cfg.DirectoryURL}
	}

	slog.Info("ACME HTTP-01 challenge configured",
		"domains", m.cfg.Domains,
		"email", m.cfg.Email)

	return nil
}

// setupDNS configures the DNS-01 challenge solver.
func (m *Manager) setupDNS() error {
	solver, err := NewDNSSolver(m.cfg)
	if err != nil {
		return fmt.Errorf("create DNS solver: %w", err)
	}
	m.dnsSolver = solver

	dirURL := m.cfg.DirectoryURL
	if dirURL == "" {
		dirURL = LetsEncryptProductionURL
	}

	m.acmeClient = &acme.Client{DirectoryURL: dirURL}

	slog.Info("ACME DNS-01 challenge configured",
		"domains", m.cfg.Domains,
		"email", m.cfg.Email,
		"dns_provider", m.cfg.DNSProvider)

	return nil
}

// TLSConfig returns a tls.Config that uses ACME-managed certificates.
// For HTTP-01, the autocert manager handles certificate retrieval on-demand.
// For DNS-01, a pre-fetched certificate is served.
func (m *Manager) TLSConfig() *tls.Config {
	if m.httpMgr != nil {
		return m.httpMgr.TLSConfig()
	}

	return &tls.Config{
		GetCertificate: m.getDNSCertificate,
		MinVersion:     tls.VersionTLS12,
	}
}

// HTTPHandler returns an http.Handler that serves ACME HTTP-01 challenge
// responses and redirects other requests to HTTPS. Returns nil when using
// DNS-01 challenges.
func (m *Manager) HTTPHandler(fallback http.Handler) http.Handler {
	if m.httpMgr != nil {
		return m.httpMgr.HTTPHandler(fallback)
	}
	return nil
}

// EnsureCertificate obtains a certificate via DNS-01 challenge if one is not
// already cached. This should be called at startup when using DNS challenges.
func (m *Manager) EnsureCertificate(ctx context.Context) error {
	if m.httpMgr != nil {
		// HTTP-01 handles certificates on-demand.
		return nil
	}

	// Check for cached certificate on disk.
	certFile := filepath.Join(m.certDir, "cert.pem")
	keyFile := filepath.Join(m.certDir, "key.pem")

	if cert, err := tls.LoadX509KeyPair(certFile, keyFile); err == nil {
		// Verify the certificate is still valid (with 30-day buffer).
		if leaf, parseErr := x509.ParseCertificate(cert.Certificate[0]); parseErr == nil {
			if time.Until(leaf.NotAfter) > 30*24*time.Hour {
				m.mu.Lock()
				m.cert = &cert
				m.mu.Unlock()
				slog.Info("loaded cached ACME certificate",
					"expires", leaf.NotAfter,
					"domains", leaf.DNSNames)
				return nil
			}
			slog.Info("cached certificate expires soon, renewing",
				"expires", leaf.NotAfter)
		}
	}

	// Obtain new certificate via DNS-01.
	return m.obtainDNSCertificate(ctx)
}

// StartRenewal starts a background goroutine that renews DNS-01 certificates
// before they expire.
func (m *Manager) StartRenewal(ctx context.Context) {
	if m.httpMgr != nil {
		// autocert handles renewal automatically.
		return
	}

	go func() {
		ticker := time.NewTicker(12 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.mu.RLock()
				cert := m.cert
				m.mu.RUnlock()

				if cert == nil {
					continue
				}

				leaf, err := x509.ParseCertificate(cert.Certificate[0])
				if err != nil {
					continue
				}

				// Renew when less than 30 days remain.
				if time.Until(leaf.NotAfter) < 30*24*time.Hour {
					slog.Info("renewing ACME certificate",
						"expires", leaf.NotAfter)
					if err := m.obtainDNSCertificate(ctx); err != nil {
						slog.Error("certificate renewal failed", "error", err)
					}
				}
			}
		}
	}()
}

// getDNSCertificate returns the current DNS-01 certificate for TLS handshakes.
func (m *Manager) getDNSCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	m.mu.RLock()
	cert := m.cert
	m.mu.RUnlock()

	if cert == nil {
		return nil, errors.New("no ACME certificate available")
	}
	return cert, nil
}

// obtainDNSCertificate performs the full ACME DNS-01 flow to obtain a certificate.
func (m *Manager) obtainDNSCertificate(ctx context.Context) error {
	slog.Info("obtaining certificate via DNS-01", "domains", m.cfg.Domains)

	// Generate account key if needed.
	accountKeyFile := filepath.Join(m.certDir, "account.key")
	accountKey, err := loadOrCreateKey(accountKeyFile)
	if err != nil {
		return fmt.Errorf("account key: %w", err)
	}
	m.acmeClient.Key = accountKey

	// Register account (no-op if already registered).
	acct := &acme.Account{Contact: []string{"mailto:" + m.cfg.Email}}
	if _, err := m.acmeClient.Register(ctx, acct, acme.AcceptTOS); err != nil {
		// Handle "already registered" (409 Conflict) by retrieving the existing account.
		acmeErr, ok := err.(*acme.Error)
		if ok && acmeErr.StatusCode == 409 {
			// Already registered, this is fine.
		} else if _, regErr := m.acmeClient.GetReg(ctx, ""); regErr != nil {
			return fmt.Errorf("ACME register: %w", err)
		}
	}

	// Create order for the configured domains.
	order, err := m.acmeClient.AuthorizeOrder(ctx, acme.DomainIDs(m.cfg.Domains...))
	if err != nil {
		return fmt.Errorf("ACME authorize order: %w", err)
	}

	// Solve DNS-01 challenges.
	for _, authzURL := range order.AuthzURLs {
		authz, err := m.acmeClient.GetAuthorization(ctx, authzURL)
		if err != nil {
			return fmt.Errorf("get authorization: %w", err)
		}

		if authz.Status == acme.StatusValid {
			continue
		}

		// Find DNS-01 challenge.
		var challenge *acme.Challenge
		for _, c := range authz.Challenges {
			if c.Type == "dns-01" {
				challenge = c
				break
			}
		}
		if challenge == nil {
			return fmt.Errorf("no dns-01 challenge for %s", authz.Identifier.Value)
		}

		// Compute the DNS record value.
		keyAuth, err := m.acmeClient.DNS01ChallengeRecord(challenge.Token)
		if err != nil {
			return fmt.Errorf("compute DNS challenge record: %w", err)
		}

		domain := authz.Identifier.Value

		// Present DNS record.
		if err := m.dnsSolver.Present(ctx, domain, challenge.Token, keyAuth); err != nil {
			return fmt.Errorf("present DNS challenge for %s: %w", domain, err)
		}

		// Wait for DNS propagation.
		wait := m.cfg.DNSPropagationWait.Duration()
		if wait == 0 {
			wait = 120 * time.Second
		}
		slog.Info("waiting for DNS propagation", "domain", domain, "wait", wait)
		select {
		case <-ctx.Done():
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cleanupCancel()
			_ = m.dnsSolver.CleanUp(cleanupCtx, domain, challenge.Token, keyAuth)
			return ctx.Err()
		case <-time.After(wait):
		}

		// Accept challenge.
		if _, err := m.acmeClient.Accept(ctx, challenge); err != nil {
			_ = m.dnsSolver.CleanUp(ctx, domain, challenge.Token, keyAuth)
			return fmt.Errorf("accept challenge for %s: %w", domain, err)
		}

		// Wait for authorization to become valid.
		if _, err := m.acmeClient.WaitAuthorization(ctx, authz.URI); err != nil {
			_ = m.dnsSolver.CleanUp(ctx, domain, challenge.Token, keyAuth)
			return fmt.Errorf("wait authorization for %s: %w", domain, err)
		}

		// Clean up DNS record.
		if err := m.dnsSolver.CleanUp(ctx, domain, challenge.Token, keyAuth); err != nil {
			slog.Warn("failed to clean up DNS record", "domain", domain, "error", err)
		}
	}

	// Generate certificate key pair.
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate cert key: %w", err)
	}

	// Create CSR.
	template := &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: m.cfg.Domains[0]},
		DNSNames: m.cfg.Domains,
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, certKey)
	if err != nil {
		return fmt.Errorf("create CSR: %w", err)
	}

	// Finalize order and get certificate.
	certs, _, err := m.acmeClient.CreateOrderCert(ctx, order.FinalizeURL, csrDER, true)
	if err != nil {
		return fmt.Errorf("finalize order: %w", err)
	}

	// Save certificate chain to disk.
	certFile := filepath.Join(m.certDir, "cert.pem")
	keyFile := filepath.Join(m.certDir, "key.pem")

	if err := saveCertificates(certFile, certs); err != nil {
		return fmt.Errorf("save certificate: %w", err)
	}
	if err := saveKey(keyFile, certKey); err != nil {
		return fmt.Errorf("save key: %w", err)
	}

	// Load and cache the certificate for TLS.
	tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("load certificate: %w", err)
	}

	m.mu.Lock()
	m.cert = &tlsCert
	m.mu.Unlock()

	leaf, _ := x509.ParseCertificate(tlsCert.Certificate[0])
	slog.Info("ACME certificate obtained",
		"domains", leaf.DNSNames,
		"expires", leaf.NotAfter)

	return nil
}

// loadOrCreateKey loads an ECDSA key from disk or creates a new one.
func loadOrCreateKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		block, _ := pem.Decode(data)
		if block != nil && block.Type == "EC PRIVATE KEY" {
			return x509.ParseECPrivateKey(block.Bytes)
		}
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	if err := saveKey(path, key); err != nil {
		return nil, err
	}

	return key, nil
}

// saveCertificates writes a PEM-encoded certificate chain to disk.
func saveCertificates(path string, certs [][]byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, cert := range certs {
		if err := pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: cert}); err != nil {
			return err
		}
	}
	return nil
}

// saveKey writes a PEM-encoded ECDSA private key to disk.
func saveKey(path string, key *ecdsa.PrivateKey) error {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	return pem.Encode(f, &pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
}
