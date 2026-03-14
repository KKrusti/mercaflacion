package ticket_test

import (
	"strings"
	"testing"
	"time"

	"basket-cost/pkg/ticket"
)

// minimalReceipt returns a receipt text fragment that contains the minimum
// required fields (date, invoice, column header) plus a variable body.
func receipt(body string) string {
	return strings.Join([]string{
		"MERCADONA, S.A.   A-46103834",
		"C/ MONTSERRAT, 158 / 08140 Caldes de Montbui",
		"09/02/2026 19:43   OP: 2570140",
		"FACTURA SIMPLIFICADA: 4144-017-284404",
		"Descripció   P. Unit   Import",
		body,
		"TOTAL (€)   9,67",
	}, "\n")
}

func TestMercadonaParser_DateParsed(t *testing.T) {
	p := ticket.NewMercadonaParser()
	text := receipt("")
	got, err := p.Parse(text)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	want := time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC)
	if !got.Date.Equal(want) {
		t.Errorf("Date: want %s, got %s", want, got.Date)
	}
}

func TestMercadonaParser_InvoiceNumber(t *testing.T) {
	p := ticket.NewMercadonaParser()
	got, err := p.Parse(receipt(""))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if got.InvoiceNumber != "4144-017-284404" {
		t.Errorf("InvoiceNumber: want %q, got %q", "4144-017-284404", got.InvoiceNumber)
	}
}

func TestMercadonaParser_StoreName(t *testing.T) {
	p := ticket.NewMercadonaParser()
	got, err := p.Parse(receipt(""))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if got.Store != "Mercadona" {
		t.Errorf("Store: want %q, got %q", "Mercadona", got.Store)
	}
}

func TestMercadonaParser_MissingDate_ReturnsError(t *testing.T) {
	p := ticket.NewMercadonaParser()
	text := "FACTURA SIMPLIFICADA: 1234-567-890\nDescripció   P. Unit   Import\nTOTAL (€)   1,00"
	_, err := p.Parse(text)
	if err == nil {
		t.Error("expected error for missing date, got nil")
	}
}

func TestMercadonaParser_UnitProduct_QtyOne(t *testing.T) {
	p := ticket.NewMercadonaParser()
	body := "1   LECHE ENTERA HACENDADO 1L   0,89"
	got, err := p.Parse(receipt(body))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(got.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(got.Lines))
	}
	line := got.Lines[0]
	if line.Name != "LECHE ENTERA HACENDADO 1L" {
		t.Errorf("Name: want %q, got %q", "LECHE ENTERA HACENDADO 1L", line.Name)
	}
	if line.UnitPrice != 0.89 {
		t.Errorf("UnitPrice: want 0.89, got %f", line.UnitPrice)
	}
	if line.Quantity != 1 {
		t.Errorf("Quantity: want 1, got %d", line.Quantity)
	}
}

func TestMercadonaParser_UnitProduct_QtyMany(t *testing.T) {
	p := ticket.NewMercadonaParser()
	body := "3   AGUA MINERAL 1,5L   0,45   1,35"
	got, err := p.Parse(receipt(body))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(got.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(got.Lines))
	}
	line := got.Lines[0]
	if line.Name != "AGUA MINERAL 1,5L" {
		t.Errorf("Name: want %q, got %q", "AGUA MINERAL 1,5L", line.Name)
	}
	if line.UnitPrice != 0.45 {
		t.Errorf("UnitPrice: want 0.45, got %f", line.UnitPrice)
	}
	if line.Quantity != 3 {
		t.Errorf("Quantity: want 3, got %d", line.Quantity)
	}
}

func TestMercadonaParser_WeightProduct(t *testing.T) {
	p := ticket.NewMercadonaParser()
	body := "1   PECHUGA POLLO\n0,354 kg   6,99 €/kg   2,47"
	got, err := p.Parse(receipt(body))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(got.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %+v", len(got.Lines), got.Lines)
	}
	line := got.Lines[0]
	if line.Name != "PECHUGA POLLO" {
		t.Errorf("Name: want %q, got %q", "PECHUGA POLLO", line.Name)
	}
	if line.UnitPrice != 2.47 {
		t.Errorf("UnitPrice (line total): want 2.47, got %f", line.UnitPrice)
	}
	if line.Quantity != 1 {
		t.Errorf("Quantity: want 1, got %d", line.Quantity)
	}
}

func TestMercadonaParser_MultipleProducts(t *testing.T) {
	p := ticket.NewMercadonaParser()
	body := strings.Join([]string{
		"1   LECHE ENTERA HACENDADO 1L   0,89",
		"2   YOGUR NATURAL   0,35   0,70",
		"1   LOMO CERDO",
		"0,500 kg   9,90 €/kg   4,95",
	}, "\n")
	got, err := p.Parse(receipt(body))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(got.Lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %+v", len(got.Lines), got.Lines)
	}
}

func TestMercadonaParser_FooterLinesIgnored(t *testing.T) {
	p := ticket.NewMercadonaParser()
	body := "1   LECHE ENTERA HACENDADO 1L   0,89"
	text := receipt(body) + "\nIVA   BASE IMPOSABLE (€)   QUOTA (€)\n4%   0,50   0,02"
	got, err := p.Parse(text)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// The IVA lines should not produce extra product lines.
	if len(got.Lines) != 1 {
		t.Errorf("expected 1 line, got %d", len(got.Lines))
	}
}

// ── Multi-line (real PDF) format tests ───────────────────────────────────────

// receiptMulti builds a receipt using the multi-line column header format
// produced by the ledongthuc/pdf extractor.
func receiptMulti(body string) string {
	return strings.Join([]string{
		"MERCADONA, S.A.   A-46103834",
		"C/ MONTSERRAT, 158",
		"08140 Caldes de Montbui",
		"TELÈFON:",
		"938827220",
		"09/02/2026 19:43 ",
		" OP: 2570140",
		"FACTURA SIMPLIFICADA: 4144-017-284404",
		"Descripció",
		"P. Unit",
		"Import",
		body,
		"TOTAL (€)",
		"9,67",
	}, "\n")
}

func TestMercadonaParser_MultiLine_DateAndInvoice(t *testing.T) {
	p := ticket.NewMercadonaParser()
	got, err := p.Parse(receiptMulti(""))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	wantDate := time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC)
	if !got.Date.Equal(wantDate) {
		t.Errorf("Date: want %s, got %s", wantDate, got.Date)
	}
	if got.InvoiceNumber != "4144-017-284404" {
		t.Errorf("InvoiceNumber: want %q, got %q", "4144-017-284404", got.InvoiceNumber)
	}
}

func TestMercadonaParser_MultiLine_UnitProduct_QtyOne(t *testing.T) {
	p := ticket.NewMercadonaParser()
	body := strings.Join([]string{"1", "ARRÒS INTEGRAL", "1,10"}, "\n")
	got, err := p.Parse(receiptMulti(body))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(got.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %+v", len(got.Lines), got.Lines)
	}
	line := got.Lines[0]
	if line.Name != "ARRÒS INTEGRAL" {
		t.Errorf("Name: want %q, got %q", "ARRÒS INTEGRAL", line.Name)
	}
	if line.UnitPrice != 1.10 {
		t.Errorf("UnitPrice: want 1.10, got %f", line.UnitPrice)
	}
	if line.Quantity != 1 {
		t.Errorf("Quantity: want 1, got %d", line.Quantity)
	}
}

func TestMercadonaParser_MultiLine_UnitProduct_QtyMany(t *testing.T) {
	p := ticket.NewMercadonaParser()
	body := strings.Join([]string{"3", "ENERGY DRINK KATRINE", "1,00", "3,00"}, "\n")
	got, err := p.Parse(receiptMulti(body))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(got.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %+v", len(got.Lines), got.Lines)
	}
	line := got.Lines[0]
	if line.Name != "ENERGY DRINK KATRINE" {
		t.Errorf("Name: want %q, got %q", "ENERGY DRINK KATRINE", line.Name)
	}
	if line.UnitPrice != 1.00 {
		t.Errorf("UnitPrice: want 1.00, got %f", line.UnitPrice)
	}
	if line.Quantity != 3 {
		t.Errorf("Quantity: want 3, got %d", line.Quantity)
	}
}

func TestMercadonaParser_MultiLine_WeightProduct(t *testing.T) {
	p := ticket.NewMercadonaParser()
	body := strings.Join([]string{"1", "CARBASSÓ VERD", "0,432 kg", "2,45 €/kg", "1,06"}, "\n")
	got, err := p.Parse(receiptMulti(body))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(got.Lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %+v", len(got.Lines), got.Lines)
	}
	line := got.Lines[0]
	if line.Name != "CARBASSÓ VERD" {
		t.Errorf("Name: want %q, got %q", "CARBASSÓ VERD", line.Name)
	}
	if line.UnitPrice != 1.06 {
		t.Errorf("UnitPrice (line total): want 1.06, got %f", line.UnitPrice)
	}
	if line.Quantity != 1 {
		t.Errorf("Quantity: want 1, got %d", line.Quantity)
	}
}

func TestMercadonaParser_MultiLine_MultipleProducts(t *testing.T) {
	p := ticket.NewMercadonaParser()
	body := strings.Join([]string{
		// qty=1
		"1", "ARRÒS INTEGRAL", "1,10",
		// qty=3
		"3", "ENERGY DRINK KATRINE", "1,00", "3,00",
		// weight
		"1", "CARBASSÓ VERD", "0,432 kg", "2,45 €/kg", "1,06",
	}, "\n")
	got, err := p.Parse(receiptMulti(body))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(got.Lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %+v", len(got.Lines), got.Lines)
	}
}
