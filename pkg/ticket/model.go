// Package ticket provides types and logic for parsing digital grocery receipts
// (tiquets) and importing the extracted price data into the store.
package ticket

import "time"

// Ticket represents the header-level data extracted from a single receipt.
type Ticket struct {
	// Store is the normalised store name, e.g. "Mercadona".
	Store string
	// Date is the purchase date parsed from the receipt header.
	Date time.Time
	// InvoiceNumber is the simplified invoice reference, e.g. "4144-017-284404".
	InvoiceNumber string
	// Lines contains every product line extracted from the receipt body.
	Lines []TicketLine
}

// TicketLine represents a single product entry within a receipt.
type TicketLine struct {
	// Name is the raw product name as it appears on the receipt (uppercase).
	Name string
	// UnitPrice is the price per unit in euros, as charged on the receipt.
	// For unit products (qty=1 or qty>1) this is the per-unit price.
	// For weight products (sold by kg) this is the total line amount paid,
	// because the meaningful comparison across purchases is the actual spend
	// (the price/kg varies with weight and is not stored).
	UnitPrice float64
	// Quantity is the number of units purchased (always ≥ 1).
	// For weight products this is always 1.
	Quantity int
}
