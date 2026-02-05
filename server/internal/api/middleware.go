package api

import (
	"net/http"
	"strings"
)

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

			http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
		})
	}
}

// CORSMiddleware adds CORS headers for cross-origin requests.
func CORSMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
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
