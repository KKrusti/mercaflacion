// Package middleware provides reusable HTTP middleware for the basket-cost API.
package middleware

import (
	"basket-cost/pkg/auth"
	"basket-cost/pkg/handlers"
	"basket-cost/pkg/store"
	"context"
	"net/http"
	"strings"
)

var allowedOrigins = map[string]bool{
	"http://localhost:5173":  true,
	"http://localhost:4173":  true,
	"https://localhost:5173": true,
}

// CORS applies permissive CORS headers for allowed origins.
func CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Requested-With, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

// SecurityHeaders sets standard security-related response headers.
func SecurityHeaders(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		next(w, r)
	}
}

// OptionalAuth returns a middleware that extracts a Bearer JWT, validates it,
// and injects the user ID into the request context. Unauthenticated requests
// pass through with userID=0 (anonymous mode).
func OptionalAuth(s store.Store) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if strings.HasPrefix(header, "Bearer ") {
				token := strings.TrimPrefix(header, "Bearer ")
				if userID, isAdmin, jti, _, err := auth.ValidateToken(token); err == nil && userID > 0 {
					revoked, rErr := s.IsTokenRevoked(jti)
					if rErr == nil && !revoked {
						ctx := context.WithValue(r.Context(), handlers.UserIDContextKey{}, userID)
						ctx = context.WithValue(ctx, handlers.IsAdminContextKey{}, isAdmin)
						r = r.WithContext(ctx)
					}
				}
			}
			next(w, r)
		}
	}
}
