import { useState, useEffect } from 'react';
import type { AnalyticsResult, MostPurchasedProduct, PriceIncreaseProduct } from '../types';
import { getAnalytics } from '../api/products';
import ProductImage from './ProductImage';

interface AnalyticsProps {
  onSelectProduct: (id: string) => void;
}

function TrendUpIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
      strokeLinecap="round" strokeLinejoin="round" width="18" height="18" aria-hidden="true">
      <polyline points="23 6 13.5 15.5 8.5 10.5 1 18" />
      <polyline points="17 6 23 6 23 12" />
    </svg>
  );
}

function ShoppingBagIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
      strokeLinecap="round" strokeLinejoin="round" width="18" height="18" aria-hidden="true">
      <path d="M6 2 3 6v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6l-3-4z" />
      <line x1="3" y1="6" x2="21" y2="6" />
      <path d="M16 10a4 4 0 0 1-8 0" />
    </svg>
  );
}

function formatPrice(price: number): string {
  return price.toLocaleString('es-ES', { style: 'currency', currency: 'EUR' });
}

function formatPercent(value: number): string {
  return `+${value.toLocaleString('es-ES', { minimumFractionDigits: 1, maximumFractionDigits: 2 })}%`;
}

interface MostPurchasedRowProps {
  item: MostPurchasedProduct;
  rank: number;
  onSelectProduct: (id: string) => void;
}

function MostPurchasedRow({ item, rank, onSelectProduct }: MostPurchasedRowProps) {
  return (
    <li
      className="analytics-row"
      onClick={() => onSelectProduct(item.id)}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') onSelectProduct(item.id); }}
      aria-label={`Ver detalle de ${item.name}`}
    >
      <span className="analytics-row__rank">{rank}</span>
      <ProductImage productId={item.id} imageUrl={item.imageUrl} category={undefined} size="sm" />
      <span className="analytics-row__name">{item.name}</span>
      <span className="analytics-row__meta">
        <span className="analytics-row__badge analytics-row__badge--count">
          {item.purchaseCount} {item.purchaseCount === 1 ? 'vez' : 'veces'}
        </span>
        <span className="analytics-row__price">{formatPrice(item.currentPrice)}</span>
      </span>
    </li>
  );
}

interface PriceIncreaseRowProps {
  item: PriceIncreaseProduct;
  rank: number;
  onSelectProduct: (id: string) => void;
}

function PriceIncreaseRow({ item, rank, onSelectProduct }: PriceIncreaseRowProps) {
  return (
    <li
      className="analytics-row"
      onClick={() => onSelectProduct(item.id)}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') onSelectProduct(item.id); }}
      aria-label={`Ver detalle de ${item.name}`}
    >
      <span className="analytics-row__rank">{rank}</span>
      <ProductImage productId={item.id} imageUrl={item.imageUrl} category={undefined} size="sm" />
      <span className="analytics-row__name">{item.name}</span>
      <span className="analytics-row__meta">
        <span className="analytics-row__badge analytics-row__badge--increase">
          {formatPercent(item.increasePercent)}
        </span>
        <span className="analytics-row__price-range">
          <span>{formatPrice(item.firstPrice)}</span>
          <span className="analytics-row__arrow" aria-hidden="true">→</span>
          <span>{formatPrice(item.currentPrice)}</span>
        </span>
      </span>
    </li>
  );
}

export default function Analytics({ onSelectProduct }: AnalyticsProps) {
  const [data, setData] = useState<AnalyticsResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    getAnalytics()
      .then((result) => { if (!cancelled) { setData(result); setLoading(false); } })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Error cargando analítica');
          setLoading(false);
        }
      });
    return () => { cancelled = true; };
  }, []);

  if (loading) {
    return (
      <div className="analytics">
        <div className="analytics__loading" role="status" aria-busy="true" aria-label="Cargando analítica">
          <div className="analytics__skeleton-card" />
          <div className="analytics__skeleton-card" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="analytics">
        <p className="analytics__error" role="alert">{error}</p>
      </div>
    );
  }

  if (!data) return null;

  return (
    <div className="analytics">
      <section className="analytics__section" aria-labelledby="analytics-most-purchased-title">
        <header className="analytics__section-header">
          <span className="analytics__section-icon" aria-hidden="true"><ShoppingBagIcon /></span>
          <h2 className="analytics__section-title" id="analytics-most-purchased-title">
            Productos más comprados
          </h2>
        </header>
        {data.mostPurchased.length === 0 ? (
          <p className="analytics__empty">Aún no hay datos suficientes.</p>
        ) : (
          <ol className="analytics__list" aria-label="Productos más comprados">
            {data.mostPurchased.map((item, i) => (
              <MostPurchasedRow key={item.id} item={item} rank={i + 1} onSelectProduct={onSelectProduct} />
            ))}
          </ol>
        )}
      </section>

      <section className="analytics__section" aria-labelledby="analytics-price-increase-title">
        <header className="analytics__section-header">
          <span className="analytics__section-icon" aria-hidden="true"><TrendUpIcon /></span>
          <h2 className="analytics__section-title" id="analytics-price-increase-title">
            Mayor subida de precio
          </h2>
        </header>
        {data.biggestIncreases.length === 0 ? (
          <p className="analytics__empty">Aún no hay datos suficientes.</p>
        ) : (
          <ol className="analytics__list" aria-label="Productos con mayor subida de precio">
            {data.biggestIncreases.map((item, i) => (
              <PriceIncreaseRow key={item.id} item={item} rank={i + 1} onSelectProduct={onSelectProduct} />
            ))}
          </ol>
        )}
      </section>
    </div>
  );
}
