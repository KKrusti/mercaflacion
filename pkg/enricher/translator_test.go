package enricher

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------- NoopTranslator ----------

func TestNoopTranslator_ReturnsTextUnchanged(t *testing.T) {
	var tr NoopTranslator
	got, err := tr.Translate(context.Background(), "llet entera")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "llet entera" {
		t.Errorf("Translate() = %q, want %q", got, "llet entera")
	}
}

func TestNoopTranslator_EmptyInput(t *testing.T) {
	var tr NoopTranslator
	got, err := tr.Translate(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("Translate(%q) = %q, want empty", "", got)
	}
}

// ---------- helpers ----------

// newTestTranslator returns a MyMemoryTranslator that points at the given
// httptest.Server instead of the real MyMemory endpoint.
func newTestTranslator(srv *httptest.Server) *MyMemoryTranslator {
	return &MyMemoryTranslator{
		client:  srv.Client(),
		baseURL: srv.URL,
	}
}

// myMemoryServerWith starts a test server that always responds with translated.
func myMemoryServerWith(t *testing.T, translated string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := myMemoryResponse{}
		resp.ResponseData.TranslatedText = translated
		resp.ResponseData.Match = 1.0
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// ---------- MyMemoryTranslator ----------

func TestMyMemoryTranslator_TranslatesSuccessfully(t *testing.T) {
	srv := myMemoryServerWith(t, "leche entera")
	defer srv.Close()

	tr := newTestTranslator(srv)
	got, err := tr.Translate(context.Background(), "llet entera")
	if err != nil {
		t.Fatalf("Translate() unexpected error: %v", err)
	}
	if got != "leche entera" {
		t.Errorf("Translate() = %q, want %q", got, "leche entera")
	}
}

func TestMyMemoryTranslator_CachesResult(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		resp := myMemoryResponse{}
		resp.ResponseData.TranslatedText = "leche entera"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tr := newTestTranslator(srv)
	ctx := context.Background()
	if _, err := tr.Translate(ctx, "llet entera"); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := tr.Translate(ctx, "llet entera"); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 HTTP call (cache hit on 2nd); got %d", calls)
	}
}

func TestMyMemoryTranslator_EmptyInputReturnsEmpty(t *testing.T) {
	tr := NewMyMemoryTranslator()
	got, err := tr.Translate(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("Translate(%q) = %q, want empty", "", got)
	}
}

func TestMyMemoryTranslator_HTTPErrorReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	tr := newTestTranslator(srv)
	_, err := tr.Translate(context.Background(), "llet entera")
	if err == nil {
		t.Error("expected error for non-200 response, got nil")
	}
}

func TestMyMemoryTranslator_EmptyTranslatedTextReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := myMemoryResponse{}
		resp.ResponseData.TranslatedText = ""
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	tr := newTestTranslator(srv)
	_, err := tr.Translate(context.Background(), "llet entera")
	if err == nil {
		t.Error("expected error when translated text is empty, got nil")
	}
}

// ---------- mockTranslator (for Enricher tests) ----------

type mockTranslator struct {
	results map[string]string
	err     error
	calls   int
}

func (m *mockTranslator) Translate(_ context.Context, text string) (string, error) {
	m.calls++
	if m.err != nil {
		return "", m.err
	}
	if v, ok := m.results[text]; ok {
		return v, nil
	}
	return text, nil
}

// ---------- productKeywords ----------

func TestProductKeywords_UsesTranslator(t *testing.T) {
	tr := &mockTranslator{results: map[string]string{
		"llet entera": "leche entera",
	}}
	e := newEnricher(nil, tr)

	kw := e.productKeywords(context.Background(), "LLET ENTERA")
	if tr.calls == 0 {
		t.Error("expected translator to be called")
	}

	kwSet := make(map[string]bool, len(kw))
	for _, k := range kw {
		kwSet[k] = true
	}
	if !kwSet["leche"] {
		t.Errorf("productKeywords missing 'leche'; got %v", kw)
	}
	if !kwSet["entera"] {
		t.Errorf("productKeywords missing 'entera'; got %v", kw)
	}
}

func TestProductKeywords_FallsBackToDictionaryOnError(t *testing.T) {
	tr := &mockTranslator{err: errors.New("API unavailable")}
	e := newEnricher(nil, tr)

	// "LLET" → dictionary maps "llet" to "leche"
	kw := e.productKeywords(context.Background(), "LLET ENTERA")
	kwSet := make(map[string]bool, len(kw))
	for _, k := range kw {
		kwSet[k] = true
	}
	if !kwSet["leche"] {
		t.Errorf("expected fallback to dictionary: 'llet'→'leche'; got %v", kw)
	}
}

// TestProductKeywords_SendsLowercaseOriginalToTranslator verifies that
// productKeywords sends the lowercase original name (with accents) rather
// than the normalised (deaccented) form, so the external API can correctly
// identify accented Catalan words like "tomàquet".
func TestProductKeywords_SendsLowercaseOriginalToTranslator(t *testing.T) {
	// "tomàquet fregit" (accented lowercase) triggers the mock.
	// The old behaviour would send "tomaquet fregit" (normalised, no accent),
	// which would NOT trigger the mock and fall back to passthrough.
	tr := &mockTranslator{results: map[string]string{
		"tomàquet fregit": "tomate frito",
	}}
	e := newEnricher(nil, tr)

	kw := e.productKeywords(context.Background(), "TOMÀQUET FREGIT")
	kwSet := make(map[string]bool, len(kw))
	for _, k := range kw {
		kwSet[k] = true
	}
	if !kwSet["tomate"] {
		t.Errorf("expected 'tomate' in keywords (translator received lowercase original); got %v", kw)
	}
}

// TestProductKeywords_NormalisesTranslatorResult verifies that the result
// returned by the external translator is deaccented before keyword extraction,
// ensuring consistency with the Mercadona catalogue index (also normalised).
func TestProductKeywords_NormalisesTranslatorResult(t *testing.T) {
	// Translator returns "café" (accented Spanish). Keywords must deaccent it
	// to "cafe" to match catalogue entries built via normalise().
	tr := &mockTranslator{results: map[string]string{
		"café": "café",
	}}
	e := newEnricher(nil, tr)

	kw := e.productKeywords(context.Background(), "CAFÉ")
	kwSet := make(map[string]bool, len(kw))
	for _, k := range kw {
		kwSet[k] = true
	}
	if !kwSet["cafe"] {
		t.Errorf("expected deaccented 'cafe' in keywords; got %v", kw)
	}
	if kwSet["café"] {
		t.Error("accented 'café' should not appear in keywords; normalise must be applied")
	}
}

// TestProductKeywords_CaramelSalatEndToEnd verifies the full pipeline fix:
// "CARAMEL SALAT" must produce keywords ["caramelo", "salado"] via the
// dictionary fallback, matching the Mercadona catalogue "Caramelo Salado".
func TestProductKeywords_CaramelSalatEndToEnd(t *testing.T) {
	tr := &mockTranslator{err: errors.New("API unavailable")}
	e := newEnricher(nil, tr)

	kw := e.productKeywords(context.Background(), "CARAMEL SALAT")
	kwSet := make(map[string]bool, len(kw))
	for _, k := range kw {
		kwSet[k] = true
	}
	if !kwSet["caramelo"] {
		t.Errorf("expected 'caramelo' in keywords for 'CARAMEL SALAT'; got %v", kw)
	}
	if !kwSet["salado"] {
		t.Errorf("expected 'salado' in keywords for 'CARAMEL SALAT'; got %v", kw)
	}
}

func TestProductKeywords_NonCatalanPreserved(t *testing.T) {
	tr := &mockTranslator{results: map[string]string{
		"coca cola zero": "coca cola zero",
	}}
	e := newEnricher(nil, tr)

	kw := e.productKeywords(context.Background(), "COCA COLA ZERO")
	kwSet := make(map[string]bool, len(kw))
	for _, k := range kw {
		kwSet[k] = true
	}
	if !kwSet["coca"] || !kwSet["cola"] || !kwSet["zero"] {
		t.Errorf("productKeywords for non-Catalan = %v, want [coca cola zero]", kw)
	}
}
