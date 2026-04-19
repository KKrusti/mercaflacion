// Package ratelimit provides a per-IP token-bucket rate limiter middleware
// for net/http handlers. It uses golang.org/x/time/rate internally.
package ratelimit

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPLimiter enforces a per-IP rate limit using the token-bucket algorithm.
// Visitor state is stored in memory and pruned periodically.
type IPLimiter struct {
	mu       sync.Mutex
	visitors map[string]*entry
	r        rate.Limit
	burst    int
}

type entry struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

// New creates an IPLimiter that allows r tokens/sec with a maximum burst of
// burst tokens per IP address.
func New(r rate.Limit, burst int) *IPLimiter {
	il := &IPLimiter{
		visitors: make(map[string]*entry),
		r:        r,
		burst:    burst,
	}
	go il.cleanupLoop()
	return il
}

// Middleware returns an http.HandlerFunc middleware that rejects requests from
// IPs exceeding the configured limit with HTTP 429 Too Many Requests.
func (il *IPLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := remoteIP(r)
		if !il.allow(ip) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too many requests, please try again later", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

func (il *IPLimiter) allow(ip string) bool {
	il.mu.Lock()
	defer il.mu.Unlock()
	e, ok := il.visitors[ip]
	if !ok {
		e = &entry{lim: rate.NewLimiter(il.r, il.burst)}
		il.visitors[ip] = e
	}
	e.lastSeen = time.Now()
	return e.lim.Allow()
}

// cleanupLoop removes visitor entries that have been idle for more than
// 10 minutes. Runs every 5 minutes in the background.
func (il *IPLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		il.mu.Lock()
		for ip, e := range il.visitors {
			if time.Since(e.lastSeen) > 10*time.Minute {
				delete(il.visitors, ip)
			}
		}
		il.mu.Unlock()
	}
}

// remoteIP returns the real client IP. Behind Vercel's edge proxy the trusted
// client IP is in X-Real-Ip (injected by Vercel and not forwarded from the
// client). If that header is absent (direct connections, local dev), we take
// the rightmost value of X-Forwarded-For — the one appended by the nearest
// trusted proxy — to prevent spoofing via attacker-controlled leftmost values.
func remoteIP(r *http.Request) string {
	if realIP := r.Header.Get("X-Real-Ip"); realIP != "" {
		return strings.TrimSpace(realIP)
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
