// Package handler is the Vercel Go serverless entry point for the basket-cost API.
// A single Handler function receives all /api/* requests and dispatches them
// through the same http.ServeMux used in the traditional server, so no handler
// logic needs to change.
package handler

import (
	"basket-cost/pkg/database"
	"basket-cost/pkg/emailfetcher"
	"basket-cost/pkg/enricher"
	"basket-cost/pkg/handlers"
	"basket-cost/pkg/middleware"
	"basket-cost/pkg/ratelimit"
	"basket-cost/pkg/store"
	"basket-cost/pkg/ticket"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

var (
	setupOnce sync.Once
	mux       *http.ServeMux
	setupErr  error
)

func setup() error {
	db, err := database.Open()
	if err != nil {
		return err
	}

	s := store.New(db)
	imp := ticket.NewImporter(ticket.NewExtractor(), ticket.NewMercadonaParser(), s)
	// The enricher is initialised so that FetchProductThumbnail works for manual
	// image URL resolution. Schedule() calls are silently dropped in serverless
	// (no background worker runs); full enrichment is triggered via GitHub Actions.
	enr := enricher.New(s)
	h := handlers.New(s, imp, enr)

	// Email fetcher: only wired when EMAIL_ENCRYPTION_KEY is present.
	var poller handlers.EmailPoller
	if key := os.Getenv("EMAIL_ENCRYPTION_KEY"); key != "" {
		poller = emailfetcher.New(s, imp, mustDecodeHexKey(key))
	}

	authMiddleware := middleware.OptionalAuth(s)
	chain := func(handler http.HandlerFunc) http.HandlerFunc {
		return middleware.SecurityHeaders(middleware.CORS(authMiddleware(handler)))
	}

	// Auth endpoints get an additional per-IP rate limiter.
	authLimiter := ratelimit.New(rate.Every(10*time.Second), 10)
	chainAuth := func(handler http.HandlerFunc) http.HandlerFunc {
		return chain(authLimiter.Middleware(handler))
	}

	m := http.NewServeMux()
	m.HandleFunc("/api/auth/register", chainAuth(h.RegisterHandler))
	m.HandleFunc("/api/auth/login", chainAuth(h.LoginHandler))
	m.HandleFunc("/api/auth/logout", chain(h.LogoutHandler))
	m.HandleFunc("/api/auth/password", chain(h.ChangePasswordHandler))
	m.HandleFunc("/api/products", chain(h.SearchHandler))
	m.HandleFunc("/api/products/", chain(h.ProductRouter))
	m.HandleFunc("/api/tickets", chain(h.TicketHandler))
	m.HandleFunc("/api/analytics", chain(h.AnalyticsHandler))
	m.HandleFunc("/api/household", chain(h.HouseholdHandler))
	m.HandleFunc("/api/household/invite", chain(h.HouseholdInviteHandler))
	m.HandleFunc("/api/household/accept", chain(h.HouseholdAcceptHandler))
	m.HandleFunc("/api/ipc", chain(h.IPCHandler))
	m.HandleFunc("/api/enrich/trigger", chain(h.EnrichTriggerHandler))
	m.HandleFunc("/api/email-account", chain(h.EmailAccountHandler))
	if poller != nil {
		m.HandleFunc("/api/cron/email-poll", h.CronEmailPollHandler(poller))
	}

	mux = m
	return nil
}

// mustDecodeHexKey decodes a 64-char hex string into 32 bytes.
// Panics on startup (inside setup()) if the key is malformed — same pattern
// used for other fatal startup errors.
func mustDecodeHexKey(hexKey string) []byte {
	key, err := hex.DecodeString(hexKey)
	if err != nil || len(key) != 32 {
		panic("EMAIL_ENCRYPTION_KEY must be a 64-character hex string (32 bytes)")
	}
	return key
}

// Handler is the Vercel serverless entry point. It is called for every request
// matched by the /api/:path* rewrite rule in vercel.json.
func Handler(w http.ResponseWriter, r *http.Request) {
	setupOnce.Do(func() {
		if err := setup(); err != nil {
			log.Printf("api: setup failed: %v", err)
			setupErr = err
		}
	})
	if setupErr != nil {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}
	mux.ServeHTTP(w, r)
}
