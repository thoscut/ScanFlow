package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSMiddlewareAllowsAllOriginsWhenNoneConfigured(t *testing.T) {
	handler := CORSMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected allow-origin '*', got %q", got)
	}
}

func TestCORSMiddlewareRestrictsOrigins(t *testing.T) {
	handler := CORSMiddleware("http://allowed.com")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Allowed origin
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://allowed.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://allowed.com" {
		t.Fatalf("expected allow-origin 'http://allowed.com', got %q", got)
	}

	// Disallowed origin
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "http://evil.com")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no allow-origin header for disallowed origin, got %q", got)
	}
}

func TestCORSMiddlewareHandlesPreflightOptions(t *testing.T) {
	handler := CORSMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for OPTIONS preflight")
	}))

	req := httptest.NewRequest("OPTIONS", "/api/v1/scan", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", w.Code)
	}

	methods := w.Header().Get("Access-Control-Allow-Methods")
	for _, m := range []string{"GET", "POST", "PUT", "DELETE"} {
		if !strings.Contains(methods, m) {
			t.Fatalf("expected method %s in allowed methods: %s", m, methods)
		}
	}
}

func TestSecurityHeadersMiddlewareSetsAllHeaders(t *testing.T) {
	handler := SecurityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	expected := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":        "DENY",
		"Content-Security-Policy": "default-src 'self'",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}
	for hdr, want := range expected {
		if got := w.Header().Get(hdr); got != want {
			t.Errorf("header %s = %q, want %q", hdr, got, want)
		}
	}
}

func TestMaxBodyMiddlewareLimitsBody(t *testing.T) {
	handler := MaxBodyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, maxRequestBodySize+1)
		_, err := r.Body.Read(buf)
		if err == nil {
			t.Fatal("expected error reading oversized body")
		}
		w.WriteHeader(http.StatusOK)
	}))

	bigBody := strings.NewReader(strings.Repeat("x", int(maxRequestBodySize)+1))
	req := httptest.NewRequest("POST", "/", bigBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
}

func TestAuthMiddlewareRejectsInvalidKey(t *testing.T) {
	handler := AuthMiddleware([]string{"valid-key"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddlewareAcceptsQueryParam(t *testing.T) {
	handler := AuthMiddleware([]string{"ws-key"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/?api_key=ws-key", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with query param key, got %d", w.Code)
	}
}

func TestAuthMiddlewareRejectsEmptyKeys(t *testing.T) {
	handler := AuthMiddleware([]string{"valid-key"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// No auth header at all
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRateLimitMiddlewareAllowsNormalRequests(t *testing.T) {
	handler := RateLimitMiddleware(10, 5)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, w.Code)
		}
	}
}

func TestRateLimitMiddlewareReturns429WhenExceeded(t *testing.T) {
	burst := 3
	handler := RateLimitMiddleware(0.1, burst)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust the burst allowance.
	for i := 0; i < burst; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.1:9999"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, w.Code)
		}
	}

	// Next request should be rate limited.
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json content-type, got %q", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"error"`) {
		t.Fatalf("expected JSON error response, got %q", body)
	}
}
