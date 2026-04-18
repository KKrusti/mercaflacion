package handlers

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"basket-cost/pkg/crypto"
)

// EmailPoller is the interface the cron endpoint uses to trigger a manual poll.
type EmailPoller interface {
	PollAll(ctx context.Context)
}

// emailAccountRequest is the body for POST /api/email-account.
type emailAccountRequest struct {
	EmailAddress string `json:"emailAddress"`
	Password     string `json:"password"` // app password, never stored as-is
	IMAPHost     string `json:"imapHost"`
	IMAPPort     int    `json:"imapPort"`
}

// emailAccountResponse is the body for GET /api/email-account.
type emailAccountResponse struct {
	EmailAddress string `json:"emailAddress"`
	IMAPHost     string `json:"imapHost"`
	IMAPPort     int    `json:"imapPort"`
}

// emailEncryptionKey reads and validates EMAIL_ENCRYPTION_KEY from the environment.
// The key must be a 64-character hex string representing 32 bytes.
func emailEncryptionKey() ([]byte, error) {
	raw := os.Getenv("EMAIL_ENCRYPTION_KEY")
	if raw == "" {
		return nil, fmt.Errorf("EMAIL_ENCRYPTION_KEY is not set")
	}
	key, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("EMAIL_ENCRYPTION_KEY: invalid hex: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("EMAIL_ENCRYPTION_KEY must be 32 bytes (64 hex chars), got %d", len(key))
	}
	return key, nil
}

// EmailAccountHandler routes /api/email-account requests.
//
//	POST   → register or update the IMAP account for the authenticated user
//	GET    → return account info (no password)
//	DELETE → remove the account
func (h *Handlers) EmailAccountHandler(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r)
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.registerEmailAccount(w, r, userID)
	case http.MethodGet:
		h.getEmailAccount(w, r, userID)
	case http.MethodDelete:
		h.deleteEmailAccount(w, r, userID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handlers) registerEmailAccount(w http.ResponseWriter, r *http.Request, userID int64) {
	var req emailAccountRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.EmailAddress == "" || req.Password == "" {
		http.Error(w, "emailAddress y password son obligatorios", http.StatusBadRequest)
		return
	}
	if !emailRegex.MatchString(req.EmailAddress) {
		http.Error(w, "emailAddress no es válido", http.StatusBadRequest)
		return
	}
	if req.IMAPHost == "" {
		req.IMAPHost = "imap.gmail.com"
	}
	if req.IMAPPort == 0 {
		req.IMAPPort = 993
	}

	key, err := emailEncryptionKey()
	if err != nil {
		http.Error(w, "Configuración del servidor incorrecta", http.StatusInternalServerError)
		return
	}

	encrypted, err := crypto.Encrypt(req.Password, key)
	if err != nil {
		http.Error(w, "Error al cifrar la contraseña", http.StatusInternalServerError)
		return
	}

	if err := h.store.UpsertEmailAccount(userID, req.EmailAddress, encrypted, req.IMAPHost, req.IMAPPort); err != nil {
		http.Error(w, "Error al guardar la cuenta de correo", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(emailAccountResponse{ //nolint:errcheck
		EmailAddress: req.EmailAddress,
		IMAPHost:     req.IMAPHost,
		IMAPPort:     req.IMAPPort,
	})
}

func (h *Handlers) getEmailAccount(w http.ResponseWriter, r *http.Request, userID int64) {
	acc, err := h.store.GetEmailAccount(userID)
	if err != nil {
		http.Error(w, "Error al obtener la cuenta de correo", http.StatusInternalServerError)
		return
	}
	if acc == nil {
		http.Error(w, "No hay cuenta de correo configurada", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(emailAccountResponse{ //nolint:errcheck
		EmailAddress: acc.EmailAddress,
		IMAPHost:     acc.IMAPHost,
		IMAPPort:     acc.IMAPPort,
	})
}

func (h *Handlers) deleteEmailAccount(w http.ResponseWriter, r *http.Request, userID int64) {
	if err := h.store.DeleteEmailAccount(userID); err != nil {
		http.Error(w, "Error al eliminar la cuenta de correo", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CronEmailPollHandler handles GET /api/cron/email-poll.
// Protected by a shared secret (X-Cron-Secret header or CRON_SECRET query param)
// so that only Vercel Cron Jobs (or the local dev poller) can trigger it.
func (h *Handlers) CronEmailPollHandler(poller EmailPoller) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		secret := os.Getenv("CRON_SECRET")
		if secret != "" {
			provided := r.Header.Get("X-Cron-Secret")
			if provided == "" {
				provided = r.URL.Query().Get("cron_secret")
			}
			// Vercel Cron Jobs inject the secret as "Authorization: Bearer <CRON_SECRET>"
			if provided == "" {
				if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
					provided = strings.TrimPrefix(auth, "Bearer ")
				}
			}
			if !strings.EqualFold(provided, secret) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}

		poller.PollAll(r.Context())

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
	}
}
