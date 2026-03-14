package enricher

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"basket-cost/pkg/store"
)

// FetchProductThumbnail returns the direct image URL for a Mercadona product
// identified by its numeric catalogue ID (as it appears in tienda.mercadona.es URLs).
func (e *Enricher) FetchProductThumbnail(ctx context.Context, productID string) (string, error) {
	return e.client.FetchProductThumbnail(ctx, productID)
}

// minMatchScore is the minimum Dice coefficient required to accept a match.
// Dice = 2·|A∩B| / (|A|+|B|), so it penalises both unmatched local keywords
// (recall) and unmatched catalogue keywords (precision).
// 0.5 means the intersection must be at least half the union of both sets —
// this rejects cases like "patata" matching "patatas fritas onduladas Pringles"
// (Dice ≈ 0.33) while still accepting "patata 3kg" matching "Patata 3 kg
// Hacendado" (Dice ≈ 0.67).
const minMatchScore = 0.5

// minMatched returns the minimum number of keyword matches required before
// the Dice score is even evaluated.  When the local product has ≥ 2 keywords
// we demand at least 2 shared tokens; this prevents a single shared token
// (e.g. "patatas") from matching a completely different catalogue entry
// (e.g. "Patatas 3 kg") just because that entry ends up with only one
// meaningful keyword after stop-word filtering.
func minMatched(localLen int) int {
	if localLen >= 2 {
		return 2
	}
	return 1
}

// indexTTL is how long the cached Mercadona product index stays valid.
// After this duration the next Run() call will refresh it from the API.
const indexTTL = 24 * time.Hour

// Enricher downloads the Mercadona product catalogue and updates image URLs
// for matching products in the local store.
//
// The Mercadona product index is cached for indexTTL so that uploading
// multiple tickets in quick succession does not trigger repeated full
// catalogue downloads.
//
// Schedule() signals a pending enrichment run. A background worker started
// by Start() drains those signals one at a time, so N concurrent ticket
// uploads always result in at most one active Run() plus one queued — never
// a burst of parallel requests to the Mercadona API.
type Enricher struct {
	client     *MercadonaClient
	store      store.Store
	translator Translator

	mu             sync.Mutex // guards cachedIndex and indexFetchedAt
	cachedIndex    ProductIndex
	indexFetchedAt time.Time

	// pending is a buffered channel of capacity 1.
	// A non-blocking send on it schedules exactly one future Run(); extra
	// signals sent while a run is already queued or in progress are dropped.
	pending chan struct{}
}

// New returns an Enricher backed by the given store using the MyMemory
// translation API (Catalan→Spanish) for product name normalisation.
// Call Start to launch the background worker before using Schedule.
func New(s store.Store) *Enricher {
	return newEnricher(s, NewMyMemoryTranslator())
}

// newEnricher is the internal constructor that accepts a custom Translator.
// Used in tests to inject a mock and in New to wire the production client.
func newEnricher(s store.Store, t Translator) *Enricher {
	return &Enricher{
		client:     NewMercadonaClient(),
		store:      s,
		translator: t,
		pending:    make(chan struct{}, 1),
	}
}

// Start launches the background worker that processes scheduled enrichment
// runs. It returns immediately; the worker goroutine exits when ctx is
// cancelled. Start must be called exactly once before any call to Schedule.
func (e *Enricher) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-e.pending:
				res, err := e.Run(ctx)
				if err != nil {
					log.Printf("enricher: background run failed: %v", err)
					continue
				}
				log.Printf("enricher: updated %d/%d products", res.Updated, res.Total)
			}
		}
	}()
}

// Schedule signals the background worker to execute an enrichment run.
// If a run is already queued the signal is coalesced — the worker will
// still execute exactly one additional run after the current one finishes.
// Schedule is safe to call from multiple goroutines concurrently.
func (e *Enricher) Schedule() {
	select {
	case e.pending <- struct{}{}:
	default:
		// A run is already pending; no need to queue another.
	}
}

// productIndex returns the cached Mercadona index, rebuilding it from the API
// if the cache is empty or older than indexTTL.
func (e *Enricher) productIndex(ctx context.Context) (ProductIndex, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.cachedIndex) > 0 && time.Since(e.indexFetchedAt) < indexTTL {
		return e.cachedIndex, nil
	}

	log.Println("enricher: building Mercadona product index…")
	index, err := e.client.BuildProductIndex(ctx)
	if err != nil {
		return nil, fmt.Errorf("build product index: %w", err)
	}
	log.Printf("enricher: index contains %d entries", len(index))

	e.cachedIndex = index
	e.indexFetchedAt = time.Now()
	return index, nil
}

// EnrichResult summarises the outcome of a single enrichment run.
type EnrichResult struct {
	Total   int // products inspected
	Updated int // products whose image URL was set
	Skipped int // products with no match in the Mercadona index
}

// Run fetches the Mercadona catalogue (from cache when possible), matches it
// against products that have no image URL yet, and updates image_url for
// every match.  Products that already have an image are skipped.
func (e *Enricher) Run(ctx context.Context) (EnrichResult, error) {
	index, err := e.productIndex(ctx)
	if err != nil {
		return EnrichResult{}, err
	}

	results, err := e.store.GetProductsWithoutImage()
	if err != nil {
		return EnrichResult{}, fmt.Errorf("list products without image: %w", err)
	}

	var res EnrichResult
	res.Total = len(results)

	for _, p := range results {
		localKW := e.productKeywords(ctx, p.Name)
		if len(localKW) == 0 {
			res.Skipped++
			continue
		}

		url, ok := bestMatch(localKW, index)
		if !ok {
			res.Skipped++
			continue
		}
		if err := e.store.UpdateProductImageURL(p.ID, url); err != nil {
			return res, fmt.Errorf("update image for %s: %w", p.ID, err)
		}
		res.Updated++
	}

	return res, nil
}

// productKeywords returns the significant keywords for a raw product name.
//
// The external translator receives the lowercase original name (accents
// preserved) so that the API can correctly identify accented Catalan tokens
// such as "tomàquet" or "arrós". The result is normalised before keyword
// extraction so that any accented Spanish output is deaccented consistently
// with the Mercadona catalogue index (which is also built via normalise).
//
// On translator failure the built-in dictionary is used as a fallback so the
// enrichment run is never blocked by external API unavailability.
func (e *Enricher) productKeywords(ctx context.Context, name string) []string {
	normalised := normalise(name)

	translated, err := e.translator.Translate(ctx, strings.ToLower(name))
	if err != nil {
		log.Printf("enricher: translation failed for %q, falling back to dictionary: %v", name, err)
		return keywords(translateCatalan(normalised))
	}

	return keywords(normalise(translated))
}

// bestMatch finds the ProductEntry whose keyword set best overlaps with the
// local keywords using the Dice coefficient:
//
//	Dice = 2 · |intersection| / (|local| + |entry|)
//
// This metric penalises both missed local keywords (recall) and excess
// catalogue keywords (precision), preventing a single shared token like
// "patata" from matching an unrelated product with many extra keywords.
// It returns the thumbnail URL and true if the best score ≥ minMatchScore.
func bestMatch(localKW []string, index ProductIndex) (string, bool) {
	if len(localKW) == 0 {
		return "", false
	}

	localSet := make(map[string]bool, len(localKW))
	for _, k := range localKW {
		localSet[k] = true
	}

	bestScore := 0.0
	bestURL := ""

	for _, entry := range index {
		if len(entry.Keywords) == 0 {
			continue
		}

		matched := 0
		for _, k := range entry.Keywords {
			if localSet[k] {
				matched++
			}
		}
		if matched < minMatched(len(localKW)) {
			continue
		}

		// Dice coefficient: 2·|A∩B| / (|A|+|B|)
		score := 2.0 * float64(matched) / float64(len(localKW)+len(entry.Keywords))
		if score > bestScore {
			bestScore = score
			bestURL = entry.Thumbnail
		}
	}

	if bestScore >= minMatchScore {
		return bestURL, true
	}
	return "", false
}
