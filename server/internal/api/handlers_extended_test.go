package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thoscut/scanflow/server/internal/config"
	"github.com/thoscut/scanflow/server/internal/jobs"
)

func TestGetJobStatusFound(t *testing.T) {
	srv := newTestServer(t)

	// Create a job first
	scanReq := jobs.ScanRequest{Profile: "standard"}
	body, _ := json.Marshal(scanReq)
	req := httptest.NewRequest("POST", "/api/v1/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	var created jobs.Job
	json.NewDecoder(w.Body).Decode(&created)

	// Now get the job status
	req = httptest.NewRequest("GET", "/api/v1/scan/"+created.ID, nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var got jobs.Job
	json.NewDecoder(w.Body).Decode(&got)
	if got.ID != created.ID {
		t.Fatalf("expected job %s, got %s", created.ID, got.ID)
	}
}

func TestCancelJobEndpoint(t *testing.T) {
	srv := newTestServer(t)

	// Create a job
	scanReq := jobs.ScanRequest{Profile: "standard"}
	body, _ := json.Marshal(scanReq)
	req := httptest.NewRequest("POST", "/api/v1/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	var created jobs.Job
	json.NewDecoder(w.Body).Decode(&created)

	// Cancel the job
	req = httptest.NewRequest("DELETE", "/api/v1/scan/"+created.ID, nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCancelJobNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("DELETE", "/api/v1/scan/nonexistent-id", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListPagesEndpoint(t *testing.T) {
	srv := newTestServer(t)

	scanReq := jobs.ScanRequest{Profile: "standard"}
	body, _ := json.Marshal(scanReq)
	req := httptest.NewRequest("POST", "/api/v1/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	var created jobs.Job
	json.NewDecoder(w.Body).Decode(&created)

	// List pages
	req = httptest.NewRequest("GET", "/api/v1/scan/"+created.ID+"/pages", nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Pages []jobs.Page `json:"pages"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	// New job should have no pages yet
	if len(resp.Pages) != 0 {
		t.Fatalf("expected 0 pages, got %d", len(resp.Pages))
	}
}

func TestListPagesNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/scan/nonexistent/pages", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeletePageNotFoundJob(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("DELETE", "/api/v1/scan/nonexistent/pages/1", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeletePageInvalidNumber(t *testing.T) {
	srv := newTestServer(t)

	scanReq := jobs.ScanRequest{Profile: "standard"}
	body, _ := json.Marshal(scanReq)
	req := httptest.NewRequest("POST", "/api/v1/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	var created jobs.Job
	json.NewDecoder(w.Body).Decode(&created)

	req = httptest.NewRequest("DELETE", "/api/v1/scan/"+created.ID+"/pages/abc", nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetPreviewEndpoint(t *testing.T) {
	srv := newTestServer(t)

	scanReq := jobs.ScanRequest{Profile: "standard"}
	body, _ := json.Marshal(scanReq)
	req := httptest.NewRequest("POST", "/api/v1/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	var created jobs.Job
	json.NewDecoder(w.Body).Decode(&created)

	req = httptest.NewRequest("GET", "/api/v1/scan/"+created.ID+"/preview", nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetPreviewNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/scan/nonexistent/preview", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestContinueScanNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("POST", "/api/v1/scan/nonexistent/continue", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestFinishScanNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("POST", "/api/v1/scan/nonexistent/finish", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetProfileEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/profiles/standard", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var profile config.Profile
	json.NewDecoder(w.Body).Decode(&profile)
	if profile.Scanner.Resolution != 300 {
		t.Fatalf("expected resolution 300, got %d", profile.Scanner.Resolution)
	}
}

func TestGetProfileNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/profiles/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestCreateProfileEndpoint(t *testing.T) {
	srv := newTestServer(t)

	profile := config.Profile{
		Profile: config.ProfileInfo{Name: "custom", Description: "Custom profile"},
		Scanner: config.ProfileScanner{Resolution: 150, Mode: "lineart"},
	}
	body, _ := json.Marshal(profile)

	req := httptest.NewRequest("POST", "/api/v1/profiles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the profile can be retrieved
	req = httptest.NewRequest("GET", "/api/v1/profiles/custom", nil)
	w = httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCreateProfileMissingName(t *testing.T) {
	srv := newTestServer(t)

	profile := config.Profile{
		Scanner: config.ProfileScanner{Resolution: 150},
	}
	body, _ := json.Marshal(profile)

	req := httptest.NewRequest("POST", "/api/v1/profiles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUpdateProfileEndpoint(t *testing.T) {
	srv := newTestServer(t)

	profile := config.Profile{
		Profile: config.ProfileInfo{Name: "Standard Updated"},
		Scanner: config.ProfileScanner{Resolution: 600},
	}
	body, _ := json.Marshal(profile)

	req := httptest.NewRequest("PUT", "/api/v1/profiles/standard", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateProfileNotFound(t *testing.T) {
	srv := newTestServer(t)

	profile := config.Profile{
		Scanner: config.ProfileScanner{Resolution: 600},
	}
	body, _ := json.Marshal(profile)

	req := httptest.NewRequest("PUT", "/api/v1/profiles/nonexistent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetDeviceEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/scanner/devices/test:0", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetDeviceNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/v1/scanner/devices/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestOpenDeviceEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("POST", "/api/v1/scanner/devices/test:0/open", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCloseDeviceEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("DELETE", "/api/v1/scanner/devices/test:0/close", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStartScanWithOutputOverride(t *testing.T) {
	srv := newTestServer(t)

	scanReq := jobs.ScanRequest{
		Profile: "standard",
		Output:  &jobs.OutputConfig{Target: "filesystem", Filename: "custom.pdf"},
	}
	body, _ := json.Marshal(scanReq)

	req := httptest.NewRequest("POST", "/api/v1/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var job jobs.Job
	json.NewDecoder(w.Body).Decode(&job)
	if job.Output.Target != "filesystem" {
		t.Fatalf("expected output target 'filesystem', got %s", job.Output.Target)
	}
}

func TestStartScanWithMetadata(t *testing.T) {
	srv := newTestServer(t)

	scanReq := jobs.ScanRequest{
		Profile: "standard",
		Metadata: &jobs.DocumentMetadata{
			Title:         "Test Invoice",
			Correspondent: 5,
			Tags:          []int{1, 2, 3},
		},
	}
	body, _ := json.Marshal(scanReq)

	req := httptest.NewRequest("POST", "/api/v1/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var job jobs.Job
	json.NewDecoder(w.Body).Decode(&job)
	if job.Metadata == nil {
		t.Fatal("expected metadata to be set")
	}
	if job.Metadata.Title != "Test Invoice" {
		t.Fatalf("expected title 'Test Invoice', got %s", job.Metadata.Title)
	}
}

func TestStartScanInvalidBody(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("POST", "/api/v1/scan", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSendOutputNotFound(t *testing.T) {
	srv := newTestServer(t)

	body, _ := json.Marshal(map[string]string{"target": "paperless"})
	req := httptest.NewRequest("POST", "/api/v1/scan/nonexistent/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestReorderPagesNotFound(t *testing.T) {
	srv := newTestServer(t)

	body, _ := json.Marshal(map[string][]int{"order": {2, 1, 3}})
	req := httptest.NewRequest("POST", "/api/v1/scan/nonexistent/pages/reorder", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestStartScanDefaultProfile(t *testing.T) {
	srv := newTestServer(t)

	// Empty profile should default to "standard"
	body, _ := json.Marshal(jobs.ScanRequest{})

	req := httptest.NewRequest("POST", "/api/v1/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var job jobs.Job
	json.NewDecoder(w.Body).Decode(&job)
	if job.Profile != "standard" {
		t.Fatalf("expected profile 'standard', got %s", job.Profile)
	}
}
