package handlers_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"basket-cost/internal/database"
	"basket-cost/internal/handlers"
	"basket-cost/internal/models"
	"basket-cost/internal/store"
	"basket-cost/internal/ticket"
)

// newHandlers creates a Handlers instance backed by an in-memory SQLite DB
// pre-seeded with a small set of deterministic products.
func newHandlers(t *testing.T) *handlers.Handlers {
	t.Helper()

	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	s := store.New(db)

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
	var results []models.SearchResult
	if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
		t.Fatalf("error decoding response: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected at least one result for empty query")
	}
}

func TestSearchHandler_WithQuery_ReturnsMatches(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/products?q=leche", nil)
	w := httptest.NewRecorder()
	h.SearchHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var results []models.SearchResult
	if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
		t.Fatalf("error decoding response: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected results for 'leche'")
	}
}

func TestSearchHandler_NoMatch_ReturnsEmptyArray(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/products?q=xyznonexistent", nil)
	w := httptest.NewRecorder()
	h.SearchHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var results []models.SearchResult
	if err := json.NewDecoder(w.Body).Decode(&results); err != nil {
		t.Fatalf("error decoding response: %v", err)
	}
	if results == nil {
		t.Error("expected empty array, not null")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchHandler_ContentTypeJSON(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/products?q=leche", nil)
	w := httptest.NewRecorder()
	h.SearchHandler(w, req)
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
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

func TestProductHandler_MissingID_ReturnsBadRequest(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/products/", nil)
	w := httptest.NewRecorder()
	h.ProductHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestProductHandler_NotFound(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/products/9999", nil)
	w := httptest.NewRecorder()
	h.ProductHandler(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestProductHandler_ValidID_ReturnsProduct(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/products/1", nil)
	w := httptest.NewRecorder()
	h.ProductHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var product models.Product
	if err := json.NewDecoder(w.Body).Decode(&product); err != nil {
		t.Fatalf("error decoding response: %v", err)
	}
	if product.ID != "1" {
		t.Errorf("expected ID '1', got '%s'", product.ID)
	}
	if len(product.PriceHistory) == 0 {
		t.Error("product should have price history")
	}
}

func TestProductHandler_ContentTypeJSON(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/products/1", nil)
	w := httptest.NewRecorder()
	h.ProductHandler(w, req)
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", ct)
	}
}

// --- TicketHandler fakes ---

// fakeExtractor and fakeParser are defined here so tests stay self-contained.

type fakeTicketExtractor struct {
	text string
	err  error
}

func (f *fakeTicketExtractor) Extract(_ io.ReaderAt, _ int64) (string, error) {
	return f.text, f.err
}

type fakeTicketParser struct {
	t   *ticket.Ticket
	err error
}

func (f *fakeTicketParser) Parse(_ string) (*ticket.Ticket, error) {
	return f.t, f.err
}

// stubTicketStore implements ticket.TicketStore; it satisfies store.Store via
// embedding a *store.SQLiteStore so the same Handlers struct can hold both.
// For TicketHandler tests we only need UpsertPriceRecord, so we use a plain DB.

// newHandlersWithImporter creates a Handlers instance wired with the given Importer.
func newHandlersWithImporter(t *testing.T, imp *ticket.Importer) *handlers.Handlers {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	s := store.New(db)
	return handlers.New(s, imp, nil)
}

// buildMultipartRequest creates a multipart POST request with a "file" field
// containing the given data.
func buildMultipartRequest(t *testing.T, data []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "ticket.pdf")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/tickets", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// sampleImportTicket returns a minimal Ticket used by fake parsers in handler tests.
func sampleImportTicket() *ticket.Ticket {
	return &ticket.Ticket{
		Store:         "Mercadona",
		Date:          time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC),
		InvoiceNumber: "4144-017-284404",
		Lines: []ticket.TicketLine{
			{Name: "LECHE ENTERA HACENDADO 1L", UnitPrice: 0.89, Quantity: 1},
		},
	}
}

// --- TicketHandler tests ---

func TestTicketHandler_MethodNotAllowed(t *testing.T) {
	imp := ticket.NewImporter(&fakeTicketExtractor{}, &fakeTicketParser{t: sampleImportTicket()}, store.New(mustOpenMemDB(t)))
	h := newHandlersWithImporter(t, imp)
	req := httptest.NewRequest(http.MethodGet, "/api/tickets", nil)
	w := httptest.NewRecorder()
	h.TicketHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestTicketHandler_MissingFileField_ReturnsBadRequest(t *testing.T) {
	imp := ticket.NewImporter(&fakeTicketExtractor{}, &fakeTicketParser{t: sampleImportTicket()}, store.New(mustOpenMemDB(t)))
	h := newHandlersWithImporter(t, imp)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/tickets", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	w := httptest.NewRecorder()
	h.TicketHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTicketHandler_ValidPDF_ReturnsCreated(t *testing.T) {
	db := mustOpenMemDB(t)
	imp := ticket.NewImporter(
		&fakeTicketExtractor{text: "raw text"},
		&fakeTicketParser{t: sampleImportTicket()},
		store.New(db),
	)
	h := newHandlersWithImporter(t, imp)

	req := buildMultipartRequest(t, []byte("%PDF-1.4 fake"))
	w := httptest.NewRecorder()
	h.TicketHandler(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTicketHandler_ValidPDF_ResponseJSON(t *testing.T) {
	db := mustOpenMemDB(t)
	imp := ticket.NewImporter(
		&fakeTicketExtractor{text: "raw text"},
		&fakeTicketParser{t: sampleImportTicket()},
		store.New(db),
	)
	h := newHandlersWithImporter(t, imp)

	req := buildMultipartRequest(t, []byte("%PDF-1.4 fake"))
	w := httptest.NewRecorder()
	h.TicketHandler(w, req)

	var resp struct {
		InvoiceNumber string `json:"invoiceNumber"`
		LinesImported int    `json:"linesImported"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.InvoiceNumber != "4144-017-284404" {
		t.Errorf("invoiceNumber: want %q, got %q", "4144-017-284404", resp.InvoiceNumber)
	}
	if resp.LinesImported != 1 {
		t.Errorf("linesImported: want 1, got %d", resp.LinesImported)
	}
}

func TestTicketHandler_NonPDFMagicBytes_ReturnsUnprocessable(t *testing.T) {
	imp := ticket.NewImporter(
		&fakeTicketExtractor{},
		&fakeTicketParser{},
		store.New(mustOpenMemDB(t)),
	)
	h := newHandlersWithImporter(t, imp)
	// File that does not start with "%PDF-" — must be rejected with 422 before
	// reaching the extractor.
	req := buildMultipartRequest(t, []byte("not a pdf"))
	w := httptest.NewRecorder()
	h.TicketHandler(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestTicketHandler_ImporterError_ReturnsUnprocessable(t *testing.T) {
	imp := ticket.NewImporter(
		&fakeTicketExtractor{err: errors.New("corrupt pdf")},
		&fakeTicketParser{},
		store.New(mustOpenMemDB(t)),
	)
	h := newHandlersWithImporter(t, imp)
	// The file starts with "%PDF-" but the extractor will fail internally.
	req := buildMultipartRequest(t, []byte("%PDF-1.4 corrupt content"))
	w := httptest.NewRecorder()
	h.TicketHandler(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

// mustOpenMemDB is a test helper that opens an in-memory SQLite database.
func mustOpenMemDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open mem DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// --- fakeEnricher ---

// fakeEnricher implements handlers.EnrichScheduler without touching the network.
type fakeEnricher struct {
	called chan struct{}
}

func newFakeEnricher() *fakeEnricher {
	return &fakeEnricher{called: make(chan struct{}, 1)}
}

func (f *fakeEnricher) Schedule() {
	f.called <- struct{}{}
}

func (f *fakeEnricher) FetchProductThumbnail(_ context.Context, _ string) (string, error) {
	return "", nil // not needed in unit tests
}

// --- enricher integration test ---

func TestTicketHandler_EnricherCalledAfterImport(t *testing.T) {
	db := mustOpenMemDB(t)
	imp := ticket.NewImporter(
		&fakeTicketExtractor{text: "raw text"},
		&fakeTicketParser{t: sampleImportTicket()},
		store.New(db),
	)
	enr := newFakeEnricher()

	s := store.New(mustOpenMemDB(t))
	h := handlers.New(s, imp, enr)

	req := buildMultipartRequest(t, []byte("%PDF-1.4 fake"))
	w := httptest.NewRecorder()
	h.TicketHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	// Schedule() is called synchronously inside TicketHandler, so the signal
	// must already be in the channel by the time the handler returns.
	select {
	case <-enr.called:
		// OK
	default:
		t.Fatal("enricher.Schedule was not called")
	}
}

func TestTicketHandler_EnricherNil_DoesNotPanic(t *testing.T) {
	db := mustOpenMemDB(t)
	imp := ticket.NewImporter(
		&fakeTicketExtractor{text: "raw text"},
		&fakeTicketParser{t: sampleImportTicket()},
		store.New(db),
	)
	h := handlers.New(store.New(mustOpenMemDB(t)), imp, nil)

	req := buildMultipartRequest(t, []byte("%PDF-1.4 fake"))
	w := httptest.NewRecorder()
	h.TicketHandler(w, req) // must not panic
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestTicketHandler_DuplicateFilename_ReturnsConflict(t *testing.T) {
	db := mustOpenMemDB(t)
	s := store.New(db)
	imp := ticket.NewImporter(
		&fakeTicketExtractor{text: "raw text"},
		&fakeTicketParser{t: sampleImportTicket()},
		s,
	)
	h := handlers.New(s, imp, nil)

	// First upload: must succeed.
	req1 := buildMultipartRequest(t, []byte("%PDF-1.4 fake"))
	w1 := httptest.NewRecorder()
	h.TicketHandler(w1, req1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first upload: expected 201, got %d: %s", w1.Code, w1.Body.String())
	}

	// Second upload of the same filename: must be rejected with 409.
	req2 := buildMultipartRequest(t, []byte("%PDF-1.4 fake"))
	w2 := httptest.NewRecorder()
	h.TicketHandler(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Errorf("second upload: expected 409 Conflict, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestTicketHandler_FileMarkedProcessedAfterSuccessfulImport(t *testing.T) {
	db := mustOpenMemDB(t)
	s := store.New(db)
	imp := ticket.NewImporter(
		&fakeTicketExtractor{text: "raw text"},
		&fakeTicketParser{t: sampleImportTicket()},
		s,
	)
	h := handlers.New(s, imp, nil)

	req := buildMultipartRequest(t, []byte("%PDF-1.4 fake"))
	w := httptest.NewRecorder()
	h.TicketHandler(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	// The filename used by buildMultipartRequest is "ticket.pdf".
	// The request has no JWT so UserIDFromContext returns 0.
	processed, err := s.IsFileProcessed(0, "ticket.pdf")
	if err != nil {
		t.Fatalf("IsFileProcessed: %v", err)
	}
	if !processed {
		t.Error("expected 'ticket.pdf' to be marked as processed after successful import")
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

func TestAnalyticsHandler_ReturnsOK(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/analytics", nil)
	w := httptest.NewRecorder()
	h.AnalyticsHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAnalyticsHandler_ContentTypeJSON(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/analytics", nil)
	w := httptest.NewRecorder()
	h.AnalyticsHandler(w, req)
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}
}

func TestAnalyticsHandler_ResponseShape(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/analytics", nil)
	w := httptest.NewRecorder()
	h.AnalyticsHandler(w, req)

	var resp struct {
		MostPurchased    []json.RawMessage `json:"mostPurchased"`
		BiggestIncreases []json.RawMessage `json:"biggestIncreases"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode analytics response: %v", err)
	}
	// Both arrays must be non-nil (may be empty, but not null).
	if resp.MostPurchased == nil {
		t.Error("mostPurchased must not be null")
	}
	if resp.BiggestIncreases == nil {
		t.Error("biggestIncreases must not be null")
	}
}

func TestAnalyticsHandler_MostPurchasedPopulated(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/analytics", nil)
	w := httptest.NewRecorder()
	h.AnalyticsHandler(w, req)

	var resp struct {
		MostPurchased []struct {
			ID            string  `json:"id"`
			Name          string  `json:"name"`
			PurchaseCount int     `json:"purchaseCount"`
			CurrentPrice  float64 `json:"currentPrice"`
		} `json:"mostPurchased"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// newHandlers seeds 2 products with 2 price records each.
	if len(resp.MostPurchased) == 0 {
		t.Error("expected at least one entry in mostPurchased")
	}
	for _, item := range resp.MostPurchased {
		if item.PurchaseCount <= 0 {
			t.Errorf("product %q: PurchaseCount should be > 0, got %d", item.ID, item.PurchaseCount)
		}
	}
}

// --- RegisterHandler ---

func newAuthHandlers(t *testing.T) *handlers.Handlers {
	t.Helper()
	db := mustOpenMemDB(t)
	s := store.New(db)
	return handlers.New(s, nil, nil)
}

func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return bytes.NewBuffer(b)
}

func TestRegisterHandler_MethodNotAllowed(t *testing.T) {
	h := newAuthHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/register", nil)
	w := httptest.NewRecorder()
	h.RegisterHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestRegisterHandler_MissingBody_ReturnsBadRequest(t *testing.T) {
	h := newAuthHandlers(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	h.RegisterHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRegisterHandler_ShortUsername_ReturnsBadRequest(t *testing.T) {
	h := newAuthHandlers(t)
	body := jsonBody(t, map[string]string{"username": "ab", "password": "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", body)
	w := httptest.NewRecorder()
	h.RegisterHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short username, got %d", w.Code)
	}
}

func TestRegisterHandler_ShortPassword_ReturnsBadRequest(t *testing.T) {
	h := newAuthHandlers(t)
	body := jsonBody(t, map[string]string{"username": "validuser", "password": "short"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", body)
	w := httptest.NewRecorder()
	h.RegisterHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short password, got %d", w.Code)
	}
}

func TestRegisterHandler_Success_ReturnsCreatedWithToken(t *testing.T) {
	h := newAuthHandlers(t)
	body := jsonBody(t, map[string]string{"username": "testuser", "password": "securepassword"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", body)
	w := httptest.NewRecorder()
	h.RegisterHandler(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Token    string `json:"token"`
		UserID   int64  `json:"userId"`
		Username string `json:"username"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.UserID == 0 {
		t.Error("expected non-zero userID")
	}
	if resp.Username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", resp.Username)
	}
}

func TestRegisterHandler_DuplicateUsername_ReturnsConflict(t *testing.T) {
	h := newAuthHandlers(t)
	body1 := jsonBody(t, map[string]string{"username": "dupeuser", "password": "securepassword"})
	req1 := httptest.NewRequest(http.MethodPost, "/api/auth/register", body1)
	w1 := httptest.NewRecorder()
	h.RegisterHandler(w1, req1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first register: expected 201, got %d", w1.Code)
	}

	body2 := jsonBody(t, map[string]string{"username": "dupeuser", "password": "anotherpassword"})
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/register", body2)
	w2 := httptest.NewRecorder()
	h.RegisterHandler(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Errorf("duplicate register: expected 409, got %d", w2.Code)
	}
}

// --- LoginHandler ---

func TestLoginHandler_MethodNotAllowed(t *testing.T) {
	h := newAuthHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	w := httptest.NewRecorder()
	h.LoginHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestLoginHandler_InvalidCredentials_ReturnsUnauthorized(t *testing.T) {
	h := newAuthHandlers(t)

	// Register first.
	reg := jsonBody(t, map[string]string{"username": "loginuser", "password": "correctpassword"})
	h.RegisterHandler(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/auth/register", reg))

	// Login with wrong password.
	body := jsonBody(t, map[string]string{"username": "loginuser", "password": "wrongpassword"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	w := httptest.NewRecorder()
	h.LoginHandler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestLoginHandler_UnknownUser_ReturnsUnauthorized(t *testing.T) {
	h := newAuthHandlers(t)
	body := jsonBody(t, map[string]string{"username": "nobody", "password": "doesntmatter"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	w := httptest.NewRecorder()
	h.LoginHandler(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// --- ProductRouter ---

func TestProductRouter_DispatchesGetToProductHandler(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/products/1", nil)
	w := httptest.NewRecorder()
	h.ProductRouter(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestProductRouter_DispatchesPatchImageToImageHandler(t *testing.T) {
	h := newHandlers(t)
	body := jsonBody(t, map[string]string{"imageUrl": "https://prod-mercadona.imgix.net/img.jpg"})
	req := httptest.NewRequest(http.MethodPatch, "/api/products/1/image", body)
	w := httptest.NewRecorder()
	h.ProductRouter(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProductRouter_WrongMethodForProduct_Returns405(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/products/1", nil)
	w := httptest.NewRecorder()
	h.ProductRouter(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// --- ProductImageHandler ---

func TestProductImageHandler_MethodNotAllowed(t *testing.T) {
	h := newHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/api/products/1/image", nil)
	w := httptest.NewRecorder()
	h.ProductImageHandler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestProductImageHandler_MissingImageURL_ReturnsBadRequest(t *testing.T) {
	h := newHandlers(t)
	body := jsonBody(t, map[string]string{"imageUrl": ""})
	req := httptest.NewRequest(http.MethodPatch, "/api/products/1/image", body)
	w := httptest.NewRecorder()
	h.ProductImageHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestProductImageHandler_ProductNotFound_Returns404(t *testing.T) {
	h := newHandlers(t)
	body := jsonBody(t, map[string]string{"imageUrl": "https://prod-mercadona.imgix.net/img.jpg"})
	req := httptest.NewRequest(http.MethodPatch, "/api/products/9999/image", body)
	w := httptest.NewRecorder()
	h.ProductImageHandler(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestProductImageHandler_ValidRequest_ReturnsOK(t *testing.T) {
	h := newHandlers(t)
	imageURL := "https://prod-mercadona.imgix.net/images/img.jpg"
	body := jsonBody(t, map[string]string{"imageUrl": imageURL})
	req := httptest.NewRequest(http.MethodPatch, "/api/products/1/image", body)
	w := httptest.NewRecorder()
	h.ProductImageHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID       string `json:"id"`
		ImageURL string `json:"imageUrl"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != "1" {
		t.Errorf("expected id '1', got %q", resp.ID)
	}
	if resp.ImageURL != imageURL {
		t.Errorf("expected imageUrl %q, got %q", imageURL, resp.ImageURL)
	}
}

func TestLoginHandler_Success_ReturnsTokenAndUserID(t *testing.T) {
	h := newAuthHandlers(t)

	// Register.
	reg := jsonBody(t, map[string]string{"username": "loginok", "password": "correctpassword"})
	regReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", reg)
	regW := httptest.NewRecorder()
	h.RegisterHandler(regW, regReq)
	if regW.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", regW.Code)
	}
	var regResp struct {
		UserID int64 `json:"userId"`
	}
	if err := json.NewDecoder(regW.Body).Decode(&regResp); err != nil {
		t.Fatalf("decode register response: %v", err)
	}

	// Login.
	body := jsonBody(t, map[string]string{"username": "loginok", "password": "correctpassword"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	w := httptest.NewRecorder()
	h.LoginHandler(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Token    string `json:"token"`
		UserID   int64  `json:"userId"`
		Username string `json:"username"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.UserID != regResp.UserID {
		t.Errorf("userID mismatch: register=%d, login=%d", regResp.UserID, resp.UserID)
	}
	if resp.Username != "loginok" {
		t.Errorf("expected username 'loginok', got %q", resp.Username)
	}
}
