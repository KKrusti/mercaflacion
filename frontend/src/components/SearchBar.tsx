import { useState, useEffect, useCallback } from 'react';
import type { SearchResult } from '../types';
import { searchProducts } from '../api/products';
import ProductBrowser from './ProductBrowser';
import type { ProductBrowserState } from './ProductBrowser';
import ProductImage from './ProductImage';

interface SearchBarProps {
  onSelectProduct: (id: string) => void;
  browserState?: ProductBrowserState;
  onBrowserStateChange?: (state: ProductBrowserState) => void;
}

function SearchIcon() {
  return (
    <svg
      className="search-icon"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="11" cy="11" r="8" />
      <line x1="21" y1="21" x2="16.65" y2="16.65" />
    </svg>
  );
}

function EmptyResultsIcon() {
  return (
    <svg
      className="empty-state__icon"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="11" cy="11" r="8" />
      <line x1="21" y1="21" x2="16.65" y2="16.65" />
      <line x1="8" y1="11" x2="14" y2="11" />
    </svg>
  );
}

export default function SearchBar({ onSelectProduct, browserState, onBrowserStateChange }: SearchBarProps) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);

  const doSearch = useCallback(async (q: string) => {
    if (q.trim().length === 0) {
      setResults([]);
      setSearched(false);
      return;
    }
    setLoading(true);
    try {
      const data = await searchProducts(q);
      setResults(data);
      setSearched(true);
    } catch (err) {
      if (import.meta.env.DEV) console.error('Search error:', err);
      setResults([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = setTimeout(() => {
      doSearch(query);
    }, 300);
    return () => clearTimeout(timer);
  }, [query, doSearch]);

  const formatPrice = (price: number) =>
    price.toFixed(2).replace('.', ',') + ' \u20AC';

  return (
    <div className="search-wrapper">
      <div className="search-fixed">
        <div className="search-container">
          <SearchIcon />
          <input
            type="text"
            className="search-input"
            placeholder="Buscar producto... (leche, aceite, pan)"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            autoFocus
            aria-label="Buscar producto"
          />
        </div>
      </div>

      <div className="search-results">
        {loading && (
          <div>
            <div className="loading">Searching...</div>
            <div className="product-list" aria-hidden="true">
              {[1, 2, 3].map((n) => (
                <div key={n} className="skeleton skeleton-card" />
              ))}
            </div>
          </div>
        )}

        {!loading && searched && results.length === 0 && (
          <div className="empty-state">
            <EmptyResultsIcon />
            <p>Sin resultados para &quot;{query}&quot;</p>
            <p className="empty-state__hint">Prueba con otro término de búsqueda</p>
          </div>
        )}

        {!loading && results.length > 0 && (
          <div className="product-list" role="list">
            {results.map((product) => (
              <button
                key={product.id}
                className="product-card"
                onClick={() => onSelectProduct(product.id)}
                role="listitem"
                aria-label={`${product.name} — ${formatPrice(product.currentPrice)}`}
              >
                <ProductImage productId={product.id} category={product.category} size="md" />
                <div className="product-card-info">
                  <h3>{product.name}</h3>
                  {product.category && (
                    <span className="category">{product.category}</span>
                  )}
                </div>
                <div className="product-card-price">
                  <div className="current">{formatPrice(product.currentPrice)}</div>
                  <div className="range">
                    {formatPrice(product.minPrice)} – {formatPrice(product.maxPrice)}
                  </div>
                </div>
              </button>
            ))}
          </div>
        )}

        {!loading && !searched && (
          <ProductBrowser
            onSelectProduct={onSelectProduct}
            browserState={browserState}
            onBrowserStateChange={onBrowserStateChange}
          />
        )}
      </div>
    </div>
  );
}
