// Command devserver runs the basket-cost API locally by delegating all requests
// to the same handler used by the Vercel serverless function.
// Use this for local development alongside the Vite frontend.
//
// Usage:
//
//	go run ./cmd/devserver
//
// Requires DATABASE_URL and JWT_SECRET environment variables.
package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	handler "basket-cost/api"
	"basket-cost/pkg/database"
	"basket-cost/pkg/emailfetcher"
	"basket-cost/pkg/store"
	"basket-cost/pkg/ticket"
)

func main() {
	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("JWT_SECRET no está configurada. Genera una clave segura con: openssl rand -base64 32")
	}
	if os.Getenv("DATABASE_URL") == "" {
		log.Fatal("DATABASE_URL no está configurada.")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	srv := &http.Server{
		Addr:              port,
		Handler:           http.HandlerFunc(handler.Handler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start the email poller in the background if configured.
	if hexKey := os.Getenv("EMAIL_ENCRYPTION_KEY"); hexKey != "" {
		key, err := hex.DecodeString(hexKey)
		if err != nil || len(key) != 32 {
			log.Fatal("EMAIL_ENCRYPTION_KEY must be a 64-character hex string (32 bytes)")
		}
		db, err := database.Open()
		if err != nil {
			log.Fatalf("email poller: open db: %v", err)
		}
		s := store.New(db)
		imp := ticket.NewImporter(ticket.NewExtractor(), ticket.NewMercadonaParser(), s)
		fetcher := emailfetcher.New(s, imp, key)

		interval := 48 * time.Hour
		if v := os.Getenv("EMAIL_POLL_INTERVAL"); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				interval = d
			}
		}
		ctx := context.Background()
		go emailfetcher.RunPoller(ctx, fetcher, interval)
		log.Printf("Email poller started (interval=%v)", interval)
	}

	fmt.Printf("Basket Cost API (dev) running on http://localhost%s\n", port)
	log.Fatal(srv.ListenAndServe())
}
