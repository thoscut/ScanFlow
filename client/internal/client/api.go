package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client communicates with the ScanFlow server API.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New creates a new API client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ScanRequest is the request body for starting a scan.
type ScanRequest struct {
	Profile  string            `json:"profile,omitempty"`
	DeviceID string            `json:"device_id,omitempty"`
	Options  *ScanOptions      `json:"options,omitempty"`
	Output   *OutputConfig     `json:"output,omitempty"`
	Metadata *DocumentMetadata `json:"metadata,omitempty"`
}

type ScanOptions struct {
	Resolution int     `json:"resolution,omitempty"`
	Mode       string  `json:"mode,omitempty"`
	Source     string  `json:"source,omitempty"`
	PageWidth  float64 `json:"page_width,omitempty"`
	PageHeight float64 `json:"page_height,omitempty"`
}

type OutputConfig struct {
	Target   string `json:"target"`
	Filename string `json:"filename,omitempty"`
}

type DocumentMetadata struct {
	Title         string `json:"title,omitempty"`
	Created       string `json:"created,omitempty"`
	Correspondent int    `json:"correspondent,omitempty"`
	DocumentType  int    `json:"document_type,omitempty"`
	Tags          []int  `json:"tags,omitempty"`
}

// ScanJob represents a scan job returned by the API.
type ScanJob struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Profile   string    `json:"profile"`
	Pages     []Page    `json:"pages"`
	Progress  int       `json:"progress"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Page struct {
	Number int `json:"number"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Device represents a scanner device.
type Device struct {
	Name   string `json:"name"`
	Vendor string `json:"vendor"`
	Model  string `json:"model"`
	Type   string `json:"type"`
}

// Profile represents a scan profile.
type Profile struct {
	Profile struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"profile"`
	Scanner struct {
		Resolution int     `json:"resolution"`
		Mode       string  `json:"mode"`
		Source     string  `json:"source"`
		PageHeight float64 `json:"page_height"`
	} `json:"scanner"`
}

// ServerStatus contains the server status response.
type ServerStatus struct {
	Status     string `json:"status"`
	Version    string `json:"version"`
	Scanner    bool   `json:"scanner"`
	Devices    int    `json:"devices"`
	ActiveJobs int    `json:"active_jobs"`
	TotalJobs  int    `json:"total_jobs"`
}

// ProgressUpdate is received via WebSocket.
type ProgressUpdate struct {
	Type       string `json:"type"`
	JobID      string `json:"job_id"`
	Status     string `json:"status,omitempty"`
	Page       int    `json:"page,omitempty"`
	Progress   int    `json:"progress,omitempty"`
	Message    string `json:"message,omitempty"`
	PreviewURL string `json:"preview_url,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Health checks the server health endpoint.
func (c *Client) Health(ctx context.Context) error {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/health", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Status returns the server status.
func (c *Client) Status(ctx context.Context) (*ServerStatus, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/status", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var status ServerStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &status, nil
}

// ListDevices returns available scanner devices.
func (c *Client) ListDevices(ctx context.Context) ([]Device, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/scanner/devices", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Devices []Device `json:"devices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Devices, nil
}

// ListProfiles returns available scan profiles.
func (c *Client) ListProfiles(ctx context.Context) ([]Profile, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/profiles", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Profiles []Profile `json:"profiles"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Profiles, nil
}

// GetProfile returns a specific scan profile.
func (c *Client) GetProfile(ctx context.Context, name string) (*Profile, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/profiles/"+name, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var profile Profile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &profile, nil
}

// StartScan initiates a new scan job.
func (c *Client) StartScan(ctx context.Context, req *ScanRequest) (*ScanJob, error) {
	resp, err := c.doRequest(ctx, "POST", "/api/v1/scan", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var job ScanJob
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &job, nil
}

// GetJobStatus returns the current status of a scan job.
func (c *Client) GetJobStatus(ctx context.Context, jobID string) (*ScanJob, error) {
	resp, err := c.doRequest(ctx, "GET", "/api/v1/scan/"+jobID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var job ScanJob
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &job, nil
}

// CancelJob cancels an active scan job.
func (c *Client) CancelJob(ctx context.Context, jobID string) error {
	resp, err := c.doRequest(ctx, "DELETE", "/api/v1/scan/"+jobID, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// ContinueScan tells the server to scan more pages for an existing job.
func (c *Client) ContinueScan(ctx context.Context, jobID string) error {
	resp, err := c.doRequest(ctx, "POST", "/api/v1/scan/"+jobID+"/continue", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// FinishScan tells the server to finalize the scan job and produce output.
func (c *Client) FinishScan(ctx context.Context, jobID string, output *OutputConfig, metadata *DocumentMetadata) error {
	body := map[string]interface{}{}
	if output != nil {
		body["output"] = output
	}
	if metadata != nil {
		body["metadata"] = metadata
	}

	resp, err := c.doRequest(ctx, "POST", "/api/v1/scan/"+jobID+"/finish", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// DeletePage removes a page from an active scan job.
func (c *Client) DeletePage(ctx context.Context, jobID string, pageNum int) error {
	resp, err := c.doRequest(ctx, "DELETE", fmt.Sprintf("/api/v1/scan/%s/pages/%d", jobID, pageNum), nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != "" {
			return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("server error: %d", resp.StatusCode)
	}

	return resp, nil
}
