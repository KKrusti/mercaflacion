package models

import "time"

// User represents an application user account.
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email,omitempty"`
	PasswordHash string    `json:"-"` // never serialised to JSON
	CreatedAt    time.Time `json:"createdAt"`
}

// PriceRecord represents a single price observation for a product,
// typically extracted from a digital receipt/ticket.
type PriceRecord struct {
	Date  time.Time `json:"date"`
	Price float64   `json:"price"`
	Store string    `json:"store,omitempty"`
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

// AnalyticsResult is the top-level response body for GET /api/analytics.
type AnalyticsResult struct {
	MostPurchased    []MostPurchasedProduct `json:"mostPurchased"`
	BiggestIncreases []PriceIncreaseProduct `json:"biggestIncreases"`
}
