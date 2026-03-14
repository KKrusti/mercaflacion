// Package store provides the data access layer backed by PostgreSQL.
package store

import (
	"basket-cost/pkg/models"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// Store is the interface the HTTP handlers depend on.
// Both the real PostgreSQL implementation and test fakes satisfy it.
type Store interface {
	CreateUser(username, email, passwordHash string) (int64, error)
	GetUserByUsername(username string) (*models.User, error)
	GetUserByID(id int64) (*models.User, error)
	UpdateUserPassword(userID int64, passwordHash string) error

	SearchProducts(userID int64, query string) ([]models.SearchResult, error)
	GetProductByID(userID int64, id string) (*models.Product, error)
	InsertProduct(p models.Product) error
	UpsertPriceRecord(userID int64, name string, record models.PriceRecord) error
	DeletePriceRecord(recordID int64, userID int64) error
	UpsertPriceRecordBatch(userID int64, entries []models.PriceRecordEntry) error
	UpdateProductImageURL(id, imageURL string) error
	SetProductImageURLManual(id, imageURL string) error
	GetProductsWithoutImage() ([]models.SearchResult, error)
	IsFileProcessed(userID int64, filename string) (bool, error)
	MarkFileProcessed(userID int64, filename string, importedAt time.Time) error
	GetMostPurchased(userID int64, limit int) ([]models.MostPurchasedProduct, error)
	GetBiggestPriceIncreases(userID int64, limit int) ([]models.PriceIncreaseProduct, error)

	RevokeToken(jti string, expiresAt time.Time) error
	IsTokenRevoked(jti string) (bool, error)
	CleanupExpiredTokens() error

	GetAccumulatedIPC(fromYear int) (rate float64, toYear int, err error)

	GetHouseholdMembers(userID int64) ([]models.User, error)
	CreateHousehold(ownerID int64) (int64, error)
	AddUserToHousehold(userID, householdID int64) error
	RemoveUserFromHousehold(userID int64) error
	CreateHouseholdInvitation(inviterID int64) (string, error)
	GetHouseholdInvitation(token string) (*models.HouseholdInvitation, error)
	DeleteHouseholdInvitation(token string) error
}

// PostgresStore is the production Store backed by a *sql.DB (PostgreSQL).
type PostgresStore struct {
	db *sql.DB
}

// New returns a PostgresStore wrapping the given database connection.
func New(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

// isDuplicateKey returns true when err is a PostgreSQL unique-constraint violation.
func isDuplicateKey(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// CreateUser inserts a new user and returns the generated ID.
// Returns an error containing "UNIQUE" when the username is already taken.
func (s *PostgresStore) CreateUser(username, email, passwordHash string) (int64, error) {
	var id int64
	err := s.db.QueryRow(
		`INSERT INTO users (username, email, password_hash, created_at)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		username, email, passwordHash, time.Now().UTC(),
	).Scan(&id)
	if err != nil {
		if isDuplicateKey(err) {
			return 0, fmt.Errorf("create user %q: UNIQUE constraint violation", username)
		}
		return 0, fmt.Errorf("create user %q: %w", username, err)
	}
	return id, nil
}

// GetUserByUsername returns the user matching username, or nil if not found.
func (s *PostgresStore) GetUserByUsername(username string) (*models.User, error) {
	var u models.User
	err := s.db.QueryRow(
		`SELECT id, username, email, password_hash, created_at, is_admin FROM users WHERE username = $1`, username,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.IsAdmin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user %q: %w", username, err)
	}
	return &u, nil
}

// GetUserByID returns the user with the given ID, or nil if not found.
func (s *PostgresStore) GetUserByID(id int64) (*models.User, error) {
	var u models.User
	err := s.db.QueryRow(
		`SELECT id, username, email, password_hash, created_at, is_admin FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.IsAdmin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id %d: %w", id, err)
	}
	return &u, nil
}

// UpdateUserPassword replaces the stored password hash for the given user.
func (s *PostgresStore) UpdateUserPassword(userID int64, passwordHash string) error {
	_, err := s.db.Exec(
		`UPDATE users SET password_hash = $1 WHERE id = $2`, passwordHash, userID,
	)
	if err != nil {
		return fmt.Errorf("update password for user %d: %w", userID, err)
	}
	return nil
}

// InsertProduct inserts a product and all its price records inside a single
// transaction. If the product already exists it is skipped (idempotent seed).
func (s *PostgresStore) InsertProduct(p models.Product) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec(
		`INSERT INTO products (id, name, category) VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO NOTHING`,
		p.ID, p.Name, p.Category,
	)
	if err != nil {
		return fmt.Errorf("insert product %s: %w", p.ID, err)
	}

	for _, r := range p.PriceHistory {
		_, err = tx.Exec(
			`INSERT INTO price_records (product_id, date, price, store) VALUES ($1, $2, $3, $4)`,
			p.ID, r.Date.Format(time.DateOnly), r.Price, r.Store,
		)
		if err != nil {
			return fmt.Errorf("insert price record for product %s: %w", p.ID, err)
		}
	}
	return tx.Commit()
}

// householdUserIDs returns the IDs of all users sharing a household with userID,
// including userID itself. For anonymous (userID=0) returns [0] for IS NULL queries.
func (s *PostgresStore) householdUserIDs(userID int64) ([]int64, error) {
	if userID == 0 {
		return []int64{0}, nil
	}
	var householdID sql.NullInt64
	err := s.db.QueryRow(`SELECT household_id FROM users WHERE id = $1`, userID).Scan(&householdID)
	if err != nil || !householdID.Valid {
		return []int64{userID}, nil //nolint:nilerr
	}
	rows, err := s.db.Query(`SELECT id FROM users WHERE household_id = $1`, householdID.Int64)
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

// userIDsInClause builds a SQL fragment for filtering price_records by user IDs.
// startIdx is the first $N placeholder index to use.
// Returns the SQL fragment, the args for those placeholders, and the next free index.
//
// For anonymous access (ids=[0]): returns "user_id IS NULL", nil, startIdx.
// PostgreSQL supports reusing $N in multiple places within the same query, so
// the returned clause can appear multiple times without repeating the args.
func userIDsInClause(ids []int64, startIdx int) (clause string, args []any, nextIdx int) {
	if len(ids) == 1 && ids[0] == 0 {
		return "user_id IS NULL", nil, startIdx
	}
	placeholders := make([]string, len(ids))
	args = make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", startIdx+i)
		args[i] = id
	}
	return "user_id IN (" + strings.Join(placeholders, ",") + ")", args, startIdx + len(ids)
}

// nullableUserID converts userID=0 to a SQL NULL for unauthenticated requests.
func nullableUserID(userID int64) any {
	if userID == 0 {
		return nil
	}
	return userID
}

// reNonAlphanumeric matches any character that is not a lowercase letter or digit.
var reNonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// slugify converts a product name to a stable, URL-safe ID.
func slugify(name string) string {
	lower := strings.ToLower(name)
	slug := reNonAlphanumeric.ReplaceAllString(lower, "-")
	slug = strings.Trim(slug, "-")
	return slug
}

// SearchProducts returns products whose price records belong to userID's household.
// An empty query returns all. Results ordered by most-recently-purchased, desc.
// When userID == 0, returns products with user_id IS NULL (anonymous/seed data).
func (s *PostgresStore) SearchProducts(userID int64, query string) ([]models.SearchResult, error) {
	ids, err := s.householdUserIDs(userID)
	if err != nil {
		return nil, fmt.Errorf("resolve household: %w", err)
	}
	// In PostgreSQL, $N placeholders can be reused — the same clause appears
	// multiple times in the query but args are passed only once.
	clause, baseArgs, nextIdx := userIDsInClause(ids, 1)

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
		rows, err = s.db.Query(baseSQL+" ORDER BY last_date DESC, p.name", baseArgs...)
	} else {
		likeArg := fmt.Sprintf("$%d", nextIdx)
		rows, err = s.db.Query(
			baseSQL+` AND p.name ILIKE `+likeArg+` ORDER BY last_date DESC, p.name`,
			append(baseArgs, "%"+query+"%")...,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("search products: %w", err)
	}
	defer rows.Close()

	var results []models.SearchResult
	for rows.Next() {
		var r models.SearchResult
		var currentPrice, minPrice, maxPrice sql.NullFloat64
		var lastDate sql.NullString
		if err := rows.Scan(&r.ID, &r.Name, &r.Category, &r.ImageURL, &currentPrice, &minPrice, &maxPrice, &lastDate); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
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

// UpsertPriceRecord ensures the named product exists and appends a new price
// record scoped to userID.
func (s *PostgresStore) UpsertPriceRecord(userID int64, name string, record models.PriceRecord) error {
	id := slugify(name)

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec(
		`INSERT INTO products (id, name, category) VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO NOTHING`,
		id, name, "",
	)
	if err != nil {
		return fmt.Errorf("upsert product %q: %w", name, err)
	}

	_, err = tx.Exec(
		`INSERT INTO price_records (product_id, date, price, store, user_id) VALUES ($1, $2, $3, $4, $5)`,
		id, record.Date.Format(time.DateOnly), record.Price, record.Store, nullableUserID(userID),
	)
	if err != nil {
		return fmt.Errorf("insert price record for product %q: %w", name, err)
	}
	return tx.Commit()
}

// UpsertPriceRecordBatch persists all entries inside a single transaction.
func (s *PostgresStore) UpsertPriceRecordBatch(userID int64, entries []models.PriceRecordEntry) error {
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
			`INSERT INTO products (id, name, category) VALUES ($1, $2, $3)
			 ON CONFLICT (id) DO NOTHING`,
			id, e.Name, "",
		)
		if err != nil {
			return fmt.Errorf("upsert product %q: %w", e.Name, err)
		}
		_, err = tx.Exec(
			`INSERT INTO price_records (product_id, date, price, store, user_id) VALUES ($1, $2, $3, $4, $5)`,
			id, e.Record.Date.Format(time.DateOnly), e.Record.Price, e.Record.Store, nullableUserID(userID),
		)
		if err != nil {
			return fmt.Errorf("insert price record for product %q: %w", e.Name, err)
		}
	}
	return tx.Commit()
}

// GetProductByID returns the product with its price history scoped to userID's household.
// Returns nil if no product with that ID exists.
func (s *PostgresStore) GetProductByID(userID int64, id string) (*models.Product, error) {
	row := s.db.QueryRow(
		`SELECT id, name, category, image_url, image_url_locked FROM products WHERE id = $1`, id,
	)
	var p models.Product
	if err := row.Scan(&p.ID, &p.Name, &p.Category, &p.ImageURL, &p.ImageURLLocked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get product %s: %w", id, err)
	}

	memberIDs, err := s.householdUserIDs(userID)
	if err != nil {
		return nil, fmt.Errorf("resolve household: %w", err)
	}
	clause, clauseArgs, _ := userIDsInClause(memberIDs, 2) // $1 is product id
	queryArgs := append([]any{id}, clauseArgs...)

	rows, err := s.db.Query(
		`SELECT id, date, price, store FROM price_records WHERE product_id = $1 AND `+clause+` ORDER BY date ASC`,
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
	if len(p.PriceHistory) > 0 {
		p.CurrentPrice = p.PriceHistory[len(p.PriceHistory)-1].Price
	}
	return &p, nil
}

// UpdateProductImageURL sets the image_url for the product. Does not set the locked flag.
func (s *PostgresStore) UpdateProductImageURL(id, imageURL string) error {
	_, err := s.db.Exec(
		`UPDATE products SET image_url = $1 WHERE id = $2`, imageURL, id,
	)
	if err != nil {
		return fmt.Errorf("update image_url for product %s: %w", id, err)
	}
	return nil
}

// SetProductImageURLManual sets a user-provided image URL and locks the product.
func (s *PostgresStore) SetProductImageURLManual(id, imageURL string) error {
	_, err := s.db.Exec(
		`UPDATE products SET image_url = $1, image_url_locked = TRUE WHERE id = $2`, imageURL, id,
	)
	if err != nil {
		return fmt.Errorf("set manual image_url for product %s: %w", id, err)
	}
	return nil
}

// GetProductsWithoutImage returns products with no image URL and not manually locked.
func (s *PostgresStore) GetProductsWithoutImage() ([]models.SearchResult, error) {
	rows, err := s.db.Query(
		`SELECT id, name FROM products WHERE (image_url IS NULL OR image_url = '') AND image_url_locked = FALSE ORDER BY name`,
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
// member of userID's household.
func (s *PostgresStore) IsFileProcessed(userID int64, filename string) (bool, error) {
	ids, err := s.householdUserIDs(userID)
	if err != nil {
		return false, fmt.Errorf("resolve household: %w", err)
	}
	clause, clauseArgs, _ := userIDsInClause(ids, 2) // $1 is filename
	queryArgs := append([]any{filename}, clauseArgs...)
	var count int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM processed_files WHERE filename = $1 AND `+clause, queryArgs...,
	).Scan(&count); err != nil {
		return false, fmt.Errorf("check processed file %q: %w", filename, err)
	}
	return count > 0, nil
}

// MarkFileProcessed records filename as successfully imported by userID (idempotent).
func (s *PostgresStore) MarkFileProcessed(userID int64, filename string, importedAt time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO processed_files (filename, imported_at, user_id) VALUES ($1, $2, $3)
		 ON CONFLICT (filename, COALESCE(user_id, 0)) DO NOTHING`,
		filename, importedAt.UTC(), nullableUserID(userID),
	)
	if err != nil {
		return fmt.Errorf("mark file processed %q: %w", filename, err)
	}
	return nil
}

// GetMostPurchased returns the top `limit` products ranked by number of price records.
func (s *PostgresStore) GetMostPurchased(userID int64, limit int) ([]models.MostPurchasedProduct, error) {
	ids, err := s.householdUserIDs(userID)
	if err != nil {
		return nil, fmt.Errorf("resolve household: %w", err)
	}
	clause, baseArgs, nextIdx := userIDsInClause(ids, 1)
	limitArg := fmt.Sprintf("$%d", nextIdx)

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
		LIMIT ` + limitArg

	rows, err := s.db.Query(q, append(baseArgs, limit)...)
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

// GetBiggestPriceIncreases returns the top `limit` products by percentage increase.
func (s *PostgresStore) GetBiggestPriceIncreases(userID int64, limit int) ([]models.PriceIncreaseProduct, error) {
	ids, err := s.householdUserIDs(userID)
	if err != nil {
		return nil, fmt.Errorf("resolve household: %w", err)
	}
	clause, baseArgs, nextIdx := userIDsInClause(ids, 1)
	limitArg := fmt.Sprintf("$%d", nextIdx)

	q := `
		SELECT
			p.id,
			p.name,
			COALESCE(p.image_url, '') AS image_url,
			first_rec.price           AS first_price,
			last_rec.price            AS current_price,
			ROUND(((last_rec.price - first_rec.price) / first_rec.price * 100)::NUMERIC, 2)::DOUBLE PRECISION AS increase_pct
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
		LIMIT ` + limitArg

	rows, err := s.db.Query(q, append(baseArgs, limit)...)
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

// GetHouseholdMembers returns all users in the same household as userID.
func (s *PostgresStore) GetHouseholdMembers(userID int64) ([]models.User, error) {
	var householdID sql.NullInt64
	if err := s.db.QueryRow(`SELECT household_id FROM users WHERE id = $1`, userID).Scan(&householdID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get household id for user %d: %w", userID, err)
	}
	if !householdID.Valid {
		return nil, nil
	}
	rows, err := s.db.Query(
		`SELECT id, username, email, created_at FROM users WHERE household_id = $1 ORDER BY id ASC`,
		householdID.Int64,
	)
	if err != nil {
		return nil, fmt.Errorf("get household members: %w", err)
	}
	defer rows.Close()
	var members []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, u)
	}
	return members, rows.Err()
}

// CreateHousehold creates a new household, assigns ownerID to it, and returns the household ID.
func (s *PostgresStore) CreateHousehold(ownerID int64) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var hid int64
	if err := tx.QueryRow(`INSERT INTO households (created_at) VALUES ($1) RETURNING id`, time.Now().UTC()).Scan(&hid); err != nil {
		return 0, fmt.Errorf("create household: %w", err)
	}
	if _, err = tx.Exec(`UPDATE users SET household_id = $1 WHERE id = $2`, hid, ownerID); err != nil {
		return 0, fmt.Errorf("assign household to user %d: %w", ownerID, err)
	}
	return hid, tx.Commit()
}

// AddUserToHousehold moves userID into the given household, deleting the old one if empty.
func (s *PostgresStore) AddUserToHousehold(userID, householdID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var oldHouseholdID sql.NullInt64
	_ = tx.QueryRow(`SELECT household_id FROM users WHERE id = $1`, userID).Scan(&oldHouseholdID)

	if _, err = tx.Exec(`UPDATE users SET household_id = $1 WHERE id = $2`, householdID, userID); err != nil {
		return fmt.Errorf("add user %d to household %d: %w", userID, householdID, err)
	}
	if oldHouseholdID.Valid && oldHouseholdID.Int64 != householdID {
		var count int
		_ = tx.QueryRow(`SELECT COUNT(*) FROM users WHERE household_id = $1`, oldHouseholdID.Int64).Scan(&count)
		if count == 0 {
			if _, err = tx.Exec(`DELETE FROM households WHERE id = $1`, oldHouseholdID.Int64); err != nil {
				return fmt.Errorf("delete empty household %d: %w", oldHouseholdID.Int64, err)
			}
		}
	}
	return tx.Commit()
}

// RemoveUserFromHousehold removes userID from their household, deleting it if empty.
func (s *PostgresStore) RemoveUserFromHousehold(userID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var householdID sql.NullInt64
	if err := tx.QueryRow(`SELECT household_id FROM users WHERE id = $1`, userID).Scan(&householdID); err != nil {
		return fmt.Errorf("get household for user %d: %w", userID, err)
	}
	if _, err = tx.Exec(`UPDATE users SET household_id = NULL WHERE id = $1`, userID); err != nil {
		return fmt.Errorf("remove user %d from household: %w", userID, err)
	}
	if householdID.Valid {
		var count int
		_ = tx.QueryRow(`SELECT COUNT(*) FROM users WHERE household_id = $1`, householdID.Int64).Scan(&count)
		if count == 0 {
			if _, err = tx.Exec(`DELETE FROM households WHERE id = $1`, householdID.Int64); err != nil {
				return fmt.Errorf("delete empty household %d: %w", householdID.Int64, err)
			}
		}
	}
	return tx.Commit()
}

// CreateHouseholdInvitation generates a 24-hour token. Creates a household if needed.
func (s *PostgresStore) CreateHouseholdInvitation(inviterID int64) (string, error) {
	var householdID sql.NullInt64
	if err := s.db.QueryRow(`SELECT household_id FROM users WHERE id = $1`, inviterID).Scan(&householdID); err != nil {
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
	// Invalidate any existing invitations for this household so there is only one active token.
	if _, err := s.db.Exec(`DELETE FROM household_invitations WHERE household_id = $1`, hid); err != nil {
		return "", fmt.Errorf("invalidate old invitations: %w", err)
	}
	token, err := generateToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	_, err = s.db.Exec(
		`INSERT INTO household_invitations (token, household_id, inviter_id, expires_at) VALUES ($1, $2, $3, $4)`,
		token, hid, inviterID, expiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("create invitation: %w", err)
	}
	return token, nil
}

// GetHouseholdInvitation returns the invitation for token, or nil if not found.
func (s *PostgresStore) GetHouseholdInvitation(token string) (*models.HouseholdInvitation, error) {
	var inv models.HouseholdInvitation
	err := s.db.QueryRow(
		`SELECT token, household_id, inviter_id, expires_at FROM household_invitations WHERE token = $1`, token,
	).Scan(&inv.Token, &inv.HouseholdID, &inv.InviterID, &inv.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get invitation: %w", err)
	}
	return &inv, nil
}

// DeleteHouseholdInvitation removes the invitation identified by token.
func (s *PostgresStore) DeleteHouseholdInvitation(token string) error {
	_, err := s.db.Exec(`DELETE FROM household_invitations WHERE token = $1`, token)
	if err != nil {
		return fmt.Errorf("delete invitation: %w", err)
	}
	return nil
}

// DeletePriceRecord deletes the price record only when it belongs to userID's household.
func (s *PostgresStore) DeletePriceRecord(recordID int64, userID int64) error {
	ids, err := s.householdUserIDs(userID)
	if err != nil {
		return fmt.Errorf("resolve household: %w", err)
	}
	clause, clauseArgs, _ := userIDsInClause(ids, 2) // $1 is recordID
	args := append([]any{recordID}, clauseArgs...)
	result, err := s.db.Exec(
		`DELETE FROM price_records WHERE id = $1 AND `+clause,
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

// RevokeToken stores the JTI so it is rejected before its natural expiry.
func (s *PostgresStore) RevokeToken(jti string, expiresAt time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO revoked_tokens (jti, expires_at) VALUES ($1, $2)
		 ON CONFLICT (jti) DO NOTHING`,
		jti, expiresAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("revoke token %q: %w", jti, err)
	}
	return nil
}

// IsTokenRevoked returns true if the given JTI exists in the revocation table.
func (s *PostgresStore) IsTokenRevoked(jti string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM revoked_tokens WHERE jti = $1`, jti,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check revoked token %q: %w", jti, err)
	}
	return count > 0, nil
}

// CleanupExpiredTokens deletes revocation entries whose expiry has passed.
func (s *PostgresStore) CleanupExpiredTokens() error {
	_, err := s.db.Exec(
		`DELETE FROM revoked_tokens WHERE expires_at < $1`, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("cleanup expired tokens: %w", err)
	}
	return nil
}

// GetAccumulatedIPC computes the compound interannual IPC from fromYear to latest.
func (s *PostgresStore) GetAccumulatedIPC(fromYear int) (rate float64, toYear int, err error) {
	rows, err := s.db.Query(
		`SELECT year, rate FROM ipc_rates WHERE year >= $1 ORDER BY year ASC`, fromYear,
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

// generateToken returns a cryptographically random 32-byte hex string (64 chars).
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
