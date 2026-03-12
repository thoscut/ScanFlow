package acme

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/thoscut/scanflow/server/internal/config"
)

const duckDNSAPIBase = "https://www.duckdns.org/update"

// duckDNSSolver implements DNSSolver using the DuckDNS API.
// DuckDNS supports setting a single TXT record per subdomain.
type duckDNSSolver struct {
	token  string
	client *http.Client
}

func newDuckDNSSolver(cfg config.ACMEDuckDNSConfig) (*duckDNSSolver, error) {
	if cfg.TokenFile == "" {
		return nil, fmt.Errorf("duckdns: token_file is required")
	}
	token, err := readTokenFile(cfg.TokenFile)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, fmt.Errorf("duckdns: token_file is empty")
	}
	return &duckDNSSolver{
		token:  token,
		client: &http.Client{},
	}, nil
}

func (s *duckDNSSolver) Present(ctx context.Context, domain, token, keyAuth string) error {
	subdomain := extractDuckDNSSubdomain(domain)

	params := url.Values{
		"domains": {subdomain},
		"token":   {s.token},
		"txt":     {keyAuth},
	}

	reqURL := duckDNSAPIBase + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("duckdns: update TXT record: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "OK" {
		return fmt.Errorf("duckdns: update TXT failed: %s", string(body))
	}

	slog.Info("duckdns TXT record updated",
		"subdomain", subdomain,
		"domain", domain)

	return nil
}

func (s *duckDNSSolver) CleanUp(ctx context.Context, domain, token, keyAuth string) error {
	subdomain := extractDuckDNSSubdomain(domain)

	params := url.Values{
		"domains": {subdomain},
		"token":   {s.token},
		"txt":     {""},
		"clear":   {"true"},
	}

	reqURL := duckDNSAPIBase + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("duckdns: clear TXT record: %w", err)
	}
	defer resp.Body.Close()

	slog.Info("duckdns TXT record cleared",
		"subdomain", subdomain,
		"domain", domain)

	return nil
}

// extractDuckDNSSubdomain extracts the DuckDNS subdomain from a full domain.
// e.g. "myscanner.duckdns.org" -> "myscanner"
func extractDuckDNSSubdomain(domain string) string {
	for i := 0; i < len(domain); i++ {
		if domain[i] == '.' {
			return domain[:i]
		}
	}
	return domain
}
