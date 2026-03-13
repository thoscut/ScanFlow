package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsEndpointReturns200(t *testing.T) {
	m := NewMetrics()
	handler := m.Handler()

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("expected text/plain content type, got %q", ct)
	}
}

func TestMetricsJobCounters(t *testing.T) {
	m := NewMetrics()

	m.JobStarted()
	m.JobCompleted()
	m.JobStarted()
	m.JobFailed()
	m.JobStarted()
	m.JobCancelled()
	m.JobStarted()
	m.JobCompleted()

	handler := m.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()

	expect := []string{
		`scanflow_jobs_total{status="completed"} 2`,
		`scanflow_jobs_total{status="failed"} 1`,
		`scanflow_jobs_total{status="cancelled"} 1`,
		`scanflow_jobs_active 0`,
	}
	for _, e := range expect {
		if !strings.Contains(body, e) {
			t.Errorf("expected %q in output, body:\n%s", e, body)
		}
	}
}

func TestMetricsPageCounter(t *testing.T) {
	m := NewMetrics()
	m.PageScanned()
	m.PageScanned()
	m.PageScanned()

	handler := m.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "scanflow_scan_pages_total 3") {
		t.Errorf("expected page count 3, body:\n%s", body)
	}
}

func TestMetricsHTTPRecording(t *testing.T) {
	m := NewMetrics()

	m.RecordHTTPRequest(200, 10*time.Millisecond)
	m.RecordHTTPRequest(201, 20*time.Millisecond)
	m.RecordHTTPRequest(404, 5*time.Millisecond)
	m.RecordHTTPRequest(500, 50*time.Millisecond)

	handler := m.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()

	expect := []string{
		`scanflow_http_requests_total{code="2xx"} 2`,
		`scanflow_http_requests_total{code="4xx"} 1`,
		`scanflow_http_requests_total{code="5xx"} 1`,
		`scanflow_http_request_duration_seconds_count 4`,
	}
	for _, e := range expect {
		if !strings.Contains(body, e) {
			t.Errorf("expected %q in output, body:\n%s", e, body)
		}
	}
}

func TestMetricsMiddlewareRecords(t *testing.T) {
	m := NewMetrics()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	handler := MetricsMiddleware(m)(inner)

	req := httptest.NewRequest("GET", "/foo", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// The middleware should have recorded one 4xx request.
	metricsReq := httptest.NewRequest("GET", "/metrics", nil)
	metricsW := httptest.NewRecorder()
	m.Handler().ServeHTTP(metricsW, metricsReq)

	body := metricsW.Body.String()
	if !strings.Contains(body, `scanflow_http_requests_total{code="4xx"} 1`) {
		t.Errorf("expected 4xx count 1, body:\n%s", body)
	}
}

func TestMetricsActiveGauge(t *testing.T) {
	m := NewMetrics()

	m.JobStarted()
	m.JobStarted()
	m.JobStarted()

	handler := m.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "scanflow_jobs_active 3") {
		t.Errorf("expected active 3, body:\n%s", body)
	}

	// One finishes via completion.
	m.JobCompleted()

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body = w.Body.String()
	if !strings.Contains(body, "scanflow_jobs_active 2") {
		t.Errorf("expected active 2 after completion, body:\n%s", body)
	}
}

func TestMetricsPrometheusFormat(t *testing.T) {
	m := NewMetrics()
	handler := m.Handler()

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	body := w.Body.String()

	// Verify HELP and TYPE annotations exist.
	requiredAnnotations := []string{
		"# HELP scanflow_jobs_total",
		"# TYPE scanflow_jobs_total counter",
		"# HELP scanflow_jobs_active",
		"# TYPE scanflow_jobs_active gauge",
		"# HELP scanflow_jobs_pending",
		"# TYPE scanflow_jobs_pending gauge",
		"# HELP scanflow_scan_pages_total",
		"# TYPE scanflow_scan_pages_total counter",
		"# HELP scanflow_http_requests_total",
		"# TYPE scanflow_http_requests_total counter",
		"# HELP scanflow_http_request_duration_seconds",
		"# TYPE scanflow_http_request_duration_seconds summary",
	}
	for _, a := range requiredAnnotations {
		if !strings.Contains(body, a) {
			t.Errorf("missing annotation %q in output:\n%s", a, body)
		}
	}
}

func TestMetricsEndpointViaRouter(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("expected text/plain content type, got %q", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "scanflow_jobs_total") {
		t.Fatalf("expected metrics output, got:\n%s", body)
	}
}
