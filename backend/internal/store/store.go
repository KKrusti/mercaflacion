// Package store provides the data access layer backed by SQLite.
package store

import (
	"basket-cost/internal/models"
	"database/sql"
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
	GetProductByID(id string) (*models.Product, error)
	InsertProduct(p models.Product) error
	// UpsertPriceRecord ensures the named product exists (creating it if needed)
	// and appends a new price record scoped to userID.
	UpsertPriceRecord(userID int64, name string, record models.PriceRecord) error
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

// SearchProducts returns products that have at least one price record belonging
// to userID and whose name contains query (case-insensitive).
// An empty query returns all matching products. Results are ordered by the most
// recent purchase date for that user, descending.
// When userID == 0, returns products with user_id IS NULL (anonymous/seed data).
func (s *SQLiteStore) SearchProducts(userID int64, query string) ([]models.SearchResult, error) {
	// userIDCond is the SQL fragment used to match the user column. For real
	// users we use "= ?" (with the ID as argument); for anonymous (userID == 0)
	// we use "IS NULL" (no argument needed).
	userIDCond := "= ?"
	if userID == 0 {
		userIDCond = "IS NULL"
	}

	baseSQL := `
		SELECT
			p.id,
			p.name,
			p.category,
			p.image_url,
			(SELECT price FROM price_records WHERE product_id = p.id AND user_id ` + userIDCond + ` ORDER BY date DESC LIMIT 1) AS current_price,
			(SELECT MIN(price) FROM price_records WHERE product_id = p.id AND user_id ` + userIDCond + `)                        AS min_price,
			(SELECT MAX(price) FROM price_records WHERE product_id = p.id AND user_id ` + userIDCond + `)                        AS max_price,
			(SELECT MAX(date)  FROM price_records WHERE product_id = p.id AND user_id ` + userIDCond + `)                        AS last_date
		FROM products p
		WHERE EXISTS (SELECT 1 FROM price_records WHERE product_id = p.id AND user_id ` + userIDCond + `)
	`

	var (
		rows *sql.Rows
		err  error
	)

	var args []any
	if userID != 0 {
		// userID appears 5 times in the base query (4 subqueries + 1 EXISTS).
		args = []any{userID, userID, userID, userID, userID}
	}

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

// GetProductByID returns the full product with its price history, or nil if not found.
func (s *SQLiteStore) GetProductByID(id string) (*models.Product, error) {
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

	rows, err := s.db.Query(
		`SELECT date, price, store FROM price_records WHERE product_id = ? ORDER BY date ASC`,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("get price records for product %s: %w", id, err)
	}
	defer rows.Close()

	for rows.Next() {
		var rec models.PriceRecord
		var dateStr string
		if err := rows.Scan(&dateStr, &rec.Price, &rec.Store); err != nil {
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

// IsFileProcessed returns true when filename has already been imported by userID.
// When userID == 0, checks for records where user_id IS NULL (anonymous/seed data).
func (s *SQLiteStore) IsFileProcessed(userID int64, filename string) (bool, error) {
	var count int
	var err error
	if userID == 0 {
		err = s.db.QueryRow(
			`SELECT COUNT(*) FROM processed_files WHERE filename = ? AND user_id IS NULL`, filename,
		).Scan(&count)
	} else {
		err = s.db.QueryRow(
			`SELECT COUNT(*) FROM processed_files WHERE filename = ? AND user_id = ?`, filename, userID,
		).Scan(&count)
	}
	if err != nil {
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
// price records belonging to userID. Products with no records for that user are excluded.
// When userID == 0, returns products with user_id IS NULL (anonymous/seed data).
func (s *SQLiteStore) GetMostPurchased(userID int64, limit int) ([]models.MostPurchasedProduct, error) {
	userIDCond := "= ?"
	if userID == 0 {
		userIDCond = "IS NULL"
	}

	q := `
		SELECT
			p.id,
			p.name,
			COALESCE(p.image_url, '') AS image_url,
			COUNT(pr.id)              AS purchase_count,
			COALESCE((SELECT price FROM price_records WHERE product_id = p.id AND user_id ` + userIDCond + ` ORDER BY date DESC LIMIT 1), 0) AS current_price
		FROM products p
		JOIN price_records pr ON pr.product_id = p.id AND pr.user_id ` + userIDCond + `
		GROUP BY p.id
		ORDER BY purchase_count DESC, p.name ASC
		LIMIT ?
	`
	var rows *sql.Rows
	var err error
	if userID == 0 {
		rows, err = s.db.Query(q, limit)
	} else {
		rows, err = s.db.Query(q, userID, userID, limit)
	}
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

// GetBiggestPriceIncreases returns the top `limit` products by percentage price
// increase for userID, from first to latest record. Only products with ≥2 records
// for that user and a strictly positive increase are included.
// When userID == 0, returns results for records with user_id IS NULL.
func (s *SQLiteStore) GetBiggestPriceIncreases(userID int64, limit int) ([]models.PriceIncreaseProduct, error) {
	userIDCond := "= ?"
	if userID == 0 {
		userIDCond = "IS NULL"
	}

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
			WHERE user_id ` + userIDCond + `
			  AND (product_id, date) IN (
				SELECT product_id, MIN(date) FROM price_records WHERE user_id ` + userIDCond + ` GROUP BY product_id
			)
		) first_rec ON first_rec.product_id = p.id
		JOIN (
			SELECT product_id, price
			FROM price_records
			WHERE user_id ` + userIDCond + `
			  AND (product_id, date) IN (
				SELECT product_id, MAX(date) FROM price_records WHERE user_id ` + userIDCond + ` GROUP BY product_id
			)
		) last_rec ON last_rec.product_id = p.id
		WHERE last_rec.price > first_rec.price
		  AND (SELECT COUNT(*) FROM price_records WHERE product_id = p.id AND user_id ` + userIDCond + `) >= 2
		ORDER BY increase_pct DESC, p.name ASC
		LIMIT ?
	`
	var rows *sql.Rows
	var err error
	if userID == 0 {
		rows, err = s.db.Query(q, limit)
	} else {
		rows, err = s.db.Query(q, userID, userID, userID, userID, userID, limit)
	}
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
