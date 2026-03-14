package ticket

import (
	"fmt"
	"io"

	"basket-cost/pkg/models"
)

// TicketStore is the subset of store.Store required by the Importer.
// Using a narrow interface keeps the ticket package decoupled from the full
// store package and makes testing easier.
type TicketStore interface {
	// UpsertPriceRecordBatch persists all entries scoped to userID inside a
	// single transaction. Either every entry is committed or none is.
	UpsertPriceRecordBatch(userID int64, entries []models.PriceRecordEntry) error
}

// ImportResult summarises the outcome of a single ticket import.
type ImportResult struct {
	// InvoiceNumber is the receipt identifier.
	InvoiceNumber string
	// LinesImported is the number of product lines successfully persisted.
	LinesImported int
}

// Importer orchestrates PDF extraction → parsing → persistence.
type Importer struct {
	extractor PDFExtractor
	parser    Parser
	store     TicketStore
}

// NewImporter wires up the three collaborators.
func NewImporter(extractor PDFExtractor, parser Parser, store TicketStore) *Importer {
	return &Importer{
		extractor: extractor,
		parser:    parser,
		store:     store,
	}
}

// Import reads a PDF from r, parses it as a Mercadona receipt, and persists
// all product lines atomically inside a single transaction scoped to userID.
// If any line fails to persist the entire ticket is rolled back.
// r must implement io.ReaderAt; use bytes.NewReader for in-memory data.
func (imp *Importer) Import(userID int64, r io.ReaderAt, size int64) (*ImportResult, error) {
	text, err := imp.extractor.Extract(r, size)
	if err != nil {
		return nil, fmt.Errorf("extract pdf text: %w", err)
	}

	t, err := imp.parser.Parse(text)
	if err != nil {
		return nil, fmt.Errorf("parse receipt: %w", err)
	}

	entries := make([]models.PriceRecordEntry, len(t.Lines))
	for i, line := range t.Lines {
		entries[i] = models.PriceRecordEntry{
			Name: line.Name,
			Record: models.PriceRecord{
				Date:  t.Date,
				Price: line.UnitPrice,
				Store: t.Store,
			},
		}
	}

	if err := imp.store.UpsertPriceRecordBatch(userID, entries); err != nil {
		return nil, fmt.Errorf("persist ticket %s: %w", t.InvoiceNumber, err)
	}

	return &ImportResult{
		InvoiceNumber: t.InvoiceNumber,
		LinesImported: len(entries),
	}, nil
}
