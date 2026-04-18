package models

import "time"

// User represents an application user account.
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email,omitempty"`
	PasswordHash string    `json:"-"` // never serialised to JSON
	CreatedAt    time.Time `json:"createdAt"`
	IsAdmin      bool      `json:"isAdmin"`
}

// PriceRecord represents a single price observation for a product,
// typically extracted from a digital receipt/ticket.
type PriceRecord struct {
	RecordID int64     `json:"recordId,omitempty"` // DB primary key; 0 for seed/anonymous records
	Date     time.Time `json:"date"`
	Price    float64   `json:"price"`
	Store    string    `json:"store,omitempty"`
}

// Product represents a grocery item with its price history.
type Product struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Category       string        `json:"category,omitempty"`
	ImageURL       string        `json:"imageUrl,omitempty"`
	ImageURLLocked bool          `json:"imageUrlLocked"`
	CurrentPrice   float64       `json:"currentPrice"`
	PriceHistory   []PriceRecord `json:"priceHistory"`
}

// SearchResult is a lightweight version of Product returned in search listings.
type SearchResult struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Category         string  `json:"category,omitempty"`
	ImageURL         string  `json:"imageUrl,omitempty"`
	CurrentPrice     float64 `json:"currentPrice"`
	MinPrice         float64 `json:"minPrice"`
	MaxPrice         float64 `json:"maxPrice"`
	LastPurchaseDate string  `json:"lastPurchaseDate,omitempty"`
}

// PriceRecordEntry is the unit of work for batch price-record persistence.
// It pairs a product name with the price observation to record.
type PriceRecordEntry struct {
	Name   string
	Record PriceRecord
}

// MostPurchasedProduct is a row in the "most purchased products" analytics ranking.
// PurchaseCount reflects the total number of price records (i.e. ticket lines) for the product.
type MostPurchasedProduct struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	ImageURL      string  `json:"imageUrl,omitempty"`
	PurchaseCount int     `json:"purchaseCount"`
	CurrentPrice  float64 `json:"currentPrice"`
}

// PriceIncreaseProduct is a row in the "highest price increase" analytics ranking.
// IncreasePercent is ((currentPrice - firstPrice) / firstPrice) * 100.
type PriceIncreaseProduct struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	ImageURL        string  `json:"imageUrl,omitempty"`
	FirstPrice      float64 `json:"firstPrice"`
	CurrentPrice    float64 `json:"currentPrice"`
	IncreasePercent float64 `json:"increasePercent"`
}

// BasketProductInflation is the per-product contribution inside a basket audit.
type BasketProductInflation struct {
	ProductID        string  `json:"productId"`
	ProductName      string  `json:"productName"`
	ImageURL         string  `json:"imageUrl,omitempty"`
	FirstPrice       float64 `json:"firstPrice"`
	CurrentPrice     float64 `json:"currentPrice"`
	InflationPercent float64 `json:"inflationPercent"`
}

// BasketInflationPoint represents the weighted price inflation for a single ticket.
// InflationPercent = (sum_paid - sum_first_price) / sum_first_price * 100.
// Products appearing for the first time contribute 0% (their first price equals current).
type BasketInflationPoint struct {
	Date             string                   `json:"date"`
	InflationPercent float64                  `json:"inflationPercent"`
	ProductsCount    int                      `json:"productsCount"`
	Products         []BasketProductInflation `json:"products"`
}

// AnalyticsResult is the top-level response body for GET /api/analytics.
type AnalyticsResult struct {
	MostPurchased    []MostPurchasedProduct `json:"mostPurchased"`
	BiggestIncreases []PriceIncreaseProduct `json:"biggestIncreases"`
	BasketInflation  []BasketInflationPoint `json:"basketInflation"`
}

// IPCResult is the response body for GET /api/ipc?from=<year>.
// AccumulatedRate is the compound interannual inflation for Catalonia from
// FromYear to the most recent available year, expressed as a decimal
// (e.g. 0.0537 means +5.37%).
type IPCResult struct {
	FromYear        int     `json:"from_year"`
	ToYear          int     `json:"to_year"`
	AccumulatedRate float64 `json:"accumulated_rate"`
}

// Household represents a shared grocery group (e.g. people living together).
// Members share ticket imports and purchase analytics.
type Household struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
}

// HouseholdInvitation is a time-limited token that allows another user to join
// a household. Tokens expire after 24 hours.
type HouseholdInvitation struct {
	Token       string    `json:"-"`
	HouseholdID int64     `json:"-"`
	InviterID   int64     `json:"-"`
	ExpiresAt   time.Time `json:"-"`
}

// EmailAccount stores the IMAP credentials for automatic receipt ingestion.
// The password is stored encrypted; callers must decrypt before use.
type EmailAccount struct {
	ID                int64     `json:"id"`
	UserID            int64     `json:"userId"`
	EmailAddress      string    `json:"emailAddress"`
	EncryptedPassword string    `json:"-"` // never serialised to JSON
	IMAPHost          string    `json:"imapHost"`
	IMAPPort          int       `json:"imapPort"`
	LastUIDSeen       uint32    `json:"lastUidSeen"`
	CreatedAt         time.Time `json:"createdAt"`
}
