package api

import (
	"log/slog"
	"net/http"
	"strings"
)

// maxRequestBodySize is the default limit for request bodies (1 MB).
const maxRequestBodySize int64 = 1 << 20

// AuthMiddleware validates API key authentication via Bearer token or X-API-Key header.
func AuthMiddleware(validKeys []string) func(http.Handler) http.Handler {
	keySet := make(map[string]bool, len(validKeys))
	for _, k := range validKeys {
		keySet[k] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check Bearer token
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				token := strings.TrimPrefix(auth, "Bearer ")
				if keySet[token] {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check X-API-Key header
			apiKey := r.Header.Get("X-API-Key")
			if keySet[apiKey] {
				next.ServeHTTP(w, r)
				return
			}

			// Check query parameter (for WebSocket connections)
			if key := r.URL.Query().Get("api_key"); keySet[key] {
				next.ServeHTTP(w, r)
				return
			}

			slog.Warn("authentication failed",
				"remote_addr", r.RemoteAddr,
				"method", r.Method,
				"path", r.URL.Path)
			http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		})
	}
}

// CORSMiddleware adds CORS headers for cross-origin requests.
// When allowedOrigins is empty the middleware permits all origins for
// backward-compatible local-network use.
func CORSMiddleware(allowedOrigins ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if len(allowed) == 0 {
				// No restrictions configured – allow all (local network default).
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if allowed[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-API-Key")
			w.Header().Set("Access-Control-Max-Age", "3600")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeadersMiddleware adds security-relevant HTTP response headers.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// MaxBodyMiddleware limits the size of incoming request bodies to prevent
// denial-of-service via excessively large payloads.
func MaxBodyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
		next.ServeHTTP(w, r)
	})
}
