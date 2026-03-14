package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	myMemoryBaseURL    = "https://api.mymemory.translated.net"
	translatorLangPair = "ca|es"
	translatorTimeout  = 5 * time.Second
)

// Translator translates a normalised Catalan product name to Spanish.
// Implementations must be safe for concurrent use.
type Translator interface {
	Translate(ctx context.Context, text string) (string, error)
}

// myMemoryResponse holds the subset of fields returned by the MyMemory API.
// ResponseStatus is json.Number because the API has returned both "200" (string)
// and 200 (number) depending on the endpoint version.
type myMemoryResponse struct {
	ResponseData struct {
		TranslatedText string  `json:"translatedText"`
		Match          float64 `json:"match"`
	} `json:"responseData"`
	ResponseStatus json.Number `json:"responseStatus"`
}

// MyMemoryTranslator calls the free MyMemory translation API (ca→es) and
// caches results in memory to avoid repeated requests for the same text.
// The API requires no authentication for low-volume use.
type MyMemoryTranslator struct {
	client  *http.Client
	baseURL string   // overridable for testing
	cache   sync.Map // map[string]string
}

// NewMyMemoryTranslator returns a MyMemoryTranslator ready for use.
func NewMyMemoryTranslator() *MyMemoryTranslator {
	return &MyMemoryTranslator{
		client:  &http.Client{Timeout: translatorTimeout},
		baseURL: myMemoryBaseURL,
	}
}

// Translate returns a Spanish translation of text (Catalan input).
// Results are cached; subsequent calls with the same text return instantly.
func (t *MyMemoryTranslator) Translate(ctx context.Context, text string) (string, error) {
	if text == "" {
		return "", nil
	}
	if cached, ok := t.cache.Load(text); ok {
		return cached.(string), nil
	}

	apiURL := fmt.Sprintf(
		"%s/get?q=%s&langpair=%s",
		t.baseURL,
		url.QueryEscape(text),
		translatorLangPair,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("translator: build request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("translator: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("translator: unexpected status %d", resp.StatusCode)
	}

	var body myMemoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("translator: decode response: %w", err)
	}

	translated := body.ResponseData.TranslatedText
	if translated == "" {
		return "", fmt.Errorf("translator: empty translation for %q", text)
	}

	t.cache.Store(text, translated)
	return translated, nil
}

// NoopTranslator returns text unchanged. Used when translation is disabled or
// when a real Translator is not available (e.g. in tests that don't need it).
type NoopTranslator struct{}

func (NoopTranslator) Translate(_ context.Context, text string) (string, error) {
	return text, nil
}
