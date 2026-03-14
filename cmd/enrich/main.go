// Command enrich fetches product images from the Mercadona public API and
// persists them in the Neon PostgreSQL database.
//
// Usage:
//
//	go run ./cmd/enrich
//
// Requires DATABASE_URL environment variable (loaded from .env by task enrich).
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"basket-cost/pkg/database"
	"basket-cost/pkg/enricher"
	"basket-cost/pkg/store"
)

func main() {
	db, err := database.Open()
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	s := store.New(db)
	e := enricher.New(s)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	result, err := e.Run(ctx)
	if err != nil {
		log.Fatalf("enricher run: %v", err)
	}

	log.Printf("enricher done — total: %d, updated: %d, skipped: %d",
		result.Total, result.Updated, result.Skipped)
}
