package main

import (
	"basket-cost/internal/auth"
	"basket-cost/internal/database"
	"basket-cost/internal/enricher"
	"basket-cost/internal/handlers"
	"basket-cost/internal/ratelimit"
	"basket-cost/internal/store"
	"basket-cost/internal/ticket"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

var allowedOrigins = map[string]bool{
	"http://localhost:5173": true,
	"http://localhost:4173": true,
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
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

func securityHeadersMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		next(w, r)
	}
}

// optionalAuthMiddleware extracts a Bearer JWT from the Authorization header
// (if present), validates it, and checks it has not been revoked. On success
// the user ID is injected into the request context. Unauthenticated requests
// are passed through with userID=0 (anonymous mode).
func optionalAuthMiddleware(s store.Store) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if strings.HasPrefix(header, "Bearer ") {
				token := strings.TrimPrefix(header, "Bearer ")
				if userID, jti, _, err := auth.ValidateToken(token); err == nil && userID > 0 {
					revoked, rErr := s.IsTokenRevoked(jti)
					if rErr == nil && !revoked {
						ctx := context.WithValue(r.Context(), handlers.UserIDContextKey{}, userID)
						r = r.WithContext(ctx)
					}
				}
			}
			next(w, r)
		}
	}
}

func main() {
	// CRÍTICO: JWT_SECRET es obligatorio. Sin ella los tokens son trivialmente
	// falsificables usando el secreto por defecto hardcoded.
	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("JWT_SECRET no está configurada. Copia .env.example a .env y rellena el valor. " +
			"Genera una clave segura con: openssl rand -base64 32")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "basket-cost.db"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	db, err := database.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	s := store.New(db)
	imp := ticket.NewImporter(ticket.NewExtractor(), ticket.NewMercadonaParser(), s)
	enr := enricher.New(s)
	enr.Start(context.Background())
	h := handlers.New(s, imp, enr)

	// chain applies the standard middleware stack to any handler.
	authMiddleware := optionalAuthMiddleware(s)
	chain := func(handler http.HandlerFunc) http.HandlerFunc {
		return securityHeadersMiddleware(corsMiddleware(authMiddleware(handler)))
	}

	// authLimiter restricts login/register to 10 bursts, then 1 req / 10 s per IP.
	// This makes brute-force attacks impractical without blocking legitimate use.
	authLimiter := ratelimit.New(rate.Every(10*time.Second), 10)
	chainAuth := func(handler http.HandlerFunc) http.HandlerFunc {
		return chain(authLimiter.Middleware(handler))
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/auth/register", chainAuth(h.RegisterHandler))
	mux.HandleFunc("/api/auth/login", chainAuth(h.LoginHandler))
	mux.HandleFunc("/api/auth/logout", chain(h.LogoutHandler))
	mux.HandleFunc("/api/auth/password", chain(h.ChangePasswordHandler))
	mux.HandleFunc("/api/products", chain(h.SearchHandler))
	mux.HandleFunc("/api/products/", chain(h.ProductRouter))
	mux.HandleFunc("/api/tickets", chain(h.TicketHandler))
	mux.HandleFunc("/api/analytics", chain(h.AnalyticsHandler))
	mux.HandleFunc("/api/household", chain(h.HouseholdHandler))
	mux.HandleFunc("/api/household/invite", chain(h.HouseholdInviteHandler))
	mux.HandleFunc("/api/household/accept", chain(h.HouseholdAcceptHandler))
	mux.HandleFunc("/api/ipc", chain(h.IPCHandler))

	// Periodic cleanup of expired revoked-token entries (runs every hour).
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := s.CleanupExpiredTokens(); err != nil {
				log.Printf("server: cleanup expired tokens: %v", err)
			}
		}
	}()

	srv := &http.Server{
		Addr:              port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	fmt.Printf("Basket Cost API server running on http://localhost%s\n", port)
	log.Fatal(srv.ListenAndServe())
}
