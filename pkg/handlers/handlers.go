// Package handlers implements the HTTP handlers for the Basket Cost API.
package handlers

import (
	"basket-cost/pkg/auth"
	"basket-cost/pkg/models"
	"basket-cost/pkg/store"
	"basket-cost/pkg/ticket"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const pdfMagic = "%PDF-"

// maxJSONBodyBytes is the maximum size accepted for JSON request bodies.
// Prevents resource exhaustion from oversized payloads.
const maxJSONBodyBytes = 1 << 20 // 1 MB

// emailRegex is a lightweight format check — not RFC 5322 complete, but
// catches obvious non-email strings without importing a heavy validator.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// decodeJSONBody wraps r.Body with a size limit, then JSON-decodes into v.
// Returns false and writes an HTTP error when the body is too large or malformed.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Bad request: invalid JSON", http.StatusBadRequest)
		}
		return false
	}
	return true
}

// validatePasswordComplexity returns an error message when the password does
// not meet the minimum requirements, or an empty string when it is valid.
func validatePasswordComplexity(p string) string {
	if len(p) < 8 {
		return "la contraseña debe tener al menos 8 caracteres"
	}
	var hasUpper, hasLower, hasDigit bool
	for _, r := range p {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return "la contraseña debe contener mayúsculas, minúsculas y un número"
	}
	return ""
}

// validateImageURL returns an error when raw is not a valid http(s) URL.
func validateImageURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("URL inválida")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("el scheme de la URL debe ser http o https")
	}
	if u.Host == "" {
		return fmt.Errorf("la URL debe incluir un host")
	}
	return nil
}

// UserIDContextKey is the context key used to pass the authenticated user ID
// between the auth middleware (in cmd/server) and the HTTP handlers.
// It is exported so the middleware can set it without an import cycle.
type UserIDContextKey struct{}

// UserIDFromContext extracts the authenticated user ID from the request context.
// Returns 0 if no user ID is present (unauthenticated request).
func UserIDFromContext(r *http.Request) int64 {
	v, _ := r.Context().Value(UserIDContextKey{}).(int64)
	return v
}

// IsAdminContextKey is the context key for the authenticated user's admin flag.
type IsAdminContextKey struct{}

// IsAdminFromContext returns true when the authenticated user is an admin.
func IsAdminFromContext(r *http.Request) bool {
	v, _ := r.Context().Value(IsAdminContextKey{}).(bool)
	return v
}

// reProductPage matches Mercadona product page URLs and captures the numeric product ID.
// Example: https://tienda.mercadona.es/product/60722/chocolate-negro-...
var reProductPage = regexp.MustCompile(`^https?://tienda\.mercadona\.es/products?/(\d+)`)

// EnrichScheduler is the subset of *enricher.Enricher used by Handlers.
// Defined as an interface so tests can inject a fake without network calls.
type EnrichScheduler interface {
	Schedule()
	// FetchProductThumbnail resolves the direct image URL for a Mercadona
	// numeric product ID (as found in tienda.mercadona.es product page URLs).
	FetchProductThumbnail(ctx context.Context, productID string) (string, error)
}

type Handlers struct {
	store    store.Store
	importer *ticket.Importer
	enricher EnrichScheduler
}

// New returns a Handlers instance. enr may be nil to skip post-import enrichment.
func New(s store.Store, imp *ticket.Importer, enr EnrichScheduler) *Handlers {
	return &Handlers{store: s, importer: imp, enricher: enr}
}

// --- Auth handlers ---

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type loginResponse struct {
	Token    string `json:"token"`
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	IsAdmin  bool   `json:"isAdmin"`
}

func (h *Handlers) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req registerRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || utf8.RuneCountInString(req.Username) < 3 {
		http.Error(w, "Bad request: username must be at least 3 characters", http.StatusBadRequest)
		return
	}
	if msg := validatePasswordComplexity(req.Password); msg != "" {
		http.Error(w, "Bad request: "+msg, http.StatusBadRequest)
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email != "" && !emailRegex.MatchString(req.Email) {
		http.Error(w, "Bad request: email format is invalid", http.StatusBadRequest)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Printf("handlers: hash password: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	userID, err := h.store.CreateUser(req.Username, strings.TrimSpace(req.Email), hash)
	if err != nil {
		// Treat duplicate username as a client error.
		if strings.Contains(err.Error(), "UNIQUE") {
			http.Error(w, "Conflict: username already taken", http.StatusConflict)
			return
		}
		log.Printf("handlers: create user: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	token, err := auth.GenerateToken(userID, false)
	if err != nil {
		log.Printf("handlers: generate token: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(loginResponse{
		Token:    token,
		UserID:   userID,
		Username: req.Username,
		Email:    strings.TrimSpace(req.Email),
	}); err != nil {
		log.Printf("handlers: encode register response: %v", err)
	}
}

func (h *Handlers) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req registerRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	user, err := h.store.GetUserByUsername(strings.TrimSpace(req.Username))
	if err != nil {
		log.Printf("handlers: get user: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if user == nil || auth.CheckPassword(req.Password, user.PasswordHash) != nil {
		http.Error(w, "Unauthorized: invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateToken(user.ID, user.IsAdmin)
	if err != nil {
		log.Printf("handlers: generate token: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(loginResponse{
		Token:    token,
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		IsAdmin:  user.IsAdmin,
	}); err != nil {
		log.Printf("handlers: encode login response: %v", err)
	}
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// ChangePasswordHandler handles PATCH /api/auth/password.
// Requires a valid JWT. Verifies the current password and replaces it with the new one.
func (h *Handlers) ChangePasswordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := UserIDFromContext(r)
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req changePasswordRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	if msg := validatePasswordComplexity(req.NewPassword); msg != "" {
		http.Error(w, "Bad request: "+msg, http.StatusBadRequest)
		return
	}

	user, err := h.store.GetUserByID(userID)
	if err != nil {
		log.Printf("handlers: get user by id %d: %v", userID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if auth.CheckPassword(req.CurrentPassword, user.PasswordHash) != nil {
		http.Error(w, "Unauthorized: current password is incorrect", http.StatusUnauthorized)
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		log.Printf("handlers: hash new password: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := h.store.UpdateUserPassword(userID, newHash); err != nil {
		log.Printf("handlers: update password for user %d: %v", userID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"ok": true}); err != nil {
		log.Printf("handlers: encode change password response: %v", err)
	}
}

// --- Product handlers ---

// ProductRouter dispatches /api/products/{id}, /api/products/{id}/image, and
// /api/products/{id}/prices/{recordID} to the appropriate handler.
func (h *Handlers) ProductRouter(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.URL.Path, "/prices/") {
		h.DeletePriceRecordHandler(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/image") {
		h.ProductImageHandler(w, r)
		return
	}
	h.ProductHandler(w, r)
}

func (h *Handlers) SearchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := UserIDFromContext(r)
	results, err := h.store.SearchProducts(userID, r.URL.Query().Get("q"))
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		log.Printf("handlers: encode search response: %v", err)
	}
}

func (h *Handlers) ProductHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Path[len("/api/products/"):]
	if id == "" {
		http.Error(w, "Product ID required", http.StatusBadRequest)
		return
	}

	userID := UserIDFromContext(r)
	product, err := h.store.GetProductByID(userID, id)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if product == nil {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(product); err != nil {
		log.Printf("handlers: encode product response: %v", err)
	}
}

type productImageRequest struct {
	ImageURL string `json:"imageUrl"`
}

// ProductImageHandler handles PATCH /api/products/{id}/image.
// It sets a manually provided image URL and locks the product so the enricher
// will not overwrite it in future runs.
func (h *Handlers) ProductImageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Auth check first — before any expensive operations or external calls.
	imageUserID := UserIDFromContext(r)
	if imageUserID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Path: /api/products/{id}/image — strip prefix and suffix.
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/products/")
	id := strings.TrimSuffix(trimmed, "/image")
	if id == "" {
		http.Error(w, "Product ID required", http.StatusBadRequest)
		return
	}

	var req productImageRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	req.ImageURL = strings.TrimSpace(req.ImageURL)
	if req.ImageURL == "" {
		http.Error(w, "Bad request: imageUrl is required", http.StatusBadRequest)
		return
	}

	// If the user pasted a Mercadona product page URL instead of a direct image
	// URL, resolve the thumbnail automatically using the Mercadona catalogue API.
	if h.enricher != nil {
		if m := reProductPage.FindStringSubmatch(req.ImageURL); len(m) == 2 {
			resolved, err := h.enricher.FetchProductThumbnail(r.Context(), m[1])
			if err != nil {
				log.Printf("handlers: resolve mercadona thumbnail for %s: %v", m[1], err)
				http.Error(w, "Unprocessable entity: could not resolve image from Mercadona product URL", http.StatusUnprocessableEntity)
				return
			}
			req.ImageURL = resolved
		}
	}

	// Validate the final URL scheme to prevent javascript:, file://, data:, etc.
	if err := validateImageURL(req.ImageURL); err != nil {
		http.Error(w, "Bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	product, err := h.store.GetProductByID(imageUserID, id)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if product == nil {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	if err := h.store.SetProductImageURLManual(id, req.ImageURL); err != nil {
		log.Printf("handlers: set manual image for %s: %v", id, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"id": id, "imageUrl": req.ImageURL}); err != nil {
		log.Printf("handlers: encode image response: %v", err)
	}
}

// DeletePriceRecordHandler handles DELETE /api/products/{id}/prices/{recordID}.
// Removes a single price record that belongs to the authenticated user's household.
func (h *Handlers) DeletePriceRecordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := UserIDFromContext(r)
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Path: /api/products/{productID}/prices/{recordID}
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/products/")
	parts := strings.SplitN(trimmed, "/prices/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, "Bad request: invalid path", http.StatusBadRequest)
		return
	}
	recordID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		http.Error(w, "Bad request: recordId must be an integer", http.StatusBadRequest)
		return
	}

	if err := h.store.DeletePriceRecord(recordID, userID); err != nil {
		log.Printf("handlers: delete price record %d for user %d: %v", recordID, userID, err)
		http.Error(w, "Not found: record not found or not owned by user", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"ok": true}); err != nil {
		log.Printf("handlers: encode delete price record response: %v", err)
	}
}

// LogoutHandler handles POST /api/auth/logout.
// Revokes the caller's JWT so it cannot be reused even before its expiry.
func (h *Handlers) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	tokenStr := strings.TrimPrefix(header, "Bearer ")

	_, _, jti, expiresAt, err := auth.ValidateToken(tokenStr)
	if err != nil || jti == "" {
		// Token is already invalid; treat logout as successful.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		return
	}

	if err := h.store.RevokeToken(jti, expiresAt); err != nil {
		log.Printf("handlers: revoke token: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"ok": true}); err != nil {
		log.Printf("handlers: encode logout response: %v", err)
	}
}

type ticketResponse struct {
	InvoiceNumber string `json:"invoiceNumber"`
	LinesImported int    `json:"linesImported"`
}

type analyticsResponse struct {
	MostPurchased    []models.MostPurchasedProduct `json:"mostPurchased"`
	BiggestIncreases []models.PriceIncreaseProduct `json:"biggestIncreases"`
}

func (h *Handlers) TicketHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := UserIDFromContext(r)

	const maxUploadSize = 10 << 20
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "Bad request: could not parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Bad request: missing 'file' field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := header.Filename

	already, err := h.store.IsFileProcessed(userID, filename)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if already {
		http.Error(w, "Conflict: file already imported", http.StatusConflict)
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Internal server error: could not read file", http.StatusInternalServerError)
		return
	}

	// Validate PDF magic bytes before invoking the parser to reject non-PDF uploads early.
	if len(data) < len(pdfMagic) || string(data[:len(pdfMagic)]) != pdfMagic {
		http.Error(w, "Unprocessable entity: file does not appear to be a valid PDF", http.StatusUnprocessableEntity)
		return
	}

	result, err := h.importer.Import(userID, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		log.Printf("handlers: ticket import failed for %q: %v", filename, err)
		http.Error(w, "Unprocessable entity: could not parse the PDF as a Mercadona receipt", http.StatusUnprocessableEntity)
		return
	}

	if err := h.store.MarkFileProcessed(userID, filename, time.Now()); err != nil {
		// Non-fatal: the import succeeded; log and continue.
		log.Printf("handlers: could not mark file processed %q: %v", filename, err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(ticketResponse{
		InvoiceNumber: result.InvoiceNumber,
		LinesImported: result.LinesImported,
	}); err != nil {
		log.Printf("handlers: encode ticket response: %v", err)
	}

	// Concurrent Schedule calls are coalesced by the enricher, so batch uploads
	// trigger only one enrichment run.
	if h.enricher != nil {
		h.enricher.Schedule()
	}
}

const analyticsLimit = 10

func (h *Handlers) AnalyticsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := UserIDFromContext(r)

	mostPurchased, err := h.store.GetMostPurchased(userID, analyticsLimit)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	biggestIncreases, err := h.store.GetBiggestPriceIncreases(userID, analyticsLimit)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(analyticsResponse{
		MostPurchased:    mostPurchased,
		BiggestIncreases: biggestIncreases,
	}); err != nil {
		log.Printf("handlers: encode analytics response: %v", err)
	}
}

// --- Household handlers ---

// HouseholdHandler dispatches GET (list members) and DELETE (leave) for /api/household.
func (h *Handlers) HouseholdHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getHousehold(w, r)
	case http.MethodDelete:
		h.leaveHousehold(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

type householdMemberDTO struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

func (h *Handlers) getHousehold(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r)
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	members, err := h.store.GetHouseholdMembers(userID)
	if err != nil {
		log.Printf("handlers: get household members: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	dtos := make([]householdMemberDTO, len(members))
	for i, m := range members {
		dtos[i] = householdMemberDTO{ID: m.ID, Username: m.Username}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"members": dtos}); err != nil {
		log.Printf("handlers: encode household response: %v", err)
	}
}

func (h *Handlers) leaveHousehold(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r)
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if err := h.store.RemoveUserFromHousehold(userID); err != nil {
		log.Printf("handlers: leave household: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"ok": true}); err != nil {
		log.Printf("handlers: encode leave response: %v", err)
	}
}

// HouseholdInviteHandler handles POST /api/household/invite.
// Creates a 24-hour invitation token. If the caller has no household, one is created.
func (h *Handlers) HouseholdInviteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := UserIDFromContext(r)
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	token, err := h.store.CreateHouseholdInvitation(userID)
	if err != nil {
		log.Printf("handlers: create invitation: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"token": token}); err != nil {
		log.Printf("handlers: encode invite response: %v", err)
	}
}

// HouseholdAcceptHandler handles POST /api/household/accept?token=<tok>.
// The authenticated user joins the household identified by the invitation token.
func (h *Handlers) HouseholdAcceptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := UserIDFromContext(r)
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Bad request: missing token", http.StatusBadRequest)
		return
	}
	inv, err := h.store.GetHouseholdInvitation(token)
	if err != nil {
		log.Printf("handlers: get invitation: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if inv == nil || time.Now().UTC().After(inv.ExpiresAt) {
		http.Error(w, "Not found: invitation not found or expired", http.StatusNotFound)
		return
	}
	if inv.InviterID == userID {
		http.Error(w, "Bad request: cannot accept your own invitation", http.StatusBadRequest)
		return
	}
	if err := h.store.AddUserToHousehold(userID, inv.HouseholdID); err != nil {
		log.Printf("handlers: add to household: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	// Best-effort deletion; invitation was consumed regardless.
	if err := h.store.DeleteHouseholdInvitation(token); err != nil {
		log.Printf("handlers: delete invitation %s: %v", token, err)
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"ok": true}); err != nil {
		log.Printf("handlers: encode accept response: %v", err)
	}
}

// IPCHandler handles GET /api/ipc?from=<year> and returns the compound
// interannual IPC for Catalonia accumulated from the given year to the most
// recent available year in the database.
func (h *Handlers) IPCHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	fromStr := r.URL.Query().Get("from")
	if fromStr == "" {
		http.Error(w, "missing 'from' query parameter", http.StatusBadRequest)
		return
	}
	fromYear, err := strconv.Atoi(fromStr)
	currentYear := time.Now().Year()
	if err != nil || fromYear < 2000 || fromYear > currentYear {
		http.Error(w, "invalid 'from' year", http.StatusBadRequest)
		return
	}

	rate, toYear, err := h.store.GetAccumulatedIPC(fromYear)
	if err != nil {
		log.Printf("handlers: get accumulated ipc from %d: %v", fromYear, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	result := models.IPCResult{
		FromYear:        fromYear,
		ToYear:          toYear,
		AccumulatedRate: rate,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("handlers: encode ipc response: %v", err)
	}
}

// EnrichTriggerHandler handles POST /api/enrich/trigger.
// Only admin users may call this endpoint. It dispatches the enrich workflow
// on GitHub Actions via workflow_dispatch so the long-running enrichment job
// runs outside of Vercel's request timeout.
func (h *Handlers) EnrichTriggerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !IsAdminFromContext(r) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	ghToken := os.Getenv("GH_WORKFLOW_TOKEN")
	if ghToken == "" {
		http.Error(w, "Internal server error: enrichment not configured", http.StatusInternalServerError)
		return
	}

	body := `{"ref":"master"}`
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost,
		"https://api.github.com/repos/KKrusti/basket-cost/actions/workflows/enrich.yml/dispatches",
		strings.NewReader(body))
	if err != nil {
		log.Printf("handlers: enrich trigger: build request: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+ghToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("handlers: enrich trigger: call github: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		log.Printf("handlers: enrich trigger: github returned %d", resp.StatusCode)
		http.Error(w, "Internal server error: could not trigger enrichment", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
