package server

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const claimsKey contextKey = "claims"

// ClaimsFromContext extracts JWT Claims from an HTTP request context.
func ClaimsFromContext(ctx context.Context) *Claims {
	if v := ctx.Value(claimsKey); v != nil {
		return v.(*Claims)
	}
	return nil
}

// authMiddleware extracts and validates a JWT from the Authorization header.
// On success, it injects Claims into the request context.
// If required is true, returns 401 for missing/invalid tokens.
// If required is false, proceeds without claims for unauthenticated requests.
func authMiddleware(auth *AuthService, required bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			if required {
				http.Error(w, `{"error":"authorization required"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		// Expect "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, `{"error":"invalid authorization header"}`, http.StatusUnauthorized)
			return
		}

		claims, err := auth.ValidateToken(parts[1])
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// corsMiddleware adds CORS headers for whitelisted origins.
func corsMiddleware(allowedOrigins []string, next http.Handler) http.Handler {
	originSet := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[strings.ToLower(o)] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if len(originSet) == 0 || originSet[strings.ToLower(origin)] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// rateLimiter tracks per-IP request counts within a time window.
type rateLimiter struct {
	mu       sync.Mutex
	requests map[string]*rateBucket
	limit    int
	window   time.Duration
}

type rateBucket struct {
	count  int
	expiry time.Time
}

func newRateLimiter(requestsPerMinute int) *rateLimiter {
	return &rateLimiter{
		requests: make(map[string]*rateBucket),
		limit:    requestsPerMinute,
		window:   time.Minute,
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, ok := rl.requests[ip]
	if !ok || now.After(bucket.expiry) {
		rl.requests[ip] = &rateBucket{count: 1, expiry: now.Add(rl.window)}
		return true
	}
	bucket.count++
	return bucket.count <= rl.limit
}

// cleanup removes expired entries (call periodically).
func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	for ip, bucket := range rl.requests {
		if now.After(bucket.expiry) {
			delete(rl.requests, ip)
		}
	}
}

// rateLimitMiddleware rejects requests that exceed the per-IP rate limit.
func rateLimitMiddleware(rl *rateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if idx := strings.LastIndex(ip, ":"); idx >= 0 {
			ip = ip[:idx]
		}
		if !rl.allow(ip) {
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
