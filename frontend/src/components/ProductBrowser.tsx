import { useState, useEffect } from 'react';
import type { SearchResult } from '../types';
import { getAllProducts } from '../api/products';
import ProductImage from './ProductImage';

type Columns = 3 | 4;
type PageSize = 12 | 24 | 48 | 96;

const PAGE_SIZES: PageSize[] = [12, 24, 48, 96];

export interface ProductBrowserState {
  page: number;
  pageSize: PageSize;
  columns: Columns;
}

interface ProductBrowserProps {
  onSelectProduct: (id: string) => void;
  browserState?: ProductBrowserState;
  onBrowserStateChange?: (state: ProductBrowserState) => void;
}

// SVG icons — inline, no external dependency

function GridIcon3() {
  return (
    <svg viewBox="0 0 20 20" fill="currentColor" width="16" height="16" aria-hidden="true">
      <rect x="1"  y="1" width="5" height="5" rx="1" />
      <rect x="7"  y="1" width="6" height="5" rx="1" />
      <rect x="14" y="1" width="5" height="5" rx="1" />
      <rect x="1"  y="8" width="5" height="5" rx="1" />
      <rect x="7"  y="8" width="6" height="5" rx="1" />
      <rect x="14" y="8" width="5" height="5" rx="1" />
      <rect x="1"  y="15" width="5" height="5" rx="1" />
      <rect x="7"  y="15" width="6" height="5" rx="1" />
      <rect x="14" y="15" width="5" height="5" rx="1" />
    </svg>
  );
}

function GridIcon4() {
  return (
    <svg viewBox="0 0 20 20" fill="currentColor" width="16" height="16" aria-hidden="true">
      <rect x="1"  y="1" width="4" height="4" rx="1" />
      <rect x="6"  y="1" width="4" height="4" rx="1" />
      <rect x="11" y="1" width="4" height="4" rx="1" />
      <rect x="15" y="1" width="4" height="4" rx="1" />
      <rect x="1"  y="7" width="4" height="4" rx="1" />
      <rect x="6"  y="7" width="4" height="4" rx="1" />
      <rect x="11" y="7" width="4" height="4" rx="1" />
      <rect x="15" y="7" width="4" height="4" rx="1" />
      <rect x="1"  y="13" width="4" height="4" rx="1" />
      <rect x="6"  y="13" width="4" height="4" rx="1" />
      <rect x="11" y="13" width="4" height="4" rx="1" />
      <rect x="15" y="13" width="4" height="4" rx="1" />
    </svg>
  );
}

function ChevronLeftIcon() {
  return (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="2"
      strokeLinecap="round" strokeLinejoin="round" width="16" height="16" aria-hidden="true">
      <polyline points="13 16 7 10 13 4" />
    </svg>
  );
}

function ChevronRightIcon() {
  return (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="2"
      strokeLinecap="round" strokeLinejoin="round" width="16" height="16" aria-hidden="true">
      <polyline points="7 4 13 10 7 16" />
    </svg>
  );
}

const DEFAULT_STATE: ProductBrowserState = { page: 0, pageSize: 48, columns: 3 };

export default function ProductBrowser({
  onSelectProduct,
  browserState,
  onBrowserStateChange,
}: ProductBrowserProps) {
  const controlled = browserState !== undefined && onBrowserStateChange !== undefined;

  // Internal state — used only when running uncontrolled (no parent state provided).
  const [internalState, setInternalState] = useState<ProductBrowserState>(DEFAULT_STATE);

  const { page, pageSize, columns } = controlled ? browserState : internalState;

  function updateState(next: Partial<ProductBrowserState>) {
    const updated = { ...(controlled ? browserState : internalState), ...next };
    if (controlled) {
      onBrowserStateChange(updated);
    } else {
      setInternalState(updated);
    }
  }

  const [products, setProducts] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    let cancelled = false;
    getAllProducts()
      .then((data) => {
        if (!cancelled) {
          setProducts(data);
          setLoading(false);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setError(true);
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Reset to first page when page size or columns change.
  function handlePageSize(size: PageSize) {
    updateState({ pageSize: size, page: 0 });
  }

  function handleColumns(cols: Columns) {
    updateState({ columns: cols, page: 0 });
  }

  if (loading) {
    return (
      <div>
        <div className="browser-toolbar" aria-hidden="true">
          <span className="browser-toolbar__label">Explorar catálogo</span>
        </div>
        <div
          className={`browser-grid browser-grid--${columns}`}
          aria-hidden="true"
          data-testid="browser-skeleton"
        >
          {Array.from({ length: columns * 2 }).map((_, n) => (
            <div key={n} className="skeleton browser-skeleton-item" />
          ))}
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="empty-state">
        <p>No se pudieron cargar los productos.</p>
      </div>
    );
  }

  if (products.length === 0) {
    return (
      <div className="empty-state">
        <p>No hay productos en el catálogo.</p>
      </div>
    );
  }

  const totalPages = Math.ceil(products.length / pageSize);
  // Clamp page in case the list shrank after a reload.
  const safePage = Math.min(page, Math.max(0, totalPages - 1));
  const start = safePage * pageSize;
  const visibleProducts = products.slice(start, start + pageSize);

  return (
    <div className="product-browser">
      {/* Toolbar */}
      <div className="browser-toolbar">
        <span className="browser-toolbar__label">
          Explorar catálogo
          <span className="browser-toolbar__count"> ({products.length})</span>
        </span>

        <div className="browser-toolbar__controls">
          {/* Page-size selector */}
          <div className="browser-pagesize" role="group" aria-label="Productos por página">
            {PAGE_SIZES.map((size) => (
              <button
                key={size}
                className={`browser-pagesize__btn${pageSize === size ? ' browser-pagesize__btn--active' : ''}`}
                onClick={() => handlePageSize(size)}
                aria-pressed={pageSize === size}
              >
                {size}
              </button>
            ))}
          </div>

          {/* Layout toggle */}
          <div className="browser-layout-toggle" role="group" aria-label="Columnas del grid">
            <button
              className={`browser-layout-btn${columns === 3 ? ' browser-layout-btn--active' : ''}`}
              onClick={() => handleColumns(3)}
              aria-pressed={columns === 3}
              aria-label="3 columnas"
            >
              <GridIcon3 />
            </button>
            <button
              className={`browser-layout-btn${columns === 4 ? ' browser-layout-btn--active' : ''}`}
              onClick={() => handleColumns(4)}
              aria-pressed={columns === 4}
              aria-label="4 columnas"
            >
              <GridIcon4 />
            </button>
          </div>
        </div>
      </div>

      {/* Product grid */}
      <div
        className={`browser-grid browser-grid--${columns}`}
        role="list"
        data-testid="browser-grid"
      >
        {visibleProducts.map((product) => (
          <button
            key={product.id}
            className="browser-product-card"
            onClick={() => onSelectProduct(product.id)}
            role="listitem"
            aria-label={product.name}
          >
            <ProductImage productId={product.id} imageUrl={product.imageUrl} category={product.category} size="md" />
            <span className="browser-product-card__name">{product.name}</span>
            {product.category && (
              <span className="category">{product.category}</span>
            )}
            <span className="browser-product-card__price">
              {product.currentPrice.toFixed(2).replace('.', ',')} €
            </span>
          </button>
        ))}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="browser-pagination" role="navigation" aria-label="Paginación">
          <button
            className="browser-pagination__btn"
            onClick={() => updateState({ page: Math.max(0, safePage - 1) })}
            disabled={safePage === 0}
            aria-label="Página anterior"
          >
            <ChevronLeftIcon />
          </button>

          <span className="browser-pagination__info">
            {safePage + 1} / {totalPages}
          </span>

          <button
            className="browser-pagination__btn"
            onClick={() => updateState({ page: Math.min(totalPages - 1, safePage + 1) })}
            disabled={safePage === totalPages - 1}
            aria-label="Página siguiente"
          >
            <ChevronRightIcon />
          </button>
        </div>
      )}
    </div>
  );
}
