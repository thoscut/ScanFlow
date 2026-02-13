package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
	"github.com/thoscut/scanflow/server/internal/output"
	"github.com/thoscut/scanflow/server/internal/processor"
	"github.com/thoscut/scanflow/server/internal/scanner"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Server.Auth.Enabled = false // Disable auth for tests

	sc := scanner.New("", true, scanner.ScanOptions{})
	sc.Init()

	q := jobs.NewQueue()
	profiles, _ := config.NewProfileStore("")
	proc := processor.NewPipeline(cfg.Processing)
	outputs := output.NewManager(cfg.Output)

	return NewServer(cfg, sc, q, profiles, proc, outputs)
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["status"] != "ok" {
		t.Fatalf("expected status 'ok', got %s", resp["status"])
	}
}

func TestStatusEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["status"] != "ok" {
		t.Fatalf("expected status 'ok', got %v", resp["status"])
	}
}

func TestListDevicesEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/scanner/devices", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Devices []scanner.Device `json:"devices"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Devices) == 0 {
		t.Fatal("expected at least one device")
	}
}

func TestListProfilesEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/profiles", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Profiles []config.Profile `json:"profiles"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Profiles) < 3 {
		t.Fatalf("expected at least 3 profiles, got %d", len(resp.Profiles))
	}
}

func TestStartScanEndpoint(t *testing.T) {
	srv := newTestServer(t)

	scanReq := jobs.ScanRequest{
		Profile: "standard",
	}
	body, _ := json.Marshal(scanReq)

	req := httptest.NewRequest("POST", "/api/v1/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}

	var job jobs.Job
	json.NewDecoder(w.Body).Decode(&job)

	if job.ID == "" {
		t.Fatal("expected non-empty job ID")
	}
	if job.Profile != "standard" {
		t.Fatalf("expected profile 'standard', got %s", job.Profile)
	}
}

func TestStartScanInvalidProfile(t *testing.T) {
	srv := newTestServer(t)

	scanReq := jobs.ScanRequest{
		Profile: "nonexistent",
	}
	body, _ := json.Marshal(scanReq)

	req := httptest.NewRequest("POST", "/api/v1/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestGetJobStatusNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/scan/nonexistent-id", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestListOutputsEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/outputs", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Outputs []output.Target `json:"outputs"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	// Filesystem should always be available
	found := false
	for _, o := range resp.Outputs {
		if o.Name == "filesystem" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected filesystem output to be available")
	}
}

func TestAuthMiddleware(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.Auth.Enabled = true
	cfg.Server.Auth.APIKeys = []string{"test-key-123"}

	sc := scanner.New("", true, scanner.ScanOptions{})
	sc.Init()

	q := jobs.NewQueue()
	profiles, _ := config.NewProfileStore("")
	proc := processor.NewPipeline(cfg.Processing)
	outputs := output.NewManager(cfg.Output)

	srv := NewServer(cfg, sc, q, profiles, proc, outputs)

	// Without auth should fail
	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", w.Code)
	}

	// With Bearer token should succeed
	req = httptest.NewRequest("GET", "/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer test-key-123")
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with auth, got %d", w.Code)
	}

	// With X-API-Key header should succeed
	req = httptest.NewRequest("GET", "/api/v1/status", nil)
	req.Header.Set("X-API-Key", "test-key-123")
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with X-API-Key, got %d", w.Code)
	}

	// Health endpoint should work without auth
	req = httptest.NewRequest("GET", "/api/v1/health", nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for health without auth, got %d", w.Code)
	}
}

func TestGetSettingsEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/settings", nil)
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp settingsResponse
	json.NewDecoder(w.Body).Decode(&resp)

	// Default config has OCR enabled
	if !resp.OcrEnabled {
		t.Fatal("expected OCR to be enabled by default")
	}
	if resp.OcrLanguage != "deu+eng" {
		t.Fatalf("expected language 'deu+eng', got %s", resp.OcrLanguage)
	}
}

func TestUpdateSettingsEndpoint(t *testing.T) {
	srv := newTestServer(t)

	// Disable OCR via settings
	body, _ := json.Marshal(settingsResponse{
		OcrEnabled:  false,
		OcrLanguage: "eng",
	})

	req := httptest.NewRequest("PUT", "/api/v1/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp settingsResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.OcrEnabled {
		t.Fatal("expected OCR to be disabled after update")
	}
	if resp.OcrLanguage != "eng" {
		t.Fatalf("expected language 'eng', got %s", resp.OcrLanguage)
	}

	// Verify settings are persisted by reading them back
	req = httptest.NewRequest("GET", "/api/v1/settings", nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	json.NewDecoder(w.Body).Decode(&resp)
	if resp.OcrEnabled {
		t.Fatal("expected OCR to remain disabled after re-read")
	}
}

func TestStartScanWithOcrDisabled(t *testing.T) {
	srv := newTestServer(t)

	ocrOff := false
	scanReq := jobs.ScanRequest{
		Profile:    "standard",
		OcrEnabled: &ocrOff,
	}
	body, _ := json.Marshal(scanReq)

	req := httptest.NewRequest("POST", "/api/v1/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}

	var job jobs.Job
	json.NewDecoder(w.Body).Decode(&job)

	if job.OcrEnabled == nil {
		t.Fatal("expected ocr_enabled to be set")
	}
	if *job.OcrEnabled {
		t.Fatal("expected ocr_enabled to be false")
	}
}
