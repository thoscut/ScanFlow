package api

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
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

// tokenBucket tracks token state for a single client IP.
type tokenBucket struct {
	tokens   float64
	lastSeen time.Time
}

// maxRateLimitEntries caps the number of tracked IPs to prevent memory
// exhaustion from a flood of unique addresses.
const maxRateLimitEntries = 10000

// rateLimiter holds the shared state for rate limiting across all IPs.
type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*tokenBucket
	rate    float64 // tokens added per second
	burst   int     // max tokens (bucket capacity)
}

func newRateLimiter(ctx context.Context, rate float64, burst int) *rateLimiter {
	rl := &rateLimiter{
		clients: make(map[string]*tokenBucket),
		rate:    rate,
		burst:   burst,
	}
	go rl.cleanup(ctx)
	return rl
}

// allow checks whether a request from ip is permitted.
func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.clients[ip]
	if !ok {
		// Reject new entries if the map is at capacity to prevent memory
		// exhaustion from a large number of unique IPs.
		if len(rl.clients) >= maxRateLimitEntries {
			return false
		}
		rl.clients[ip] = &tokenBucket{
			tokens:   float64(rl.burst) - 1,
			lastSeen: now,
		}
		return true
	}

	// Refill tokens based on elapsed time.
	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens = math.Min(float64(rl.burst), b.tokens+elapsed*rl.rate)
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}

	b.tokens--
	return true
}

// cleanup removes entries that haven't been seen for over a minute.
// It stops when ctx is cancelled, allowing graceful shutdown.
func (rl *rateLimiter) cleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-time.Minute)
			for ip, b := range rl.clients {
				if b.lastSeen.Before(cutoff) {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// clientIP extracts the IP address from r.RemoteAddr, stripping the port.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RateLimitMiddleware limits requests per client IP using an in-memory token
// bucket. requestsPerSecond controls the refill rate and burst sets the maximum
// number of tokens (requests) that can be consumed in a burst.
func RateLimitMiddleware(requestsPerSecond float64, burst int) func(http.Handler) http.Handler {
	// Use a background context; the cleanup goroutine runs for the lifetime
	// of the process (the middleware is created once at startup).
	ctx, cancel := context.WithCancel(context.Background())
	rl := newRateLimiter(ctx, requestsPerSecond, burst)

	// Ensure the cleanup goroutine can be stopped if needed.
	_ = cancel // retained so the context can be cancelled in future if needed

	retryAfter := "1"
	if requestsPerSecond > 0 {
		retryAfter = fmt.Sprintf("%.0f", math.Ceil(1.0/requestsPerSecond))
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !rl.allow(ip) {
				slog.Warn("rate limit exceeded",
					"remote_addr", r.RemoteAddr,
					"method", r.Method,
					"path", r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", retryAfter)
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
