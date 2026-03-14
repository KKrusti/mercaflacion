package ticket

import (
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDFExtractor is the contract for extracting raw text from a PDF source.
// Having an interface here lets tests inject a fake without touching the
// filesystem or the PDF library.
type PDFExtractor interface {
	// Extract reads all pages of the PDF from r (which must implement ReaderAt)
	// and returns the concatenated plain-text content.
	Extract(r io.ReaderAt, size int64) (string, error)
}

// LedongthucExtractor implements PDFExtractor using the ledongthuc/pdf library.
type LedongthucExtractor struct{}

// NewExtractor returns a ready-to-use LedongthucExtractor.
func NewExtractor() *LedongthucExtractor {
	return &LedongthucExtractor{}
}

// Extract reads the PDF from r and returns its plain-text content.
// All pages are concatenated with a newline separator.
func (e *LedongthucExtractor) Extract(r io.ReaderAt, size int64) (string, error) {
	reader, err := pdf.NewReader(r, size)
	if err != nil {
		return "", fmt.Errorf("open pdf reader: %w", err)
	}

	var sb strings.Builder
	numPages := reader.NumPage()
	for i := 1; i <= numPages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			return "", fmt.Errorf("extract text from page %d: %w", i, err)
		}
		sb.WriteString(text)
		sb.WriteByte('\n')
	}

	return sb.String(), nil
}
