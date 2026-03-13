package api

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics collects Prometheus-compatible application metrics using only the
// standard library. All counters use sync/atomic for lock-free updates.
type Metrics struct {
	// Job counters by status.
	jobsCompleted atomic.Int64
	jobsFailed    atomic.Int64
	jobsCancelled atomic.Int64

	// Active and pending gauges.
	jobsActive  atomic.Int64
	jobsPending atomic.Int64

	// Page counter.
	scanPagesTotal atomic.Int64

	// HTTP request counter by status code bucket.
	httpRequests2xx atomic.Int64
	httpRequests3xx atomic.Int64
	httpRequests4xx atomic.Int64
	httpRequests5xx atomic.Int64

	// HTTP request duration (count + sum).
	httpDurationMu    sync.Mutex
	httpDurationCount int64
	httpDurationSum   float64
}

// NewMetrics creates a zero-valued Metrics collector.
func NewMetrics() *Metrics {
	return &Metrics{}
}

// ---------------------------------------------------------------------------
// Job helpers
// ---------------------------------------------------------------------------

// JobCompleted increments the completed-jobs counter and decrements the active
// gauge.
func (m *Metrics) JobCompleted() {
	m.jobsCompleted.Add(1)
	m.jobsActive.Add(-1)
}

// JobFailed increments the failed-jobs counter and decrements the active gauge.
func (m *Metrics) JobFailed() {
	m.jobsFailed.Add(1)
	m.jobsActive.Add(-1)
}

// JobCancelled increments the cancelled-jobs counter and decrements the active
// gauge.
func (m *Metrics) JobCancelled() {
	m.jobsCancelled.Add(1)
	m.jobsActive.Add(-1)
}

// JobStarted increments the active-jobs gauge.
func (m *Metrics) JobStarted() {
	m.jobsActive.Add(1)
}

// JobFinished decrements the active-jobs gauge without recording a terminal
// status. Use JobCompleted / JobFailed / JobCancelled when the outcome is
// known.
func (m *Metrics) JobFinished() {
	m.jobsActive.Add(-1)
}

// SetJobsPending sets the pending-jobs gauge to an absolute value.
func (m *Metrics) SetJobsPending(n int64) {
	m.jobsPending.Store(n)
}

// PageScanned increments the total-pages counter.
func (m *Metrics) PageScanned() {
	m.scanPagesTotal.Add(1)
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

// RecordHTTPRequest records an HTTP request by status code and duration.
func (m *Metrics) RecordHTTPRequest(statusCode int, duration time.Duration) {
	switch {
	case statusCode >= 500:
		m.httpRequests5xx.Add(1)
	case statusCode >= 400:
		m.httpRequests4xx.Add(1)
	case statusCode >= 300:
		m.httpRequests3xx.Add(1)
	default:
		m.httpRequests2xx.Add(1)
	}

	secs := duration.Seconds()
	m.httpDurationMu.Lock()
	m.httpDurationCount++
	m.httpDurationSum += secs
	m.httpDurationMu.Unlock()
}

// ---------------------------------------------------------------------------
// Prometheus text exposition
// ---------------------------------------------------------------------------

// Handler returns an http.HandlerFunc that writes metrics in Prometheus text
// exposition format.
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		// Job totals
		fmt.Fprint(w, "# HELP scanflow_jobs_total Total number of scan jobs by status.\n")
		fmt.Fprint(w, "# TYPE scanflow_jobs_total counter\n")
		fmt.Fprintf(w, "scanflow_jobs_total{status=\"completed\"} %d\n", m.jobsCompleted.Load())
		fmt.Fprintf(w, "scanflow_jobs_total{status=\"failed\"} %d\n", m.jobsFailed.Load())
		fmt.Fprintf(w, "scanflow_jobs_total{status=\"cancelled\"} %d\n", m.jobsCancelled.Load())

		// Active gauge
		fmt.Fprint(w, "# HELP scanflow_jobs_active Number of currently active scan jobs.\n")
		fmt.Fprint(w, "# TYPE scanflow_jobs_active gauge\n")
		fmt.Fprintf(w, "scanflow_jobs_active %d\n", m.jobsActive.Load())

		// Pending gauge
		fmt.Fprint(w, "# HELP scanflow_jobs_pending Number of pending scan jobs.\n")
		fmt.Fprint(w, "# TYPE scanflow_jobs_pending gauge\n")
		fmt.Fprintf(w, "scanflow_jobs_pending %d\n", m.jobsPending.Load())

		// Pages
		fmt.Fprint(w, "# HELP scanflow_scan_pages_total Total number of scanned pages.\n")
		fmt.Fprint(w, "# TYPE scanflow_scan_pages_total counter\n")
		fmt.Fprintf(w, "scanflow_scan_pages_total %d\n", m.scanPagesTotal.Load())

		// HTTP requests
		fmt.Fprint(w, "# HELP scanflow_http_requests_total Total number of HTTP requests by status code class.\n")
		fmt.Fprint(w, "# TYPE scanflow_http_requests_total counter\n")
		fmt.Fprintf(w, "scanflow_http_requests_total{code=\"2xx\"} %d\n", m.httpRequests2xx.Load())
		fmt.Fprintf(w, "scanflow_http_requests_total{code=\"3xx\"} %d\n", m.httpRequests3xx.Load())
		fmt.Fprintf(w, "scanflow_http_requests_total{code=\"4xx\"} %d\n", m.httpRequests4xx.Load())
		fmt.Fprintf(w, "scanflow_http_requests_total{code=\"5xx\"} %d\n", m.httpRequests5xx.Load())

		// HTTP duration
		m.httpDurationMu.Lock()
		count := m.httpDurationCount
		sum := m.httpDurationSum
		m.httpDurationMu.Unlock()

		fmt.Fprint(w, "# HELP scanflow_http_request_duration_seconds Total duration of HTTP requests.\n")
		fmt.Fprint(w, "# TYPE scanflow_http_request_duration_seconds summary\n")
		fmt.Fprintf(w, "scanflow_http_request_duration_seconds_count %d\n", count)
		fmt.Fprintf(w, "scanflow_http_request_duration_seconds_sum %f\n", sum)
	}
}

// ---------------------------------------------------------------------------
// Middleware
// ---------------------------------------------------------------------------

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.statusCode = code
	sr.ResponseWriter.WriteHeader(code)
}

// MetricsMiddleware returns chi-compatible middleware that records every HTTP
// request's status code and duration.
func MetricsMiddleware(m *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rec, r)
			m.RecordHTTPRequest(rec.statusCode, time.Since(start))
		})
	}
}
