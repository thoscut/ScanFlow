package acme

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/thoscut/scanflow/server/internal/config"
)

// DNSSolver manages DNS TXT records for ACME DNS-01 challenges.
type DNSSolver interface {
	// Present creates a TXT record at _acme-challenge.<domain> with the given value.
	Present(ctx context.Context, domain, token, keyAuth string) error
	// CleanUp removes the TXT record created by Present.
	CleanUp(ctx context.Context, domain, token, keyAuth string) error
}

// NewDNSSolver creates a DNS solver for the configured provider.
func NewDNSSolver(cfg config.ACMEConfig) (DNSSolver, error) {
	switch cfg.DNSProvider {
	case "cloudflare":
		return newCloudflareSolver(cfg.Cloudflare)
	case "duckdns":
		return newDuckDNSSolver(cfg.DuckDNS)
	case "route53":
		return newRoute53Solver(cfg.Route53)
	case "exec":
		return newExecSolver(cfg.Exec)
	case "":
		return nil, fmt.Errorf("dns_provider is required when challenge = \"dns\"")
	default:
		return nil, fmt.Errorf("unsupported DNS provider: %q (supported: cloudflare, duckdns, route53, exec)", cfg.DNSProvider)
	}
}

// readTokenFile reads a secret token from a file, trimming whitespace.
func readTokenFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read token file %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// challengeRecordName returns the DNS record name for an ACME DNS-01 challenge.
func challengeRecordName(domain string) string {
	return "_acme-challenge." + domain
}
