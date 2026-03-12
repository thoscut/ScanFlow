package acme

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/thoscut/scanflow/server/internal/config"
)

const cloudflareAPIBase = "https://api.cloudflare.com/client/v4"

// cloudflareSolver implements DNSSolver using the Cloudflare API.
type cloudflareSolver struct {
	apiToken string
	zoneID   string
	client   *http.Client

	// Track record IDs for cleanup.
	records map[string]string // domain -> record ID
}

func newCloudflareSolver(cfg config.ACMECloudflareConfig) (*cloudflareSolver, error) {
	if cfg.APITokenFile == "" {
		return nil, fmt.Errorf("cloudflare: api_token_file is required")
	}
	token, err := readTokenFile(cfg.APITokenFile)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, fmt.Errorf("cloudflare: api_token_file is empty")
	}
	return &cloudflareSolver{
		apiToken: token,
		zoneID:   cfg.ZoneID,
		client:   &http.Client{},
		records:  make(map[string]string),
	}, nil
}

func (s *cloudflareSolver) Present(ctx context.Context, domain, token, keyAuth string) error {
	zoneID := s.zoneID
	if zoneID == "" {
		var err error
		zoneID, err = s.findZoneID(ctx, domain)
		if err != nil {
			return fmt.Errorf("cloudflare: find zone: %w", err)
		}
	}

	recordName := challengeRecordName(domain)

	body, _ := json.Marshal(map[string]any{
		"type":    "TXT",
		"name":    recordName,
		"content": keyAuth,
		"ttl":     120,
	})

	url := fmt.Sprintf("%s/zones/%s/dns_records", cloudflareAPIBase, zoneID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare: create record: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cloudflare: create record: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Result struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	s.records[domain] = result.Result.ID

	slog.Info("cloudflare DNS record created",
		"domain", domain,
		"record", recordName,
		"record_id", result.Result.ID)

	return nil
}

func (s *cloudflareSolver) CleanUp(ctx context.Context, domain, token, keyAuth string) error {
	recordID, ok := s.records[domain]
	if !ok {
		return nil
	}
	delete(s.records, domain)

	zoneID := s.zoneID
	if zoneID == "" {
		var err error
		zoneID, err = s.findZoneID(ctx, domain)
		if err != nil {
			return fmt.Errorf("cloudflare: find zone: %w", err)
		}
	}

	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", cloudflareAPIBase, zoneID, recordID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare: delete record: %w", err)
	}
	defer resp.Body.Close()

	slog.Info("cloudflare DNS record deleted", "domain", domain, "record_id", recordID)
	return nil
}

// findZoneID locates the Cloudflare zone ID for the given domain.
func (s *cloudflareSolver) findZoneID(ctx context.Context, domain string) (string, error) {
	// Walk up the domain to find the zone.
	parts := splitDomain(domain)
	for i := range parts {
		candidate := joinDomain(parts[i:])
		url := fmt.Sprintf("%s/zones?name=%s", cloudflareAPIBase, candidate)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+s.apiToken)

		resp, err := s.client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		var result struct {
			Result []struct {
				ID string `json:"id"`
			} `json:"result"`
		}
		json.NewDecoder(resp.Body).Decode(&result)

		if len(result.Result) > 0 {
			return result.Result[0].ID, nil
		}
	}

	return "", fmt.Errorf("no Cloudflare zone found for %s", domain)
}

func splitDomain(domain string) []string {
	var parts []string
	for domain != "" {
		parts = append(parts, domain)
		idx := indexOf(domain, '.')
		if idx < 0 {
			break
		}
		domain = domain[idx+1:]
	}
	return parts
}

func joinDomain(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
