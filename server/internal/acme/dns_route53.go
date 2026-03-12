package acme

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/thoscut/scanflow/server/internal/config"
)

// route53Solver implements DNSSolver using the AWS Route 53 API.
type route53Solver struct {
	accessKeyID     string
	secretAccessKey string
	hostedZoneID    string
	region          string
	client          *http.Client
}

func newRoute53Solver(cfg config.ACMERoute53Config) (*route53Solver, error) {
	if cfg.HostedZoneID == "" {
		return nil, fmt.Errorf("route53: hosted_zone_id is required")
	}

	secretKey := ""
	if cfg.SecretAccessKeyFile != "" {
		var err error
		secretKey, err = readTokenFile(cfg.SecretAccessKeyFile)
		if err != nil {
			return nil, err
		}
	}

	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	return &route53Solver{
		accessKeyID:     cfg.AccessKeyID,
		secretAccessKey: secretKey,
		hostedZoneID:    cfg.HostedZoneID,
		region:          region,
		client:          &http.Client{},
	}, nil
}

func (s *route53Solver) Present(ctx context.Context, domain, token, keyAuth string) error {
	return s.changeRecord(ctx, "UPSERT", domain, keyAuth)
}

func (s *route53Solver) CleanUp(ctx context.Context, domain, token, keyAuth string) error {
	return s.changeRecord(ctx, "DELETE", domain, keyAuth)
}

func (s *route53Solver) changeRecord(ctx context.Context, action, domain, value string) error {
	recordName := challengeRecordName(domain)

	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ChangeResourceRecordSetsRequest xmlns="https://route53.amazonaws.com/doc/2013-04-01/">
  <ChangeBatch>
    <Changes>
      <Change>
        <Action>%s</Action>
        <ResourceRecordSet>
          <Name>%s</Name>
          <Type>TXT</Type>
          <TTL>60</TTL>
          <ResourceRecords>
            <ResourceRecord>
              <Value>"%s"</Value>
            </ResourceRecord>
          </ResourceRecords>
        </ResourceRecordSet>
      </Change>
    </Changes>
  </ChangeBatch>
</ChangeResourceRecordSetsRequest>`, action, recordName, value)

	url := fmt.Sprintf("https://route53.amazonaws.com/2013-04-01/hostedzone/%s/rrset",
		s.hostedZoneID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/xml")

	s.signRequest(req, []byte(body))

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("route53: %s record: %w", action, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("route53: %s record: HTTP %d: %s", action, resp.StatusCode, string(respBody))
	}

	slog.Info("route53 DNS record changed",
		"action", action,
		"record", recordName,
		"domain", domain)

	return nil
}

// signRequest adds AWS Signature Version 4 headers to the request.
func (s *route53Solver) signRequest(req *http.Request, body []byte) {
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Host", req.URL.Host)

	// Compute payload hash.
	payloadHash := sha256Hex(body)

	// Create canonical request.
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-amz-date:%s\n",
		req.Header.Get("Content-Type"), req.URL.Host, amzDate)
	signedHeaders := "content-type;host;x-amz-date"

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		req.URL.Path,
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash)

	// Create string to sign.
	credentialScope := fmt.Sprintf("%s/%s/route53/aws4_request", dateStamp, s.region)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate, credentialScope, sha256Hex([]byte(canonicalRequest)))

	// Calculate signature.
	signingKey := getSignatureKey(s.secretAccessKey, dateStamp, s.region, "route53")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	// Add authorization header.
	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKeyID, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func getSignatureKey(key, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+key), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}
