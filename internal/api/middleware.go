package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type clientRateLimiter struct {
	rate  rate.Limit
	burst int
	ttl   time.Duration

	mu          sync.Mutex
	clients     map[string]*clientLimiter
	lastCleanup time.Time
}

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newClientRateLimiter(rps float64, burst int) *clientRateLimiter {
	if rps <= 0 {
		return nil
	}
	if burst < 1 {
		burst = 1
	}
	return &clientRateLimiter{
		rate:    rate.Limit(rps),
		burst:   burst,
		ttl:     10 * time.Minute,
		clients: make(map[string]*clientLimiter),
	}
}

func (l *clientRateLimiter) middleware(next http.Handler) http.Handler {
	if l == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.getLimiter(clientKey(r)).Allow() {
			w.Header().Set("Retry-After", "1")
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *clientRateLimiter) getLimiter(client string) *rate.Limiter {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastCleanup) > time.Minute {
		for key, entry := range l.clients {
			if now.Sub(entry.lastSeen) > l.ttl {
				delete(l.clients, key)
			}
		}
		l.lastCleanup = now
	}

	entry, ok := l.clients[client]
	if !ok {
		entry = &clientLimiter{
			limiter:  rate.NewLimiter(l.rate, l.burst),
			lastSeen: now,
		}
		l.clients[client] = entry
	}
	entry.lastSeen = now
	return entry.limiter
}

func clientKey(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}

func hashAPIKeys(keys []string) [][sha256.Size]byte {
	hashes := make([][sha256.Size]byte, 0, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		hashes = append(hashes, sha256.Sum256([]byte(key)))
	}
	return hashes
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	if len(s.apiKeyHashes) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.validAPIKey(apiKeyFromRequest(r)) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) validAPIKey(candidate string) bool {
	if candidate == "" {
		return false
	}
	candidateHash := sha256.Sum256([]byte(candidate))
	for _, keyHash := range s.apiKeyHashes {
		if subtle.ConstantTimeCompare(candidateHash[:], keyHash[:]) == 1 {
			return true
		}
	}
	return false
}

func apiKeyFromRequest(r *http.Request) string {
	if key := strings.TrimSpace(r.Header.Get("X-API-Key")); key != "" {
		return key
	}

	scheme, token, ok := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " ")
	if ok && strings.EqualFold(scheme, "Bearer") {
		return strings.TrimSpace(token)
	}
	return ""
}
