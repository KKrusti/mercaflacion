// Package enricher fetches product images from the Mercadona public API
// and stores them in the local database.
package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"
)

const (
	mercadonaBaseURL   = "https://tienda.mercadona.es/api"
	mercadonaLang      = "es"
	defaultHTTPTimeout = 15 * time.Second

	// subcategoryDelay is the minimum time between consecutive subcategory
	// requests. Mercadona's WAF rate-limits aggressively: bursts of requests
	// trigger a 403 that blocks the IP for ~2 minutes. 2 s gives a comfortable
	// margin (tested empirically; 0.5 s already causes intermittent blocks).
	subcategoryDelay = 2 * time.Second
)

// stopWords are tokens excluded from keyword matching because they appear in
// almost every product name and contribute no discriminating signal.
var stopWords = map[string]bool{
	// Spanish articles, prepositions, conjunctions
	"de": true, "del": true, "la": true, "el": true, "los": true, "las": true,
	"un": true, "una": true, "en": true, "con": true, "sin": true, "y": true,
	"a": true, "al": true, "o": true, "para": true, "por": true, "e": true,
	// Units / formats
	"kg": true, "g": true, "ml": true, "l": true, "cl": true, "ud": true,
	"uds": true, "u": true, "pack": true, "bot": true, "lata": true,
	// Catalan articles & prepositions (tickets from Catalan stores)
	"dels": true, "les": true, "uns": true, "unes": true, "amb": true,
	"per": true, "i": true, "d": true, "s": true,
}

// stem strips a trailing 's' from token when the result still has ≥ 3
// characters. This normalises the most common Spanish plural forms
// (cacahuetes→cacahuete, desgrasados→desgrasado, patatas→patata) without a
// full stemmer. It is applied to both local and catalogue keywords so that the
// comparison is always symmetric.
func stem(token string) string {
	runes := []rune(token)
	if len(runes) >= 4 && runes[len(runes)-1] == 's' {
		return string(runes[:len(runes)-1])
	}
	return token
}

// keywords returns the significant tokens from a normalised product name.
// Tokens shorter than 3 characters or in the stop-word list are discarded.
// Each surviving token is stemmed (trailing 's' removed) so that singular and
// plural forms resolve to the same key.
func keywords(normalised string) []string {
	parts := strings.Fields(normalised)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if len([]rune(p)) >= 3 && !stopWords[p] {
			out = append(out, stem(p))
		}
	}
	return out
}

// MercadonaClient fetches product data from the Mercadona public REST API.
type MercadonaClient struct {
	http    *http.Client
	baseURL string
}

// NewMercadonaClient returns a MercadonaClient with a sensible default timeout.
func NewMercadonaClient() *MercadonaClient {
	return &MercadonaClient{
		http:    &http.Client{Timeout: defaultHTTPTimeout},
		baseURL: mercadonaBaseURL,
	}
}

// --- API response types (unexported; only used internally) ---

type categoriesResponse struct {
	Results []topCategory `json:"results"`
}

type topCategory struct {
	ID         int           `json:"id"`
	Name       string        `json:"name"`
	Categories []subCategory `json:"categories"`
}

type subCategory struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Published bool   `json:"published"`
}

type categoryDetailResponse struct {
	Categories []subCategoryDetail `json:"categories"`
}

type subCategoryDetail struct {
	Products []mercadonaProduct `json:"products"`
}

type mercadonaProduct struct {
	DisplayName string `json:"display_name"`
	Thumbnail   string `json:"thumbnail"`
}

// mercadonaProductDetail holds the subset of fields returned by the
// per-product detail endpoint (GET /api/products/{id}/?lang=es).
type mercadonaProductDetail struct {
	Thumbnail string `json:"thumbnail"`
}

// FetchProductThumbnail calls the Mercadona per-product API endpoint and
// returns the thumbnail URL for the given numeric product ID.
func (c *MercadonaClient) FetchProductThumbnail(ctx context.Context, productID string) (string, error) {
	url := fmt.Sprintf("%s/products/%s/?lang=%s", c.baseURL, productID, mercadonaLang)
	var resp mercadonaProductDetail
	if err := c.getJSON(ctx, url, &resp); err != nil {
		return "", fmt.Errorf("fetch product %s: %w", productID, err)
	}
	if resp.Thumbnail == "" {
		return "", fmt.Errorf("product %s has no thumbnail in Mercadona catalogue", productID)
	}
	return resp.Thumbnail, nil
}

// ProductEntry holds the thumbnail URL and the keyword set for one Mercadona
// product. The keyword set is used for fuzzy matching against local names.
type ProductEntry struct {
	Thumbnail string
	Keywords  []string
}

// ProductIndex is a list of all Mercadona products with their keyword sets.
// It is searched linearly; given the catalogue size (~2 000 products) this is
// fast enough and avoids the complexity of an inverted index.
type ProductIndex []ProductEntry

// BuildProductIndex downloads all published subcategories from Mercadona,
// collects every product's display_name and thumbnail, and returns an index
// ready for keyword-based matching.
//
// Requests to subcategory endpoints are throttled to one every subcategoryDelay
// to avoid triggering Mercadona's WAF rate-limiter, which blocks the IP for
// ~2 minutes on bursts. The first categories request is unthrottled.
func (c *MercadonaClient) BuildProductIndex(ctx context.Context) (ProductIndex, error) {
	subcatIDs, err := c.fetchSubcategoryIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch subcategory ids: %w", err)
	}

	ticker := time.NewTicker(subcategoryDelay)
	defer ticker.Stop()

	var index ProductIndex
	for _, id := range subcatIDs {
		// Wait for the next tick before each subcategory request.
		select {
		case <-ctx.Done():
			return index, ctx.Err()
		case <-ticker.C:
		}

		products, err := c.fetchProductsInSubcategory(ctx, id)
		if err != nil {
			// Non-fatal: log and skip subcategory.
			log.Printf("enricher: skip subcategory %d: %v", id, err)
			continue
		}
		for _, p := range products {
			if p.Thumbnail == "" {
				continue
			}
			kw := keywords(normalise(p.DisplayName))
			if len(kw) == 0 {
				continue
			}
			index = append(index, ProductEntry{
				Thumbnail: p.Thumbnail,
				Keywords:  kw,
			})
		}
	}
	return index, nil
}

// fetchSubcategoryIDs returns the IDs of all published subcategories.
func (c *MercadonaClient) fetchSubcategoryIDs(ctx context.Context) ([]int, error) {
	url := fmt.Sprintf("%s/categories/?lang=%s", c.baseURL, mercadonaLang)
	var resp categoriesResponse
	if err := c.getJSON(ctx, url, &resp); err != nil {
		return nil, err
	}

	var ids []int
	for _, top := range resp.Results {
		for _, sub := range top.Categories {
			if sub.Published {
				ids = append(ids, sub.ID)
			}
		}
	}
	return ids, nil
}

// fetchProductsInSubcategory returns all products for the given subcategory ID.
func (c *MercadonaClient) fetchProductsInSubcategory(ctx context.Context, id int) ([]mercadonaProduct, error) {
	url := fmt.Sprintf("%s/categories/%d/?lang=%s", c.baseURL, id, mercadonaLang)
	var resp categoryDetailResponse
	if err := c.getJSON(ctx, url, &resp); err != nil {
		return nil, err
	}
	var products []mercadonaProduct
	for _, sub := range resp.Categories {
		products = append(products, sub.Products...)
	}
	return products, nil
}

// getJSON performs a GET request and decodes the JSON body into v.
// Browser-like headers are set to avoid 403 responses from Mercadona's WAF.
func (c *MercadonaClient) getJSON(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("new request %s: %w", url, err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "es-ES,es;q=0.9")
	req.Header.Set("Referer", "https://tienda.mercadona.es/")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get %s: unexpected status %d", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}

// normalise converts a product name to a lowercase, ASCII-only, whitespace-
// collapsed string used as a lookup key in ProductIndex.
// Accented characters are mapped to their base ASCII equivalent where possible.
// Non-letter, non-digit characters (including punctuation and apostrophes) act
// as word separators so that e.g. "d'Embolicar" → "d embolicar".
func normalise(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := true // treat start-of-string as a boundary

	for _, r := range s {
		// Map accented → base ASCII first.
		r = deaccent(r)

		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
			prevSpace = false
		} else {
			// Spaces, punctuation, apostrophes, etc. all become a single space separator.
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		}
	}

	return strings.TrimRight(b.String(), " ")
}

// deaccent maps accented characters commonly found in Spanish/Catalan product
// names to their unaccented ASCII equivalents.
func deaccent(r rune) rune {
	switch r {
	case 'à', 'á', 'â', 'ã', 'ä', 'å', 'À', 'Á', 'Â', 'Ã', 'Ä', 'Å':
		return 'a'
	case 'è', 'é', 'ê', 'ë', 'È', 'É', 'Ê', 'Ë':
		return 'e'
	case 'ì', 'í', 'î', 'ï', 'Ì', 'Í', 'Î', 'Ï':
		return 'i'
	case 'ò', 'ó', 'ô', 'õ', 'ö', 'Ò', 'Ó', 'Ô', 'Õ', 'Ö':
		return 'o'
	case 'ù', 'ú', 'û', 'ü', 'Ù', 'Ú', 'Û', 'Ü':
		return 'u'
	case 'ñ', 'Ñ':
		return 'n'
	case 'ç', 'Ç':
		return 'c'
	case 'ł', 'Ł':
		return 'l'
	default:
		return r
	}
}
