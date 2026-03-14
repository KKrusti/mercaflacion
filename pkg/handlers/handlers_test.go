package handlers_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"basket-cost/pkg/database"
	"basket-cost/pkg/handlers"
	"basket-cost/pkg/models"
	"basket-cost/pkg/store"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// testDB is a single shared connection pool for all tests in this package.
var testDB *sql.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("DATABASE_URL_UNPOOLED")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn != "" {
		var err error
		testDB, err = database.OpenDSN(dsn)
		if err != nil {
			log.Fatalf("handlers_test: open DB: %v", err)
		}
	}
	code := m.Run()
	if testDB != nil {
		testDB.Close()
	}
	os.Exit(code)
}

// newHandlers creates a Handlers instance backed by the shared PostgreSQL DB
// pre-seeded with a small set of deterministic products.
// Tests are skipped when DATABASE_URL is not set.
func newHandlers(t *testing.T) *handlers.Handlers {
	t.Helper()
	if testDB == nil {
		t.Skip("DATABASE_URL not set — skipping PostgreSQL integration tests")
	}
	truncateAll(t, testDB)
	t.Cleanup(func() { truncateAll(t, testDB) })

	s := store.New(testDB)

	seed := []models.Product{
		{
			ID:       "1",
			Name:     "LECHE ENTERA HACENDADO 1L",
			Category: "Lácteos",
			PriceHistory: []models.PriceRecord{
				{Date: time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC), Price: 0.79, Store: "Mercadona"},
				{Date: time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC), Price: 0.89, Store: "Mercadona"},
			},
		},
		{
			ID:       "2",
			Name:     "PAN BIMBO INTEGRAL",
			Category: "Panadería",
			PriceHistory: []models.PriceRecord{
				{Date: time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC), Price: 1.89, Store: "Carrefour"},
				{Date: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Price: 2.15, Store: "Carrefour"},
			},
		},
	}
	for _, p := range seed {
		if err := s.InsertProduct(p); err != nil {
			t.Fatalf("seed product %s: %v", p.ID, err)
		}
	}

	return handlers.New(s, nil, nil)
}

func truncateAll(t *testing.T, db *sql.DB) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("truncateAll begin: %v", err)
	}
	stmts := []string{
		`DELETE FROM revoked_tokens`,
		`DELETE FROM household_invitations`,
		`DELETE FROM processed_files`,
		`DELETE FROM price_records`,
		`DELETE FROM products`,
		`UPDATE users SET household_id = NULL`,
		`DELETE FROM users`,
		`DELETE FROM households`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			_ = tx.Rollback()
			t.Fatalf("truncateAll (%s): %v", stmt, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("truncateAll commit: %v", err)
	}
}

// --- SearchHandler ---

func TestSearchHandler_MethodNotAllowed(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodPost, "/api/products?q=leche", nil)
	w := httptest.NewRecorder()
	h.SearchHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestSearchHandler_EmptyQuery_ReturnsAll(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/products", nil)
	w := httptest.NewRecorder()
	h.SearchHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var results []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// Anonymous (userID=0) only sees seed data with user_id IS NULL.
	// InsertProduct does not set user_id so it uses NULL — visible to anonymous.
	if len(results) != 2 {
		t.Errorf("expected 2 seed products, got %d", len(results))
	}
}

func TestSearchHandler_WithQuery_FiltersResults(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/products?q=leche", nil)
	w := httptest.NewRecorder()
	h.SearchHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var results []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'leche', got %d", len(results))
	}
}

// --- ProductHandler ---

func TestProductHandler_MethodNotAllowed(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodPost, "/api/products/1", nil)
	w := httptest.NewRecorder()
	h.ProductHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestProductHandler_NotFound(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/products/nonexistent", nil)
	w := httptest.NewRecorder()
	h.ProductHandler(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestProductHandler_ExistingProduct(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/products/1", nil)
	w := httptest.NewRecorder()
	h.ProductHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var p map[string]any
	if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p["id"] != "1" {
		t.Errorf("expected id=1, got %v", p["id"])
	}
}

// --- RegisterHandler ---

func TestRegisterHandler_Success(t *testing.T) {
	h := newHandlers(t)
	body := `{"username":"testuser","password":"Password1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.RegisterHandler(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegisterHandler_DuplicateUsername(t *testing.T) {
	h := newHandlers(t)
	body := `{"username":"dupeuser","password":"Password1"}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	req1.Header.Set("Content-Type", "application/json")
	httptest.NewRecorder() // discard
	w1 := httptest.NewRecorder()
	h.RegisterHandler(w1, req1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first register: expected 201, got %d", w1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.RegisterHandler(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Errorf("duplicate register: expected 409, got %d", w2.Code)
	}
}

func TestRegisterHandler_WeakPassword(t *testing.T) {
	h := newHandlers(t)
	body := `{"username":"weakpwd","password":"weak"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.RegisterHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- LoginHandler ---

func TestLoginHandler_Success(t *testing.T) {
	h := newHandlers(t)
	reg := `{"username":"loginuser","password":"Password1"}`
	r1 := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(reg))
	r1.Header.Set("Content-Type", "application/json")
	h.RegisterHandler(httptest.NewRecorder(), r1)

	login := `{"username":"loginuser","password":"Password1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(login))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.LoginHandler(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["token"] == "" {
		t.Error("expected non-empty token")
	}
}

func TestLoginHandler_WrongPassword(t *testing.T) {
	h := newHandlers(t)
	reg := `{"username":"badpwduser","password":"Password1"}`
	r1 := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(reg))
	r1.Header.Set("Content-Type", "application/json")
	h.RegisterHandler(httptest.NewRecorder(), r1)

	login := `{"username":"badpwduser","password":"Wrong1234"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(login))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.LoginHandler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// --- TicketHandler ---

func TestTicketHandler_MethodNotAllowed(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/tickets", nil)
	w := httptest.NewRecorder()
	h.TicketHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestTicketHandler_MissingFile(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodPost, "/api/tickets", nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	w := httptest.NewRecorder()
	h.TicketHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTicketHandler_InvalidPDF(t *testing.T) {
	h := newHandlers(t)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "fake.pdf")
	_, _ = io.WriteString(fw, "not a pdf file content")
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/tickets", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	h.TicketHandler(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

// --- AnalyticsHandler ---

func TestAnalyticsHandler_MethodNotAllowed(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodPost, "/api/analytics", nil)
	w := httptest.NewRecorder()
	h.AnalyticsHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestAnalyticsHandler_ReturnsValidShape(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/analytics", nil)
	w := httptest.NewRecorder()
	h.AnalyticsHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["mostPurchased"]; !ok {
		t.Error("missing mostPurchased field")
	}
	if _, ok := resp["biggestIncreases"]; !ok {
		t.Error("missing biggestIncreases field")
	}
}

// --- DeletePriceRecordHandler ---

func TestDeletePriceRecordHandler_Unauthorized(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/products/1/prices/1", nil)
	w := httptest.NewRecorder()
	h.DeletePriceRecordHandler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestDeletePriceRecordHandler_NotFound(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/products/1/prices/999999", nil)
	ctx := context.WithValue(req.Context(), handlers.UserIDContextKey{}, int64(1))
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.DeletePriceRecordHandler(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- IPCHandler ---

func TestIPCHandler_MissingFrom(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/ipc", nil)
	w := httptest.NewRecorder()
	h.IPCHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestIPCHandler_ValidYear(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/ipc?from=2020", nil)
	w := httptest.NewRecorder()
	h.IPCHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["accumulated_rate"]; !ok {
		t.Error("missing accumulated_rate field")
	}
}

// --- HouseholdInviteHandler ---

func TestHouseholdInviteHandler_Unauthorized(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodPost, "/api/household/invite", nil)
	w := httptest.NewRecorder()
	h.HouseholdInviteHandler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// --- ProductImageHandler ---

func TestProductImageHandler_Unauthorized(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodPatch, "/api/products/1/image", nil)
	w := httptest.NewRecorder()
	h.ProductImageHandler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// --- LogoutHandler ---

func TestLogoutHandler_MethodNotAllowed(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/logout", nil)
	w := httptest.NewRecorder()
	h.LogoutHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// --- ChangePasswordHandler ---

func TestChangePasswordHandler_Unauthorized(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodPatch, "/api/auth/password", nil)
	w := httptest.NewRecorder()
	h.ChangePasswordHandler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// Ensure unused imports compile.
var _ = strconv.Itoa
var _ = errors.New
var _ = io.Discard
var _ = time.Now
var _ *sql.DB
