// Package store provides the data access layer backed by SQLite.
package store

import (
	"basket-cost/internal/models"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Store is the interface the HTTP handlers depend on.
// Both the real SQLite implementation and test fakes satisfy it.
type Store interface {
	// CreateUser inserts a new user and returns its generated ID.
	CreateUser(username, email, passwordHash string) (int64, error)
	// GetUserByUsername returns the user with the given username, or nil if not found.
	GetUserByUsername(username string) (*models.User, error)
	// GetUserByID returns the user with the given ID, or nil if not found.
	GetUserByID(id int64) (*models.User, error)
	// UpdateUserPassword replaces the stored password hash for userID.
	UpdateUserPassword(userID int64, passwordHash string) error

	// SearchProducts returns products whose price records belong to userID.
	// An empty query returns all matching products.
	SearchProducts(userID int64, query string) ([]models.SearchResult, error)
	// GetProductByID returns the product and its price history scoped to the
	// household of userID. Pass userID=0 for anonymous (seed) access.
	GetProductByID(userID int64, id string) (*models.Product, error)
	InsertProduct(p models.Product) error
	// UpsertPriceRecord ensures the named product exists (creating it if needed)
	// and appends a new price record scoped to userID.
	UpsertPriceRecord(userID int64, name string, record models.PriceRecord) error
	// DeletePriceRecord deletes the price record with the given DB ID.
	// Returns an error if the record does not exist or does not belong to userID's household.
	DeletePriceRecord(recordID int64, userID int64) error
	// UpsertPriceRecordBatch persists all (name, record) pairs inside a single
	// transaction scoped to userID. Either every pair is committed or none is.
	UpsertPriceRecordBatch(userID int64, entries []models.PriceRecordEntry) error
	// UpdateProductImageURL sets the image URL for the product with the given ID.
	// Used by the enricher; does not set the locked flag.
	UpdateProductImageURL(id, imageURL string) error
	// SetProductImageURLManual sets a manually provided image URL and marks the
	// product as locked so the enricher will not overwrite it in future runs.
	SetProductImageURLManual(id, imageURL string) error
	// GetProductsWithoutImage returns a lightweight list of products that have
	// no image URL set yet and are not manually locked.
	GetProductsWithoutImage() ([]models.SearchResult, error)
	// IsFileProcessed returns true when filename has already been imported by userID.
	IsFileProcessed(userID int64, filename string) (bool, error)
	// MarkFileProcessed records filename as successfully imported by userID.
	MarkFileProcessed(userID int64, filename string, importedAt time.Time) error
	// GetMostPurchased returns the top N products by number of price records for userID.
	GetMostPurchased(userID int64, limit int) ([]models.MostPurchasedProduct, error)
	// GetBiggestPriceIncreases returns the top N products by percentage price increase
	// for userID. Only products with at least 2 records and a positive increase are included.
	GetBiggestPriceIncreases(userID int64, limit int) ([]models.PriceIncreaseProduct, error)

	// RevokeToken stores a JWT JTI in the revoked-tokens list so that the
	// token is rejected even before its natural expiry.
	RevokeToken(jti string, expiresAt time.Time) error
	// IsTokenRevoked returns true if the given JTI has been revoked.
	IsTokenRevoked(jti string) (bool, error)
	// CleanupExpiredTokens removes revoked-token entries whose expiry has
	// passed. Safe to call periodically from a background goroutine.
	CleanupExpiredTokens() error

	// GetAccumulatedIPC returns the compound interannual IPC for Catalonia from
	// fromYear up to and including the most recent available year in the database.
	// Returns the accumulated rate as a decimal (e.g. 0.0537 for +5.37%) and the
	// last year covered. Returns (0, fromYear, nil) if no data is available.
	GetAccumulatedIPC(fromYear int) (rate float64, toYear int, err error)

	// GetHouseholdMembers returns all members of the household userID belongs to.
	// Returns nil if userID has no household.
	GetHouseholdMembers(userID int64) ([]models.User, error)
	// CreateHousehold creates a new household, assigns ownerID to it, and returns the household ID.
	CreateHousehold(ownerID int64) (int64, error)
	// AddUserToHousehold moves userID into householdID, leaving any previous household.
	// If the previous household becomes empty it is deleted automatically.
	AddUserToHousehold(userID, householdID int64) error
	// RemoveUserFromHousehold removes userID from their current household.
	// If the household becomes empty it is deleted (cascade removes invitations).
	RemoveUserFromHousehold(userID int64) error
	// CreateHouseholdInvitation generates a 24-hour invitation token for inviterID.
	// If inviterID has no household, one is created first.
	// Returns the token string.
	CreateHouseholdInvitation(inviterID int64) (string, error)
	// GetHouseholdInvitation returns the invitation for token, or nil if not found.
	GetHouseholdInvitation(token string) (*models.HouseholdInvitation, error)
	// DeleteHouseholdInvitation removes the invitation identified by token.
	DeleteHouseholdInvitation(token string) error
}

// SQLiteStore is the production Store backed by a *sql.DB.
type SQLiteStore struct {
	db *sql.DB
}

// New returns a SQLiteStore wrapping the given database connection.
func New(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

// CreateUser inserts a new user and returns the auto-generated ID.
// Returns an error if the username is already taken (UNIQUE constraint).
func (s *SQLiteStore) CreateUser(username, email, passwordHash string) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO users (username, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		username, email, passwordHash, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("create user %q: %w", username, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get last insert id: %w", err)
	}
	return id, nil
}

// GetUserByUsername returns the user matching username, or nil if not found.
func (s *SQLiteStore) GetUserByUsername(username string) (*models.User, error) {
	var u models.User
	var createdAt string
	err := s.db.QueryRow(
		`SELECT id, username, email, password_hash, created_at FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user %q: %w", username, err)
	}
	u.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse user created_at: %w", err)
	}
	return &u, nil
}

// GetUserByID returns the user with the given ID, or nil if not found.
func (s *SQLiteStore) GetUserByID(id int64) (*models.User, error) {
	var u models.User
	var createdAt string
	err := s.db.QueryRow(
		`SELECT id, username, email, password_hash, created_at FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id %d: %w", id, err)
	}
	u.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse user created_at: %w", err)
	}
	return &u, nil
}

// UpdateUserPassword replaces the stored password hash for the given user.
func (s *SQLiteStore) UpdateUserPassword(userID int64, passwordHash string) error {
	_, err := s.db.Exec(
		`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID,
	)
	if err != nil {
		return fmt.Errorf("update password for user %d: %w", userID, err)
	}
	return nil
}

// InsertProduct inserts a product and all its price records inside a single
// transaction. If the product already exists it is skipped (idempotent seed).
func (s *SQLiteStore) InsertProduct(p models.Product) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec(
		`INSERT OR IGNORE INTO products (id, name, category) VALUES (?, ?, ?)`,
		p.ID, p.Name, p.Category,
	)
	if err != nil {
		return fmt.Errorf("insert product %s: %w", p.ID, err)
	}

	for _, r := range p.PriceHistory {
		_, err = tx.Exec(
			`INSERT INTO price_records (product_id, date, price, store) VALUES (?, ?, ?, ?)`,
			p.ID, r.Date.Format(time.DateOnly), r.Price, r.Store,
		)
		if err != nil {
			return fmt.Errorf("insert price record for product %s: %w", p.ID, err)
		}
	}

	return tx.Commit()
}

// householdUserIDs returns the IDs of all users sharing a household with userID,
// including userID itself. If userID==0 (anonymous), returns [0] for IS NULL queries.
// If userID has no household, returns [userID].
func (s *SQLiteStore) householdUserIDs(userID int64) ([]int64, error) {
	if userID == 0 {
		return []int64{0}, nil
	}
	var householdID sql.NullInt64
	err := s.db.QueryRow(`SELECT household_id FROM users WHERE id = ?`, userID).Scan(&householdID)
	if err != nil || !householdID.Valid {
		return []int64{userID}, nil //nolint:nilerr
	}
	rows, err := s.db.Query(`SELECT id FROM users WHERE household_id = ?`, householdID.Int64)
	if err != nil {
		return nil, fmt.Errorf("get household members: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan member id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate household members: %w", err)
	}
	if len(ids) == 0 {
		return []int64{userID}, nil
	}
	return ids, nil
}

// userIDsInClause returns the SQL condition fragment and its argument list for
// filtering price_records by a set of user IDs.
// For the anonymous case (ids=[0]), returns "user_id IS NULL" with no args.
func userIDsInClause(ids []int64) (string, []any) {
	if len(ids) == 1 && ids[0] == 0 {
		return "user_id IS NULL", nil
	}
	ph := strings.Repeat("?,", len(ids))
	ph = ph[:len(ph)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return "user_id IN (" + ph + ")", args
}

// repeatArgs repeats args n times into a single flat slice.
// Returns nil when args is empty.
func repeatArgs(args []any, n int) []any {
	if len(args) == 0 {
		return nil
	}
	out := make([]any, 0, len(args)*n)
	for range n {
		out = append(out, args...)
	}
	return out
}

// SearchProducts returns products that have at least one price record belonging
// to userID's household and whose name contains query (case-insensitive).
// An empty query returns all matching products. Results are ordered by the most
// recent purchase date, descending.
// When userID == 0, returns products with user_id IS NULL (anonymous/seed data).
func (s *SQLiteStore) SearchProducts(userID int64, query string) ([]models.SearchResult, error) {
	ids, err := s.householdUserIDs(userID)
	if err != nil {
		return nil, fmt.Errorf("resolve household: %w", err)
	}
	clause, baseArgs := userIDsInClause(ids)
	// clause appears 5 times (4 subqueries + 1 EXISTS).
	args := repeatArgs(baseArgs, 5)

	baseSQL := `
		SELECT
			p.id,
			p.name,
			p.category,
			p.image_url,
			(SELECT price FROM price_records WHERE product_id = p.id AND ` + clause + ` ORDER BY date DESC LIMIT 1) AS current_price,
			(SELECT MIN(price) FROM price_records WHERE product_id = p.id AND ` + clause + `)                        AS min_price,
			(SELECT MAX(price) FROM price_records WHERE product_id = p.id AND ` + clause + `)                        AS max_price,
			(SELECT MAX(date)  FROM price_records WHERE product_id = p.id AND ` + clause + `)                        AS last_date
		FROM products p
		WHERE EXISTS (SELECT 1 FROM price_records WHERE product_id = p.id AND ` + clause + `)
	`

	var rows *sql.Rows
	if strings.TrimSpace(query) == "" {
		rows, err = s.db.Query(baseSQL+" ORDER BY last_date DESC, p.name", args...)
	} else {
		rows, err = s.db.Query(
			baseSQL+` AND p.name LIKE ? ORDER BY last_date DESC, p.name`,
			append(args, "%"+query+"%")...,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("search products: %w", err)
	}
	defer rows.Close()

	var results []models.SearchResult
	for rows.Next() {
		var r models.SearchResult
		var category, imageURL, lastDate sql.NullString
		var currentPrice, minPrice, maxPrice sql.NullFloat64
		if err := rows.Scan(&r.ID, &r.Name, &category, &imageURL, &currentPrice, &minPrice, &maxPrice, &lastDate); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		r.Category = category.String
		r.ImageURL = imageURL.String
		r.CurrentPrice = currentPrice.Float64
		r.MinPrice = minPrice.Float64
		r.MaxPrice = maxPrice.Float64
		r.LastPurchaseDate = lastDate.String
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search results: %w", err)
	}

	if results == nil {
		results = []models.SearchResult{}
	}
	return results, nil
}

// reNonAlphanumeric matches any character that is not a lowercase letter or digit.
var reNonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// nullableUserID converts userID=0 to a SQL NULL so that unauthenticated
// requests (seed data, tests without JWT) satisfy the nullable FK constraint
// on price_records.user_id and processed_files.user_id.
func nullableUserID(userID int64) any {
	if userID == 0 {
		return nil
	}
	return userID
}

// slugify converts a product name to a stable, URL-safe ID.
// Example: "LECHE ENTERA HACENDADO 1L" → "leche-entera-hacendado-1l"
func slugify(name string) string {
	lower := strings.ToLower(name)
	slug := reNonAlphanumeric.ReplaceAllString(lower, "-")
	slug = strings.Trim(slug, "-")
	return slug
}

// UpsertPriceRecord ensures a product with the given name exists in the
// database (creating it with a generated ID if necessary) and then inserts a
// new price record scoped to userID.
func (s *SQLiteStore) UpsertPriceRecord(userID int64, name string, record models.PriceRecord) error {
	id := slugify(name)

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Insert product if it does not exist yet.
	_, err = tx.Exec(
		`INSERT OR IGNORE INTO products (id, name, category) VALUES (?, ?, ?)`,
		id, name, "",
	)
	if err != nil {
		return fmt.Errorf("upsert product %q: %w", name, err)
	}

	// Insert the price record scoped to userID (NULL when userID == 0).
	_, err = tx.Exec(
		`INSERT INTO price_records (product_id, date, price, store, user_id) VALUES (?, ?, ?, ?, ?)`,
		id, record.Date.Format(time.DateOnly), record.Price, record.Store, nullableUserID(userID),
	)
	if err != nil {
		return fmt.Errorf("insert price record for product %q: %w", name, err)
	}

	return tx.Commit()
}

// UpsertPriceRecordBatch persists all entries inside a single transaction
// scoped to userID. Either every entry is committed or none is.
// Calling it with an empty slice is a no-op that returns nil.
func (s *SQLiteStore) UpsertPriceRecordBatch(userID int64, entries []models.PriceRecordEntry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	for _, e := range entries {
		id := slugify(e.Name)

		_, err = tx.Exec(
			`INSERT OR IGNORE INTO products (id, name, category) VALUES (?, ?, ?)`,
			id, e.Name, "",
		)
		if err != nil {
			return fmt.Errorf("upsert product %q: %w", e.Name, err)
		}

		_, err = tx.Exec(
			`INSERT INTO price_records (product_id, date, price, store, user_id) VALUES (?, ?, ?, ?, ?)`,
			id, e.Record.Date.Format(time.DateOnly), e.Record.Price, e.Record.Store, nullableUserID(userID),
		)
		if err != nil {
			return fmt.Errorf("insert price record for product %q: %w", e.Name, err)
		}
	}

	return tx.Commit()
}

// GetProductByID returns the product with its price history scoped to the
// household of userID. Pass userID=0 for anonymous (seed) access.
// Returns nil if no product with that ID exists.
func (s *SQLiteStore) GetProductByID(userID int64, id string) (*models.Product, error) {
	row := s.db.QueryRow(
		`SELECT id, name, category, image_url, image_url_locked FROM products WHERE id = ?`, id,
	)

	var p models.Product
	var category, imageURL sql.NullString
	var locked int
	if err := row.Scan(&p.ID, &p.Name, &category, &imageURL, &locked); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get product %s: %w", id, err)
	}
	p.Category = category.String
	p.ImageURL = imageURL.String
	p.ImageURLLocked = locked == 1

	memberIDs, err := s.householdUserIDs(userID)
	if err != nil {
		return nil, fmt.Errorf("resolve household: %w", err)
	}
	clause, clauseArgs := userIDsInClause(memberIDs)
	queryArgs := append([]any{id}, clauseArgs...)

	rows, err := s.db.Query(
		`SELECT id, date, price, store FROM price_records WHERE product_id = ? AND `+clause+` ORDER BY date ASC`,
		queryArgs...,
	)
	if err != nil {
		return nil, fmt.Errorf("get price records for product %s: %w", id, err)
	}
	defer rows.Close()

	for rows.Next() {
		var rec models.PriceRecord
		var dateStr string
		if err := rows.Scan(&rec.RecordID, &dateStr, &rec.Price, &rec.Store); err != nil {
			return nil, fmt.Errorf("scan price record: %w", err)
		}
		rec.Date, err = time.Parse(time.DateOnly, dateStr)
		if err != nil {
			return nil, fmt.Errorf("parse date %q: %w", dateStr, err)
		}
		p.PriceHistory = append(p.PriceHistory, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate price records: %w", err)
	}

	// Derive CurrentPrice from the most recent record.
	if len(p.PriceHistory) > 0 {
		p.CurrentPrice = p.PriceHistory[len(p.PriceHistory)-1].Price
	}

	return &p, nil
}

// UpdateProductImageURL sets the image_url for the product with the given ID.
// It is a no-op if no product with that ID exists.
// It does NOT set the locked flag — use SetProductImageURLManual for that.
func (s *SQLiteStore) UpdateProductImageURL(id, imageURL string) error {
	_, err := s.db.Exec(
		`UPDATE products SET image_url = ? WHERE id = ?`, imageURL, id,
	)
	if err != nil {
		return fmt.Errorf("update image_url for product %s: %w", id, err)
	}
	return nil
}

// SetProductImageURLManual sets a user-provided image URL and marks the product
// as locked (image_url_locked = 1) so the enricher will skip it in future runs.
func (s *SQLiteStore) SetProductImageURLManual(id, imageURL string) error {
	_, err := s.db.Exec(
		`UPDATE products SET image_url = ?, image_url_locked = 1 WHERE id = ?`, imageURL, id,
	)
	if err != nil {
		return fmt.Errorf("set manual image_url for product %s: %w", id, err)
	}
	return nil
}

// GetProductsWithoutImage returns a minimal projection of every product whose
// image_url column is NULL or empty and that is not manually locked.
// Only the ID and Name fields are populated.
func (s *SQLiteStore) GetProductsWithoutImage() ([]models.SearchResult, error) {
	rows, err := s.db.Query(
		`SELECT id, name FROM products WHERE (image_url IS NULL OR image_url = '') AND image_url_locked = 0 ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("get products without image: %w", err)
	}
	defer rows.Close()

	var results []models.SearchResult
	for rows.Next() {
		var r models.SearchResult
		if err := rows.Scan(&r.ID, &r.Name); err != nil {
			return nil, fmt.Errorf("scan product without image: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate products without image: %w", err)
	}
	if results == nil {
		results = []models.SearchResult{}
	}
	return results, nil
}

// IsFileProcessed returns true when filename has already been imported by any
// member of userID's household. This prevents duplicate imports within a household.
// When userID == 0, checks for records where user_id IS NULL (anonymous/seed data).
func (s *SQLiteStore) IsFileProcessed(userID int64, filename string) (bool, error) {
	ids, err := s.householdUserIDs(userID)
	if err != nil {
		return false, fmt.Errorf("resolve household: %w", err)
	}
	clause, clauseArgs := userIDsInClause(ids)
	queryArgs := append([]any{filename}, clauseArgs...)
	var count int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM processed_files WHERE filename = ? AND `+clause, queryArgs...,
	).Scan(&count); err != nil {
		return false, fmt.Errorf("check processed file %q: %w", filename, err)
	}
	return count > 0, nil
}

// MarkFileProcessed records filename as successfully imported by userID.
// Calling it again for the same (filename, userID) pair is idempotent.
// When userID == 0, stores NULL in user_id (anonymous/seed data).
func (s *SQLiteStore) MarkFileProcessed(userID int64, filename string, importedAt time.Time) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO processed_files (filename, imported_at, user_id) VALUES (?, ?, ?)`,
		filename, importedAt.UTC().Format(time.RFC3339), nullableUserID(userID),
	)
	if err != nil {
		return fmt.Errorf("mark file processed %q: %w", filename, err)
	}
	return nil
}

// GetMostPurchased returns the top `limit` products ranked by total number of
// price records belonging to userID's household.
// When userID == 0, returns products with user_id IS NULL (anonymous/seed data).
func (s *SQLiteStore) GetMostPurchased(userID int64, limit int) ([]models.MostPurchasedProduct, error) {
	ids, err := s.householdUserIDs(userID)
	if err != nil {
		return nil, fmt.Errorf("resolve household: %w", err)
	}
	clause, baseArgs := userIDsInClause(ids)
	// clause appears twice (subquery + JOIN)
	joinArgs := append(repeatArgs(baseArgs, 2), limit)

	q := `
		SELECT
			p.id,
			p.name,
			COALESCE(p.image_url, '') AS image_url,
			COUNT(pr.id)              AS purchase_count,
			COALESCE((SELECT price FROM price_records WHERE product_id = p.id AND ` + clause + ` ORDER BY date DESC LIMIT 1), 0) AS current_price
		FROM products p
		JOIN price_records pr ON pr.product_id = p.id AND pr.` + clause + `
		GROUP BY p.id
		ORDER BY purchase_count DESC, p.name ASC
		LIMIT ?
	`
	var rows *sql.Rows
	rows, err = s.db.Query(q, joinArgs...)
	if err != nil {
		return nil, fmt.Errorf("get most purchased: %w", err)
	}
	defer rows.Close()

	var results []models.MostPurchasedProduct
	for rows.Next() {
		var r models.MostPurchasedProduct
		if err := rows.Scan(&r.ID, &r.Name, &r.ImageURL, &r.PurchaseCount, &r.CurrentPrice); err != nil {
			return nil, fmt.Errorf("scan most purchased: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate most purchased: %w", err)
	}
	if results == nil {
		results = []models.MostPurchasedProduct{}
	}
	return results, nil
}

// ---------- Household methods ----------

// GetHouseholdMembers returns all users in the same household as userID,
// ordered by ID. Returns nil (not an error) if userID has no household.
func (s *SQLiteStore) GetHouseholdMembers(userID int64) ([]models.User, error) {
	var householdID sql.NullInt64
	if err := s.db.QueryRow(`SELECT household_id FROM users WHERE id = ?`, userID).Scan(&householdID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get household id for user %d: %w", userID, err)
	}
	if !householdID.Valid {
		return nil, nil
	}
	rows, err := s.db.Query(
		`SELECT id, username, email, created_at FROM users WHERE household_id = ? ORDER BY id ASC`,
		householdID.Int64,
	)
	if err != nil {
		return nil, fmt.Errorf("get household members: %w", err)
	}
	defer rows.Close()
	var members []models.User
	for rows.Next() {
		var u models.User
		var createdAt string
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &createdAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		u.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}
		members = append(members, u)
	}
	return members, rows.Err()
}

// CreateHousehold creates a new household, assigns ownerID to it, and returns
// the household ID. Uses a transaction so both operations are atomic.
func (s *SQLiteStore) CreateHousehold(ownerID int64) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	res, err := tx.Exec(`INSERT INTO households (created_at) VALUES (?)`, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("create household: %w", err)
	}
	hid, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	if _, err = tx.Exec(`UPDATE users SET household_id = ? WHERE id = ?`, hid, ownerID); err != nil {
		return 0, fmt.Errorf("assign household to user %d: %w", ownerID, err)
	}
	return hid, tx.Commit()
}

// AddUserToHousehold moves userID into the given household.
// If userID was in a different household and that household becomes empty,
// it is deleted (cascade removes its invitations).
func (s *SQLiteStore) AddUserToHousehold(userID, householdID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var oldHouseholdID sql.NullInt64
	_ = tx.QueryRow(`SELECT household_id FROM users WHERE id = ?`, userID).Scan(&oldHouseholdID)

	if _, err = tx.Exec(`UPDATE users SET household_id = ? WHERE id = ?`, householdID, userID); err != nil {
		return fmt.Errorf("add user %d to household %d: %w", userID, householdID, err)
	}
	if oldHouseholdID.Valid && oldHouseholdID.Int64 != householdID {
		var count int
		_ = tx.QueryRow(`SELECT COUNT(*) FROM users WHERE household_id = ?`, oldHouseholdID.Int64).Scan(&count)
		if count == 0 {
			if _, err = tx.Exec(`DELETE FROM households WHERE id = ?`, oldHouseholdID.Int64); err != nil {
				return fmt.Errorf("delete empty household %d: %w", oldHouseholdID.Int64, err)
			}
		}
	}
	return tx.Commit()
}

// RemoveUserFromHousehold removes userID from their current household.
// If the household becomes empty it is deleted (cascade removes invitations).
// No-op if userID has no household.
func (s *SQLiteStore) RemoveUserFromHousehold(userID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var householdID sql.NullInt64
	if err := tx.QueryRow(`SELECT household_id FROM users WHERE id = ?`, userID).Scan(&householdID); err != nil {
		return fmt.Errorf("get household for user %d: %w", userID, err)
	}
	if _, err = tx.Exec(`UPDATE users SET household_id = NULL WHERE id = ?`, userID); err != nil {
		return fmt.Errorf("remove user %d from household: %w", userID, err)
	}
	if householdID.Valid {
		var count int
		_ = tx.QueryRow(`SELECT COUNT(*) FROM users WHERE household_id = ?`, householdID.Int64).Scan(&count)
		if count == 0 {
			if _, err = tx.Exec(`DELETE FROM households WHERE id = ?`, householdID.Int64); err != nil {
				return fmt.Errorf("delete empty household %d: %w", householdID.Int64, err)
			}
		}
	}
	return tx.Commit()
}

// CreateHouseholdInvitation generates a 24-hour invitation token for inviterID.
// If inviterID has no household, a new one is created first.
// Returns the token string.
func (s *SQLiteStore) CreateHouseholdInvitation(inviterID int64) (string, error) {
	var householdID sql.NullInt64
	if err := s.db.QueryRow(`SELECT household_id FROM users WHERE id = ?`, inviterID).Scan(&householdID); err != nil {
		return "", fmt.Errorf("get household for user %d: %w", inviterID, err)
	}
	var hid int64
	if !householdID.Valid {
		newHID, err := s.CreateHousehold(inviterID)
		if err != nil {
			return "", err
		}
		hid = newHID
	} else {
		hid = householdID.Int64
	}
	token, err := generateToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	_, err = s.db.Exec(
		`INSERT INTO household_invitations (token, household_id, inviter_id, expires_at) VALUES (?, ?, ?, ?)`,
		token, hid, inviterID, expiresAt.Format(time.RFC3339),
	)
	if err != nil {
		return "", fmt.Errorf("create invitation: %w", err)
	}
	return token, nil
}

// GetHouseholdInvitation returns the invitation matching token, or nil if not found.
func (s *SQLiteStore) GetHouseholdInvitation(token string) (*models.HouseholdInvitation, error) {
	var inv models.HouseholdInvitation
	var expiresAt string
	err := s.db.QueryRow(
		`SELECT token, household_id, inviter_id, expires_at FROM household_invitations WHERE token = ?`, token,
	).Scan(&inv.Token, &inv.HouseholdID, &inv.InviterID, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get invitation: %w", err)
	}
	inv.ExpiresAt, err = time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("parse expires_at: %w", err)
	}
	return &inv, nil
}

// DeleteHouseholdInvitation removes the invitation identified by token.
func (s *SQLiteStore) DeleteHouseholdInvitation(token string) error {
	_, err := s.db.Exec(`DELETE FROM household_invitations WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("delete invitation: %w", err)
	}
	return nil
}

// DeletePriceRecord deletes the price record with the given ID only when it
// belongs to userID's household. Returns an error if no row was affected.
func (s *SQLiteStore) DeletePriceRecord(recordID int64, userID int64) error {
	ids, err := s.householdUserIDs(userID)
	if err != nil {
		return fmt.Errorf("resolve household: %w", err)
	}
	clause, clauseArgs := userIDsInClause(ids)
	args := append([]any{recordID}, clauseArgs...)
	result, err := s.db.Exec(
		`DELETE FROM price_records WHERE id = ? AND `+clause,
		args...,
	)
	if err != nil {
		return fmt.Errorf("delete price record %d: %w", recordID, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("price record %d not found or not owned by user", recordID)
	}
	return nil
}

// RevokeToken stores the given JTI so that ValidateToken callers can check
// whether a token has been invalidated (e.g., after logout).
func (s *SQLiteStore) RevokeToken(jti string, expiresAt time.Time) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO revoked_tokens (jti, expires_at) VALUES (?, ?)`,
		jti, expiresAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("revoke token %q: %w", jti, err)
	}
	return nil
}

// IsTokenRevoked returns true if the given JTI exists in the revocation table.
func (s *SQLiteStore) IsTokenRevoked(jti string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM revoked_tokens WHERE jti = ?`, jti,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check revoked token %q: %w", jti, err)
	}
	return count > 0, nil
}

// CleanupExpiredTokens deletes revocation entries whose expiry has already
// passed. Intended to be called periodically from a background goroutine.
func (s *SQLiteStore) CleanupExpiredTokens() error {
	_, err := s.db.Exec(
		`DELETE FROM revoked_tokens WHERE expires_at < ?`,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("cleanup expired tokens: %w", err)
	}
	return nil
}

// generateToken returns a cryptographically random 32-byte hex string (64 chars).
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GetBiggestPriceIncreases returns the top `limit` products by percentage price
// increase for userID's household, from first to latest record.
// Only products with ≥2 records and a strictly positive increase are included.
// When userID == 0, returns results for records with user_id IS NULL.
func (s *SQLiteStore) GetBiggestPriceIncreases(userID int64, limit int) ([]models.PriceIncreaseProduct, error) {
	ids, err := s.householdUserIDs(userID)
	if err != nil {
		return nil, fmt.Errorf("resolve household: %w", err)
	}
	clause, baseArgs := userIDsInClause(ids)
	// clause appears 5 times
	queryArgs := append(repeatArgs(baseArgs, 5), limit)

	q := `
		SELECT
			p.id,
			p.name,
			COALESCE(p.image_url, '') AS image_url,
			first_rec.price           AS first_price,
			last_rec.price            AS current_price,
			ROUND(((last_rec.price - first_rec.price) / first_rec.price) * 100, 2) AS increase_pct
		FROM products p
		JOIN (
			SELECT product_id, price
			FROM price_records
			WHERE ` + clause + `
			  AND (product_id, date) IN (
				SELECT product_id, MIN(date) FROM price_records WHERE ` + clause + ` GROUP BY product_id
			)
		) first_rec ON first_rec.product_id = p.id
		JOIN (
			SELECT product_id, price
			FROM price_records
			WHERE ` + clause + `
			  AND (product_id, date) IN (
				SELECT product_id, MAX(date) FROM price_records WHERE ` + clause + ` GROUP BY product_id
			)
		) last_rec ON last_rec.product_id = p.id
		WHERE last_rec.price > first_rec.price
		  AND (SELECT COUNT(*) FROM price_records WHERE product_id = p.id AND ` + clause + `) >= 2
		ORDER BY increase_pct DESC, p.name ASC
		LIMIT ?
	`
	var rows *sql.Rows
	rows, err = s.db.Query(q, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("get biggest price increases: %w", err)
	}
	defer rows.Close()

	var results []models.PriceIncreaseProduct
	for rows.Next() {
		var r models.PriceIncreaseProduct
		if err := rows.Scan(&r.ID, &r.Name, &r.ImageURL, &r.FirstPrice, &r.CurrentPrice, &r.IncreasePercent); err != nil {
			return nil, fmt.Errorf("scan biggest price increases: %w", err)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate biggest price increases: %w", err)
	}
	if results == nil {
		results = []models.PriceIncreaseProduct{}
	}
	return results, nil
}

// GetAccumulatedIPC computes the compound interannual IPC for Catalonia from
// fromYear up to the latest available year in the ipc_rates table.
// Formula: (1+r₁)×(1+r₂)×…×(1+rN) - 1.
// Returns (0, fromYear, nil) when no data is found for the requested range.
func (s *SQLiteStore) GetAccumulatedIPC(fromYear int) (rate float64, toYear int, err error) {
	rows, err := s.db.Query(
		`SELECT year, rate FROM ipc_rates WHERE year >= ? ORDER BY year ASC`,
		fromYear,
	)
	if err != nil {
		return 0, fromYear, fmt.Errorf("query ipc_rates: %w", err)
	}
	defer rows.Close()

	accumulated := 1.0
	toYear = fromYear
	found := false
	for rows.Next() {
		var year int
		var r float64
		if err := rows.Scan(&year, &r); err != nil {
			return 0, fromYear, fmt.Errorf("scan ipc_rates: %w", err)
		}
		accumulated *= (1 + r)
		toYear = year
		found = true
	}
	if err := rows.Err(); err != nil {
		return 0, fromYear, fmt.Errorf("iterate ipc_rates: %w", err)
	}
	if !found {
		return 0, fromYear, nil
	}
	return accumulated - 1, toYear, nil
}
