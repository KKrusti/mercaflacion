package ticket

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Parser is the contract for turning raw receipt text into a structured Ticket.
type Parser interface {
	// Parse converts the plain-text content of a receipt into a Ticket.
	// It returns an error if the text does not match the expected format.
	Parse(text string) (*Ticket, error)
}

// MercadonaParser parses receipts from Mercadona (Caldes de Montbui branch).
// Receipts are written in Catalan.
//
// The ledongthuc/pdf extractor renders each PDF column cell on its own line,
// so a receipt body looks like:
//
//	qty           ← integer (e.g. "1", "3")
//	PRODUCT NAME
//	unit_price    ← e.g. "1,00"
//	line_total    ← present only when qty > 1 (e.g. "3,00")
//
// Weight products have an extra pair of lines between name and unit_price:
//
//	qty
//	PRODUCT NAME
//	0,432 kg      ← weight
//	2,45 €/kg     ← price per kg  ← this is the UnitPrice we store
//	1,06          ← line total (always present for weight products)
type MercadonaParser struct{}

// NewMercadonaParser returns a ready-to-use MercadonaParser.
func NewMercadonaParser() *MercadonaParser {
	return &MercadonaParser{}
}

// Compiled regexes.
var (
	// Date anywhere on a line: "09/02/2026"
	reDateLine = regexp.MustCompile(`(\d{2}/\d{2}/\d{4})`)

	// Invoice line: "FACTURA SIMPLIFICADA: 4144-017-284404"
	reInvoice = regexp.MustCompile(`FACTURA SIMPLIFICADA:\s*(\S+)`)

	// Pure integer quantity: "1", "3", "9" …
	reQty = regexp.MustCompile(`^(\d+)$`)

	// Price in Spanish locale: "1,00", "0,89", "12,50"
	rePrice = regexp.MustCompile(`^(\d+,\d{2})$`)

	// Weight line: "0,432 kg"
	reWeightKg = regexp.MustCompile(`^(\d+,\d+)\s*kg$`)

	// Price-per-kg line: "2,45 €/kg"
	rePricePerKg = regexp.MustCompile(`^(\d+,\d{2})\s*€/kg$`)

	// Footer sentinel — everything from here on is ignored.
	reFooter = regexp.MustCompile(`TOTAL\s*\(€\)`)

	// ── Legacy single-line formats (used by existing unit tests) ──────────────

	// "1   PRODUCT NAME   0,89"
	reUnitSingle = regexp.MustCompile(`^1\s{2,}(.+?)\s{2,}(\d+,\d{2})\s*$`)

	// "3   PRODUCT NAME   0,45   1,35"
	reUnitMulti = regexp.MustCompile(`^(\d+)\s{2,}(.+?)\s{2,}(\d+,\d{2})\s{2,}\d+,\d{2}\s*$`)

	// Weight continuation in single-line mode: "0,354 kg   6,99 €/kg   2,47"
	// Group 1: weight, group 2: price/kg, group 3: line total (amount paid).
	reWeightLineSingle = regexp.MustCompile(`^(\d+,\d+)\s*kg\s+(\d+,\d{2})\s*€/kg\s+(\d+,\d{2})\s*$`)

	// Detect if we are inside a single-line body (column header on one line).
	reColumnHeaderSingle = regexp.MustCompile(`Descripció\s+P\.\s*Unit\s+Import`)

	// Trailing price at end of a line: "1,99" or "12,50 "
	reTrailingPrice = regexp.MustCompile(`\d+,\d{2}\s*$`)
)

// Parse implements Parser for Mercadona receipts.
func (p *MercadonaParser) Parse(text string) (*Ticket, error) {
	lines := splitLines(text)

	t := &Ticket{Store: "Mercadona"}

	// ── Extract header fields ────────────────────────────────────────────────
	for _, line := range lines {
		if t.Date.IsZero() {
			if m := reDateLine.FindStringSubmatch(line); m != nil {
				d, err := time.Parse("02/01/2006", m[1])
				if err == nil {
					t.Date = d
				}
			}
		}
		if t.InvoiceNumber == "" {
			if m := reInvoice.FindStringSubmatch(line); m != nil {
				t.InvoiceNumber = m[1]
			}
		}
		if !t.Date.IsZero() && t.InvoiceNumber != "" {
			break
		}
	}

	if t.Date.IsZero() {
		return nil, fmt.Errorf("could not find date in receipt")
	}

	// ── Detect body format ───────────────────────────────────────────────────
	// If the column header ("Descripció   P. Unit   Import") appears on a
	// single line with spaces, we use the legacy single-line parser.
	// Otherwise we use the multi-line (real PDF) parser.
	for _, line := range lines {
		if reColumnHeaderSingle.MatchString(line) {
			p.parseSingleLineBody(lines, t)
			return t, nil
		}
	}
	p.parseMultiLineBody(lines, t)
	return t, nil
}

// parseMultiLineBody handles the format produced by the ledongthuc/pdf
// extractor where each column cell occupies its own line.
func (p *MercadonaParser) parseMultiLineBody(lines []string, t *Ticket) {
	// States
	const (
		sIdle           = iota // looking for start of body
		sQty                   // in body, looking for a qty integer
		sNameLine              // just consumed qty; expecting product name
		sPrice                 // have qty+name; expecting price or weight
		sWeightPPK             // have weight; expecting price-per-kg
		sWeightTotal           // have ppk; expecting line total — capture it as UnitPrice
		sUnitMultiTotal        // qty>1 unit product: next line is total to discard
	)

	state := sIdle
	var (
		pendingName string
		pendingQty  int
	)

	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)

		if reFooter.MatchString(trimmed) {
			break
		}

		// Detect body start: "Descripció" on its own line (multi-line header).
		if state == sIdle {
			if trimmed == "Descripció" {
				state = sQty // next meaningful lines will be qty then name
				// The header spans three lines: "Descripció", "P. Unit",
				// "Import" — we skip those by staying in sQty and ignoring
				// non-qty content until we see a qty.
			}
			continue
		}

		if trimmed == "" {
			continue
		}

		switch state {
		case sQty:
			// Looking for a qty integer.
			if m := reQty.FindStringSubmatch(trimmed); m != nil {
				qty, err := strconv.Atoi(m[1])
				if err == nil {
					pendingQty = qty
					state = sNameLine
				}
			}
			// "P. Unit" / "Import" header lines — skip silently.

		case sNameLine: // expecting product name
			// Skip "P. Unit" and "Import" header residue.
			if trimmed == "P. Unit" || trimmed == "Import" {
				continue
			}
			// Any non-price, non-qty line is the product name.
			if rePrice.MatchString(trimmed) || reQty.MatchString(trimmed) {
				// Unexpected; reset.
				state = sQty
				pendingQty = 0
				continue
			}
			pendingName = trimmed
			state = sPrice

		case sPrice:
			// Could be: price, weight line, or price-per-kg immediately.
			if reWeightKg.MatchString(trimmed) {
				// Weight product: next line will be €/kg.
				state = sWeightPPK
				continue
			}
			if m := rePrice.FindStringSubmatch(trimmed); m != nil {
				price, err := parsePrice(m[1])
				if err != nil {
					state = sQty
					continue
				}
				if pendingQty == 1 {
					// qty=1 unit product: emit now.
					t.Lines = append(t.Lines, TicketLine{
						Name:      pendingName,
						UnitPrice: price,
						Quantity:  1,
					})
					state = sQty
				} else {
					// qty > 1: this is the unit price; next line is
					// the line total which we discard.
					t.Lines = append(t.Lines, TicketLine{
						Name:      pendingName,
						UnitPrice: price,
						Quantity:  pendingQty,
					})
					state = sUnitMultiTotal
				}
				continue
			}
			// Unexpected content; reset.
			state = sQty

		case sWeightPPK:
			// Expecting the €/kg line. The actual price we store is the
			// line total on the following line, so we just advance state.
			if rePricePerKg.MatchString(trimmed) {
				state = sWeightTotal
			}
			// If we see something unexpected, reset.

		case sWeightTotal:
			// This is the total amount charged for the weight product
			// (e.g. "1,06"). Use it as UnitPrice with qty=1 so that the
			// stored value reflects what was actually paid.
			if m := rePrice.FindStringSubmatch(trimmed); m != nil {
				price, err := parsePrice(m[1])
				if err == nil {
					t.Lines = append(t.Lines, TicketLine{
						Name:      pendingName,
						UnitPrice: price,
						Quantity:  1,
					})
				}
			}
			state = sQty

		case sUnitMultiTotal:
			// Discard the line total for qty>1 unit products and look for
			// the next product.
			state = sQty
		}
	}
}

// parseSingleLineBody handles the legacy single-line format used in tests:
//
//	"1   PRODUCT NAME   0,89"
//	"3   PRODUCT NAME   0,45   1,35"
//	"1   PRODUCT NAME\n0,354 kg   6,99 €/kg   2,47"
func (p *MercadonaParser) parseSingleLineBody(lines []string, t *Ticket) {
	inBody := false
	pendingWeightProduct := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if reFooter.MatchString(trimmed) {
			break
		}

		if reColumnHeaderSingle.MatchString(trimmed) {
			inBody = true
			continue
		}
		if !inBody || trimmed == "" {
			continue
		}

		// Weight continuation line.
		if pendingWeightProduct != "" {
			if m := reWeightLineSingle.FindStringSubmatch(trimmed); m != nil {
				// m[3] is the line total (amount paid); use that as UnitPrice
				// so the stored value reflects what was actually charged.
				lineTotal, err := parsePrice(m[3])
				if err == nil {
					t.Lines = append(t.Lines, TicketLine{
						Name:      pendingWeightProduct,
						UnitPrice: lineTotal,
						Quantity:  1,
					})
				}
				pendingWeightProduct = ""
				continue
			}
			pendingWeightProduct = ""
		}

		// Unit product, qty > 1.
		if m := reUnitMulti.FindStringSubmatch(trimmed); m != nil {
			qty, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			price, err := parsePrice(m[3])
			if err != nil {
				continue
			}
			t.Lines = append(t.Lines, TicketLine{
				Name:      strings.TrimSpace(m[2]),
				UnitPrice: price,
				Quantity:  qty,
			})
			continue
		}

		// Unit product, qty = 1.
		if m := reUnitSingle.FindStringSubmatch(trimmed); m != nil {
			price, err := parsePrice(m[2])
			if err != nil {
				continue
			}
			t.Lines = append(t.Lines, TicketLine{
				Name:      strings.TrimSpace(m[1]),
				UnitPrice: price,
				Quantity:  1,
			})
			continue
		}

		// Weight product first line: "1   PRODUCT NAME" (no price on this line).
		if strings.HasPrefix(trimmed, "1 ") || strings.HasPrefix(trimmed, "1\t") {
			rest := strings.TrimSpace(trimmed[1:])
			if !reTrailingPrice.MatchString(rest) && rest != "" {
				pendingWeightProduct = rest
				continue
			}
		}
	}
}

// splitLines splits text on newlines.
func splitLines(text string) []string {
	return strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
}

// parsePrice converts a Spanish-locale price string ("1,99") to float64.
func parsePrice(s string) (float64, error) {
	normalised := strings.ReplaceAll(strings.TrimSpace(s), ",", ".")
	return strconv.ParseFloat(normalised, 64)
}
