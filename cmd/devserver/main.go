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
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	handler "basket-cost/api"
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

	fmt.Printf("Basket Cost API (dev) running on http://localhost%s\n", port)
	log.Fatal(srv.ListenAndServe())
}
