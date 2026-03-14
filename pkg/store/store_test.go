package store_test

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"basket-cost/pkg/database"
	"basket-cost/pkg/models"
	"basket-cost/pkg/store"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// testDB is a single shared connection pool for all tests in this package.
// Using one pool avoids deadlocks caused by concurrent TRUNCATE operations
// when multiple *sql.DB instances target the same Neon/PgBouncer endpoint.
var testDB *sql.DB

func TestMain(m *testing.M) {
	// Prefer the unpooled connection for tests: TRUNCATE RESTART IDENTITY CASCADE
	// requires DDL locks that can deadlock when PgBouncer transaction mode dispatches
	// concurrent statements to different backend connections.
	dsn := os.Getenv("DATABASE_URL_UNPOOLED")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn != "" {
		var err error
		testDB, err = database.OpenDSN(dsn)
		if err != nil {
			log.Fatalf("store_test: open DB: %v", err)
		}
	}
	code := m.Run()
	if testDB != nil {
		testDB.Close()
	}
	os.Exit(code)
}

// newTestStore returns a store backed by the shared test DB with a clean slate.
// Tests are skipped when DATABASE_URL is not set.
func newTestStore(t *testing.T) *store.PostgresStore {
	t.Helper()
	if testDB == nil {
		t.Skip("DATABASE_URL not set — skipping PostgreSQL integration tests")
	}
	truncateAll(t, testDB)
	t.Cleanup(func() { truncateAll(t, testDB) })
	return store.New(testDB)
}

// truncateAll removes all mutable data, preserving ipc_rates (static seed).
// Runs inside a single transaction so FK checks see the same in-transaction
// snapshot (e.g. DELETE FROM users sees that price_records are already gone).
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

// createTestUser inserts a user with a unique name derived from the test name.
func createTestUser(t *testing.T, s *store.PostgresStore) int64 {
	t.Helper()
	// Use t.Name() to guarantee uniqueness across parallel and sequential tests.
	username := fmt.Sprintf("u-%x", []byte(t.Name()))
	if len(username) > 40 {
		username = username[:40]
	}
	id, err := s.CreateUser(username, "", "$2a$12$fakehashfortesting000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("createTestUser: %v", err)
	}
	return id
}

func date(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func sampleProduct(id string) models.Product {
	return models.Product{
		ID:       id,
		Name:     "LECHE ENTERA HACENDADO 1L",
		Category: "Lácteos",
		PriceHistory: []models.PriceRecord{
			{Date: date(2025, 1, 10), Price: 0.79, Store: "Mercadona"},
			{Date: date(2025, 6, 15), Price: 0.85, Store: "Mercadona"},
			{Date: date(2026, 1, 20), Price: 0.89, Store: "Mercadona"},
		},
	}
}

func insertProductForUser(t *testing.T, s *store.PostgresStore, userID int64, p models.Product) {
	t.Helper()
	entries := make([]models.PriceRecordEntry, len(p.PriceHistory))
	for i, r := range p.PriceHistory {
		entries[i] = models.PriceRecordEntry{Name: p.Name, Record: r}
	}
	if err := s.UpsertPriceRecordBatch(userID, entries); err != nil {
		t.Fatalf("insertProductForUser %q: %v", p.Name, err)
	}
}

// ---------- InsertProduct ----------

func TestInsertProduct_Success(t *testing.T) {
	s := newTestStore(t)
	p := sampleProduct("1")
	if err := s.InsertProduct(p); err != nil {
		t.Fatalf("InsertProduct returned unexpected error: %v", err)
	}
}

func TestInsertProduct_Idempotent(t *testing.T) {
	s := newTestStore(t)
	p := sampleProduct("1")
	if err := s.InsertProduct(p); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := s.InsertProduct(p); err != nil {
		t.Fatalf("second insert (idempotent): %v", err)
	}
}

func TestInsertProduct_PriceHistoryPersisted(t *testing.T) {
	s := newTestStore(t)
	p := sampleProduct("42")
	if err := s.InsertProduct(p); err != nil {
		t.Fatalf("InsertProduct: %v", err)
	}
	got, err := s.GetProductByID(0, "42")
	if err != nil {
		t.Fatalf("GetProductByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected product, got nil")
	}
	if len(got.PriceHistory) != len(p.PriceHistory) {
		t.Errorf("price history length: want %d, got %d", len(p.PriceHistory), len(got.PriceHistory))
	}
}

// ---------- GetProductByID ----------

func TestGetProductByID_NotFound(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetProductByID(0, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for missing product, got %+v", got)
	}
}

func TestGetProductByID_Fields(t *testing.T) {
	s := newTestStore(t)
	p := sampleProduct("7")
	if err := s.InsertProduct(p); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := s.GetProductByID(0, "7")
	if err != nil {
		t.Fatalf("GetProductByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected product, got nil")
	}
	tests := []struct{ name, got, want string }{
		{"ID", got.ID, p.ID},
		{"Name", got.Name, p.Name},
		{"Category", got.Category, p.Category},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("want %q, got %q", tt.want, tt.got)
			}
		})
	}
}

func TestGetProductByID_CurrentPriceDerivedFromLatestRecord(t *testing.T) {
	s := newTestStore(t)
	p := sampleProduct("5")
	if err := s.InsertProduct(p); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := s.GetProductByID(0, "5")
	if err != nil {
		t.Fatalf("GetProductByID: %v", err)
	}
	if got.CurrentPrice != 0.89 {
		t.Errorf("CurrentPrice: want 0.89, got %f", got.CurrentPrice)
	}
}

func TestGetProductByID_PriceHistoryOrderedByDate(t *testing.T) {
	s := newTestStore(t)
	p := models.Product{
		ID:   "99",
		Name: "TEST PRODUCT",
		PriceHistory: []models.PriceRecord{
			{Date: date(2025, 6, 1), Price: 2.00, Store: "A"},
			{Date: date(2025, 1, 1), Price: 1.00, Store: "A"},
			{Date: date(2025, 12, 1), Price: 3.00, Store: "A"},
		},
	}
	if err := s.InsertProduct(p); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := s.GetProductByID(0, "99")
	if err != nil {
		t.Fatalf("GetProductByID: %v", err)
	}
	for i := 1; i < len(got.PriceHistory); i++ {
		if got.PriceHistory[i].Date.Before(got.PriceHistory[i-1].Date) {
			t.Errorf("price history not ordered by date at index %d", i)
		}
	}
}

// ---------- SearchProducts ----------

func TestSearchProducts_EmptyQuery_ReturnsAll(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	products := []models.Product{
		{ID: "a", Name: "LECHE ENTERA", Category: "Lácteos", PriceHistory: []models.PriceRecord{
			{Date: date(2025, 1, 1), Price: 0.89, Store: "Mercadona"},
		}},
		{ID: "b", Name: "PAN INTEGRAL", Category: "Panadería", PriceHistory: []models.PriceRecord{
			{Date: date(2025, 1, 1), Price: 1.25, Store: "Mercadona"},
		}},
	}
	for _, p := range products {
		insertProductForUser(t, s, uid, p)
	}
	results, err := s.SearchProducts(uid, "")
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("want 2 results, got %d", len(results))
	}
}

func TestSearchProducts_WithQuery_ReturnsMatches(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	products := []models.Product{
		{ID: "a", Name: "LECHE ENTERA HACENDADO", PriceHistory: []models.PriceRecord{
			{Date: date(2025, 1, 1), Price: 0.89, Store: "Mercadona"},
		}},
		{ID: "b", Name: "PAN INTEGRAL BIMBO", PriceHistory: []models.PriceRecord{
			{Date: date(2025, 1, 1), Price: 1.25, Store: "Mercadona"},
		}},
	}
	for _, p := range products {
		insertProductForUser(t, s, uid, p)
	}
	results, err := s.SearchProducts(uid, "leche")
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("want 1 result, got %d", len(results))
	}
	if len(results) > 0 && results[0].ID != "leche-entera-hacendado" {
		t.Errorf("want product 'leche-entera-hacendado', got %q", results[0].ID)
	}
}

func TestSearchProducts_NoMatch_ReturnsEmptySlice(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	insertProductForUser(t, s, uid, sampleProduct("1"))
	results, err := s.SearchProducts(uid, "xyznonexistent")
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if results == nil {
		t.Error("want empty slice, got nil")
	}
	if len(results) != 0 {
		t.Errorf("want 0 results, got %d", len(results))
	}
}

func TestSearchProducts_CaseInsensitive(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	p := models.Product{
		ID:   "ci",
		Name: "LECHE ENTERA",
		PriceHistory: []models.PriceRecord{
			{Date: date(2025, 1, 1), Price: 0.89, Store: "Mercadona"},
		},
	}
	insertProductForUser(t, s, uid, p)
	for _, q := range []string{"leche", "LECHE", "Leche", "lEcHe"} {
		t.Run(q, func(t *testing.T) {
			results, err := s.SearchProducts(uid, q)
			if err != nil {
				t.Fatalf("SearchProducts(%q): %v", q, err)
			}
			if len(results) != 1 {
				t.Errorf("query %q: want 1 result, got %d", q, len(results))
			}
		})
	}
}

func TestSearchProducts_MinMaxPrice(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	p := models.Product{
		ID:   "mm",
		Name: "ACEITE OLIVA",
		PriceHistory: []models.PriceRecord{
			{Date: date(2025, 1, 1), Price: 5.00, Store: "A"},
			{Date: date(2025, 6, 1), Price: 8.00, Store: "A"},
			{Date: date(2025, 12, 1), Price: 6.50, Store: "A"},
		},
	}
	insertProductForUser(t, s, uid, p)
	results, err := s.SearchProducts(uid, "")
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	r := results[0]
	if r.MinPrice != 5.00 {
		t.Errorf("MinPrice: want 5.00, got %f", r.MinPrice)
	}
	if r.MaxPrice != 8.00 {
		t.Errorf("MaxPrice: want 8.00, got %f", r.MaxPrice)
	}
}

func TestSearchProducts_LastPurchaseDatePopulated(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	p := models.Product{
		ID:   "lpd",
		Name: "YOGUR NATURAL",
		PriceHistory: []models.PriceRecord{
			{Date: date(2025, 1, 1), Price: 0.30, Store: "Mercadona"},
			{Date: date(2025, 9, 15), Price: 0.35, Store: "Mercadona"},
		},
	}
	insertProductForUser(t, s, uid, p)
	results, err := s.SearchProducts(uid, "")
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].LastPurchaseDate != "2025-09-15" {
		t.Errorf("LastPurchaseDate: want %q, got %q", "2025-09-15", results[0].LastPurchaseDate)
	}
}

func TestSearchProducts_OrderedByLastPurchaseDateDesc(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	older := models.Product{
		Name:         "PAN INTEGRAL",
		PriceHistory: []models.PriceRecord{{Date: date(2024, 1, 1), Price: 1.00, Store: "A"}},
	}
	newer := models.Product{
		Name:         "LECHE ENTERA",
		PriceHistory: []models.PriceRecord{{Date: date(2026, 2, 1), Price: 0.89, Store: "A"}},
	}
	insertProductForUser(t, s, uid, older)
	insertProductForUser(t, s, uid, newer)
	results, err := s.SearchProducts(uid, "")
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	if results[0].ID != "leche-entera" {
		t.Errorf("first result: want %q (newest), got %q", "leche-entera", results[0].ID)
	}
	if results[1].ID != "pan-integral" {
		t.Errorf("second result: want %q (oldest), got %q", "pan-integral", results[1].ID)
	}
}

func TestSearchProducts_EmptyDB_ReturnsEmptySlice(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	results, err := s.SearchProducts(uid, "")
	if err != nil {
		t.Fatalf("SearchProducts on empty DB: %v", err)
	}
	if results == nil {
		t.Error("want empty slice, got nil")
	}
	if len(results) != 0 {
		t.Errorf("want 0 results, got %d", len(results))
	}
}

// ---------- UpdateProductImageURL ----------

func TestUpdateProductImageURL_SetsURL(t *testing.T) {
	s := newTestStore(t)
	p := sampleProduct("img-test")
	if err := s.InsertProduct(p); err != nil {
		t.Fatalf("insert: %v", err)
	}
	const url = "https://prod-mercadona.imgix.net/images/abc123.jpg?fit=crop&h=300&w=300"
	if err := s.UpdateProductImageURL("img-test", url); err != nil {
		t.Fatalf("UpdateProductImageURL: %v", err)
	}
	got, err := s.GetProductByID(0, "img-test")
	if err != nil {
		t.Fatalf("GetProductByID: %v", err)
	}
	if got.ImageURL != url {
		t.Errorf("ImageURL: want %q, got %q", url, got.ImageURL)
	}
}

func TestUpdateProductImageURL_NoOpOnMissingProduct(t *testing.T) {
	s := newTestStore(t)
	if err := s.UpdateProductImageURL("nonexistent", "http://example.com/img.jpg"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------- IsFileProcessed / MarkFileProcessed ----------

func TestIsFileProcessed_UnknownFile_ReturnsFalse(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	got, err := s.IsFileProcessed(uid, "ticket-2026-01.pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected false for unknown file, got true")
	}
}

func TestMarkFileProcessed_ThenIsFileProcessed_ReturnsTrue(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	filename := "ticket-2026-02.pdf"
	if err := s.MarkFileProcessed(uid, filename, date(2026, 2, 1)); err != nil {
		t.Fatalf("MarkFileProcessed: %v", err)
	}
	got, err := s.IsFileProcessed(uid, filename)
	if err != nil {
		t.Fatalf("IsFileProcessed: %v", err)
	}
	if !got {
		t.Error("expected true after marking as processed, got false")
	}
}

func TestMarkFileProcessed_Idempotent(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	filename := "ticket-2026-03.pdf"
	if err := s.MarkFileProcessed(uid, filename, date(2026, 3, 1)); err != nil {
		t.Fatalf("first MarkFileProcessed: %v", err)
	}
	if err := s.MarkFileProcessed(uid, filename, date(2026, 3, 2)); err != nil {
		t.Fatalf("second MarkFileProcessed (idempotent): %v", err)
	}
}

// ---------- UpsertPriceRecordBatch ----------

func TestUpsertPriceRecordBatch_AllEntriesCommitted(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	entries := []models.PriceRecordEntry{
		{Name: "LECHE ENTERA", Record: models.PriceRecord{Date: date(2026, 1, 1), Price: 0.89, Store: "Mercadona"}},
		{Name: "PAN INTEGRAL", Record: models.PriceRecord{Date: date(2026, 1, 1), Price: 1.25, Store: "Mercadona"}},
		{Name: "YOGUR NATURAL", Record: models.PriceRecord{Date: date(2026, 1, 1), Price: 0.35, Store: "Mercadona"}},
	}
	if err := s.UpsertPriceRecordBatch(uid, entries); err != nil {
		t.Fatalf("UpsertPriceRecordBatch: %v", err)
	}
	results, err := s.SearchProducts(uid, "")
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("want 3 products, got %d", len(results))
	}
}

func TestUpsertPriceRecordBatch_EmptySlice_NoOp(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	if err := s.UpsertPriceRecordBatch(uid, []models.PriceRecordEntry{}); err != nil {
		t.Fatalf("UpsertPriceRecordBatch with empty slice: %v", err)
	}
	results, err := s.SearchProducts(uid, "")
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("want 0 products after no-op batch, got %d", len(results))
	}
}

func TestUpsertPriceRecordBatch_IdempotentProduct(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	e1 := models.PriceRecordEntry{Name: "LECHE ENTERA", Record: models.PriceRecord{Date: date(2026, 1, 1), Price: 0.89, Store: "Mercadona"}}
	e2 := models.PriceRecordEntry{Name: "LECHE ENTERA", Record: models.PriceRecord{Date: date(2026, 2, 1), Price: 0.92, Store: "Mercadona"}}
	if err := s.UpsertPriceRecordBatch(uid, []models.PriceRecordEntry{e1}); err != nil {
		t.Fatalf("first batch: %v", err)
	}
	if err := s.UpsertPriceRecordBatch(uid, []models.PriceRecordEntry{e2}); err != nil {
		t.Fatalf("second batch: %v", err)
	}
	p, err := s.GetProductByID(uid, "leche-entera")
	if err != nil {
		t.Fatalf("GetProductByID: %v", err)
	}
	if p == nil {
		t.Fatal("expected product, got nil")
	}
	if len(p.PriceHistory) != 2 {
		t.Errorf("want 2 price records, got %d", len(p.PriceHistory))
	}
}

func TestUpsertPriceRecordBatch_PriceAndDatePreserved(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	want := models.PriceRecord{Date: date(2026, 3, 15), Price: 2.49, Store: "Mercadona"}
	entries := []models.PriceRecordEntry{{Name: "ACEITE GIRASOL", Record: want}}
	if err := s.UpsertPriceRecordBatch(uid, entries); err != nil {
		t.Fatalf("UpsertPriceRecordBatch: %v", err)
	}
	p, err := s.GetProductByID(uid, "aceite-girasol")
	if err != nil {
		t.Fatalf("GetProductByID: %v", err)
	}
	if p == nil || len(p.PriceHistory) == 0 {
		t.Fatal("expected product with price history")
	}
	got := p.PriceHistory[0]
	if got.Price != want.Price {
		t.Errorf("Price: want %.2f, got %.2f", want.Price, got.Price)
	}
	if !got.Date.Equal(want.Date) {
		t.Errorf("Date: want %s, got %s", want.Date, got.Date)
	}
	if got.Store != want.Store {
		t.Errorf("Store: want %q, got %q", want.Store, got.Store)
	}
}

// ---------- GetProductsWithoutImage ----------

func TestGetProductsWithoutImage_ReturnsOnlyUnimaged(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	rec := models.PriceRecord{Date: date(2025, 1, 1), Price: 1.00, Store: "Mercadona"}
	if err := s.UpsertPriceRecord(uid, "LECHE ENTERA", rec); err != nil {
		t.Fatalf("upsert leche: %v", err)
	}
	if err := s.UpsertPriceRecord(uid, "PAN MOLDE", rec); err != nil {
		t.Fatalf("upsert pan: %v", err)
	}
	if err := s.UpdateProductImageURL("leche-entera", "https://img/leche.jpg"); err != nil {
		t.Fatalf("update image: %v", err)
	}
	got, err := s.GetProductsWithoutImage()
	if err != nil {
		t.Fatalf("GetProductsWithoutImage: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 product without image, got %d", len(got))
	}
	if got[0].ID != "pan-molde" {
		t.Errorf("expected pan-molde, got %q", got[0].ID)
	}
}

// ---------- GetMostPurchased ----------

func TestGetMostPurchased_RankedByPurchaseCount(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	leche := models.Product{
		Name: "LECHE ENTERA",
		PriceHistory: []models.PriceRecord{
			{Date: date(2025, 1, 1), Price: 0.79, Store: "Mercadona"},
			{Date: date(2025, 6, 1), Price: 0.85, Store: "Mercadona"},
			{Date: date(2026, 1, 1), Price: 0.89, Store: "Mercadona"},
		},
	}
	pan := models.Product{
		Name: "PAN INTEGRAL",
		PriceHistory: []models.PriceRecord{
			{Date: date(2025, 1, 1), Price: 1.25, Store: "Mercadona"},
		},
	}
	insertProductForUser(t, s, uid, leche)
	insertProductForUser(t, s, uid, pan)
	got, err := s.GetMostPurchased(uid, 10)
	if err != nil {
		t.Fatalf("GetMostPurchased: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 results, got %d", len(got))
	}
	if got[0].ID != "leche-entera" {
		t.Errorf("first: want %q (3 purchases), got %q", "leche-entera", got[0].ID)
	}
	if got[0].PurchaseCount != 3 {
		t.Errorf("leche PurchaseCount: want 3, got %d", got[0].PurchaseCount)
	}
}

func TestGetMostPurchased_EmptyDB_ReturnsEmptySlice(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	got, err := s.GetMostPurchased(uid, 10)
	if err != nil {
		t.Fatalf("GetMostPurchased: %v", err)
	}
	if got == nil {
		t.Error("want empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("want 0 results, got %d", len(got))
	}
}

// ---------- GetBiggestPriceIncreases ----------

func TestGetBiggestPriceIncreases_RankedByPercent(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	aceite := models.Product{
		Name: "ACEITE OLIVA",
		PriceHistory: []models.PriceRecord{
			{Date: date(2025, 1, 1), Price: 2.00, Store: "Mercadona"},
			{Date: date(2026, 1, 1), Price: 4.00, Store: "Mercadona"},
		},
	}
	leche := models.Product{
		Name: "LECHE ENTERA",
		PriceHistory: []models.PriceRecord{
			{Date: date(2025, 1, 1), Price: 0.80, Store: "Mercadona"},
			{Date: date(2026, 1, 1), Price: 0.90, Store: "Mercadona"},
		},
	}
	insertProductForUser(t, s, uid, aceite)
	insertProductForUser(t, s, uid, leche)
	got, err := s.GetBiggestPriceIncreases(uid, 10)
	if err != nil {
		t.Fatalf("GetBiggestPriceIncreases: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].ID != "aceite-oliva" {
		t.Errorf("first: want %q (+100%%), got %q", "aceite-oliva", got[0].ID)
	}
	if got[0].IncreasePercent != 100.0 {
		t.Errorf("aceite-oliva IncreasePercent: want 100, got %f", got[0].IncreasePercent)
	}
}

func TestGetBiggestPriceIncreases_ExcludesDecreases(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	p := models.Product{
		Name: "PAN INTEGRAL",
		PriceHistory: []models.PriceRecord{
			{Date: date(2025, 1, 1), Price: 2.00, Store: "Mercadona"},
			{Date: date(2026, 1, 1), Price: 1.50, Store: "Mercadona"},
		},
	}
	insertProductForUser(t, s, uid, p)
	got, err := s.GetBiggestPriceIncreases(uid, 10)
	if err != nil {
		t.Fatalf("GetBiggestPriceIncreases: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want 0 (decrease excluded), got %d", len(got))
	}
}

func TestGetBiggestPriceIncreases_ExcludesSingleRecord(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	p := models.Product{
		Name: "UN SOLO PRECIO",
		PriceHistory: []models.PriceRecord{
			{Date: date(2025, 1, 1), Price: 1.00, Store: "Mercadona"},
		},
	}
	insertProductForUser(t, s, uid, p)
	got, err := s.GetBiggestPriceIncreases(uid, 10)
	if err != nil {
		t.Fatalf("GetBiggestPriceIncreases: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want 0 (single record excluded), got %d", len(got))
	}
}

func TestGetBiggestPriceIncreases_FieldsPopulated(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	p := models.Product{
		Name: "YOGUR NATURAL",
		PriceHistory: []models.PriceRecord{
			{Date: date(2025, 1, 1), Price: 0.40, Store: "Mercadona"},
			{Date: date(2026, 1, 1), Price: 0.60, Store: "Mercadona"},
		},
	}
	insertProductForUser(t, s, uid, p)
	got, err := s.GetBiggestPriceIncreases(uid, 10)
	if err != nil {
		t.Fatalf("GetBiggestPriceIncreases: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
	r := got[0]
	if r.FirstPrice != 0.40 {
		t.Errorf("FirstPrice: want 0.40, got %f", r.FirstPrice)
	}
	if r.CurrentPrice != 0.60 {
		t.Errorf("CurrentPrice: want 0.60, got %f", r.CurrentPrice)
	}
	if r.IncreasePercent != 50.0 {
		t.Errorf("IncreasePercent: want 50.0, got %f", r.IncreasePercent)
	}
}

// ---------- Household ----------

func createNamedUser(t *testing.T, s *store.PostgresStore, username string) int64 {
	t.Helper()
	id, err := s.CreateUser(username, "", "$2a$12$fakehashfortesting000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("createNamedUser %q: %v", username, err)
	}
	return id
}

func TestCreateHousehold_AssignsOwner(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	hid, err := s.CreateHousehold(uid)
	if err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}
	if hid == 0 {
		t.Fatal("expected non-zero household ID")
	}
	members, err := s.GetHouseholdMembers(uid)
	if err != nil {
		t.Fatalf("GetHouseholdMembers: %v", err)
	}
	if len(members) != 1 || members[0].ID != uid {
		t.Errorf("expected owner in household, got %+v", members)
	}
}

func TestGetHouseholdMembers_NoHousehold_ReturnsNil(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	members, err := s.GetHouseholdMembers(uid)
	if err != nil {
		t.Fatalf("GetHouseholdMembers: %v", err)
	}
	if members != nil {
		t.Errorf("expected nil when user has no household, got %+v", members)
	}
}

func TestAddUserToHousehold_BothMembersVisible(t *testing.T) {
	s := newTestStore(t)
	uid1 := createNamedUser(t, s, "alice")
	uid2 := createNamedUser(t, s, "bob")
	hid, err := s.CreateHousehold(uid1)
	if err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}
	if err := s.AddUserToHousehold(uid2, hid); err != nil {
		t.Fatalf("AddUserToHousehold: %v", err)
	}
	members, err := s.GetHouseholdMembers(uid1)
	if err != nil {
		t.Fatalf("GetHouseholdMembers: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
}

func TestRemoveUserFromHousehold_LastMemberDeletesHousehold(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	if _, err := s.CreateHousehold(uid); err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}
	if err := s.RemoveUserFromHousehold(uid); err != nil {
		t.Fatalf("RemoveUserFromHousehold: %v", err)
	}
	members, err := s.GetHouseholdMembers(uid)
	if err != nil {
		t.Fatalf("GetHouseholdMembers: %v", err)
	}
	if members != nil {
		t.Errorf("expected nil after leaving household, got %+v", members)
	}
}

func TestHousehold_SharedSearchProducts(t *testing.T) {
	s := newTestStore(t)
	uid1 := createNamedUser(t, s, "alice")
	uid2 := createNamedUser(t, s, "bob")
	hid, _ := s.CreateHousehold(uid1)
	_ = s.AddUserToHousehold(uid2, hid)
	rec := models.PriceRecord{Date: date(2026, 1, 1), Price: 1.00, Store: "Mercadona"}
	if err := s.UpsertPriceRecord(uid1, "LECHE ENTERA", rec); err != nil {
		t.Fatalf("UpsertPriceRecord: %v", err)
	}
	results, err := s.SearchProducts(uid2, "")
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected bob to see alice's product, got %d results", len(results))
	}
}

func TestCreateHouseholdInvitation_ReturnsToken(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	token, err := s.CreateHouseholdInvitation(uid)
	if err != nil {
		t.Fatalf("CreateHouseholdInvitation: %v", err)
	}
	if len(token) != 64 {
		t.Errorf("expected 64-char hex token, got %d chars", len(token))
	}
}

func TestCreateHouseholdInvitation_InvalidatesPrevious(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)

	first, err := s.CreateHouseholdInvitation(uid)
	if err != nil {
		t.Fatalf("first CreateHouseholdInvitation: %v", err)
	}

	second, err := s.CreateHouseholdInvitation(uid)
	if err != nil {
		t.Fatalf("second CreateHouseholdInvitation: %v", err)
	}

	if first == second {
		t.Fatal("expected different tokens on each call")
	}

	// The first token must no longer exist.
	inv, err := s.GetHouseholdInvitation(first)
	if err != nil {
		t.Fatalf("GetHouseholdInvitation (first): %v", err)
	}
	if inv != nil {
		t.Error("first invitation should have been invalidated")
	}

	// The second token must still be valid.
	inv2, err := s.GetHouseholdInvitation(second)
	if err != nil {
		t.Fatalf("GetHouseholdInvitation (second): %v", err)
	}
	if inv2 == nil {
		t.Error("second invitation should still be active")
	}
}

func TestGetHouseholdInvitation_ReturnsInvitation(t *testing.T) {
	s := newTestStore(t)
	uid := createTestUser(t, s)
	token, _ := s.CreateHouseholdInvitation(uid)
	inv, err := s.GetHouseholdInvitation(token)
	if err != nil {
		t.Fatalf("GetHouseholdInvitation: %v", err)
	}
	if inv == nil {
		t.Fatal("expected invitation, got nil")
	}
	if inv.InviterID != uid {
		t.Errorf("InviterID: want %d, got %d", uid, inv.InviterID)
	}
	if inv.ExpiresAt.IsZero() {
		t.Error("ExpiresAt must not be zero")
	}
}

func TestGetHouseholdInvitation_UnknownToken_ReturnsNil(t *testing.T) {
	s := newTestStore(t)
	inv, err := s.GetHouseholdInvitation("nonexistenttoken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv != nil {
		t.Errorf("expected nil for unknown token, got %+v", inv)
	}
}

// ---------- CreateUser / GetUserByID ----------

func TestCreateUser_WithEmail_StoresEmail(t *testing.T) {
	s := newTestStore(t)
	id, err := s.CreateUser("emailuser", "user@example.com", "$2a$12$fakehashfortesting000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	u, err := s.GetUserByID(id)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if u == nil {
		t.Fatal("expected user, got nil")
	}
	if u.Email != "user@example.com" {
		t.Errorf("Email: want %q, got %q", "user@example.com", u.Email)
	}
}

func TestGetUserByID_UnknownID_ReturnsNil(t *testing.T) {
	s := newTestStore(t)
	u, err := s.GetUserByID(9999999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u != nil {
		t.Errorf("expected nil for unknown ID, got %+v", u)
	}
}

func TestUpdateUserPassword_PasswordHashChanges(t *testing.T) {
	s := newTestStore(t)
	id, err := s.CreateUser("pwduser", "", "$2a$12$fakehashfortesting000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	newHash := "$2a$12$newhashfortesting0000000000000000000000000000000000000"
	if err := s.UpdateUserPassword(id, newHash); err != nil {
		t.Fatalf("UpdateUserPassword: %v", err)
	}
	u, err := s.GetUserByID(id)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if u.PasswordHash != newHash {
		t.Errorf("PasswordHash: want %q, got %q", newHash, u.PasswordHash)
	}
}
