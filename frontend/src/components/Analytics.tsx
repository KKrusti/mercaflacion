import { useState, useEffect } from 'react';
import {
  BarChart,
  Bar,
  Cell,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ReferenceLine,
  ResponsiveContainer,
} from 'recharts';
import type {
  AnalyticsResult,
  MostPurchasedProduct,
  PriceIncreaseProduct,
  BasketInflationPoint,
  BasketProductInflation,
} from '../types';
import { getAnalytics } from '../api/products';
import ProductImage from './ProductImage';

type SectionKey = 'mostPurchased' | 'biggestIncreases' | 'basketInflation';

interface AnalyticsProps {
  onSelectProduct: (id: string) => void;
}

// ── Icons ────────────────────────────────────────────────────────────────────

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

function InflationIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
      strokeLinecap="round" strokeLinejoin="round" width="18" height="18" aria-hidden="true">
      <path d="M3 3v18h18" />
      <path d="M7 16l4-4 4 4 4-4" />
    </svg>
  );
}

function ChevronIcon({ open }: { open: boolean }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
      strokeLinecap="round" strokeLinejoin="round" width="16" height="16"
      aria-hidden="true"
      style={{ transform: open ? 'rotate(180deg)' : 'rotate(0deg)', transition: 'transform 0.2s ease', flexShrink: 0 }}>
      <polyline points="6 9 12 15 18 9" />
    </svg>
  );
}

// ── Formatters ───────────────────────────────────────────────────────────────

function formatPrice(price: number): string {
  return price.toLocaleString('es-ES', { style: 'currency', currency: 'EUR' });
}

function formatPercent(value: number): string {
  return `+${value.toLocaleString('es-ES', { minimumFractionDigits: 1, maximumFractionDigits: 2 })}%`;
}

function formatInflation(value: number): string {
  const sign = value >= 0 ? '+' : '';
  return `${sign}${value.toLocaleString('es-ES', { minimumFractionDigits: 1, maximumFractionDigits: 2 })}%`;
}


// ── Section header (accordion trigger) ───────────────────────────────────────

interface SectionHeaderProps {
  icon: React.ReactNode;
  title: string;
  titleId: string;
  isOpen: boolean;
  onToggle: () => void;
}

function SectionHeader({ icon, title, titleId, isOpen, onToggle }: SectionHeaderProps) {
  return (
    <button
      className="analytics__section-header analytics__section-header--toggle"
      onClick={onToggle}
      aria-expanded={isOpen}
      aria-controls={`section-body-${titleId}`}
    >
      <span className="analytics__section-icon" aria-hidden="true">{icon}</span>
      <h2 className="analytics__section-title" id={titleId}>{title}</h2>
      <ChevronIcon open={isOpen} />
    </button>
  );
}

// ── Row components ────────────────────────────────────────────────────────────

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

// ── Basket inflation chart + breakdown ────────────────────────────────────────

type Granularity = 'monthly' | 'quarterly' | 'yearly';

type AggregatedPeriod = {
  key: string;
  label: string;
  value: number;
  ticketCount: number;
  tickets: BasketInflationPoint[];
};

function getPeriodKey(date: string, g: Granularity): string {
  const [year, month] = date.split('-').map(Number);
  if (g === 'monthly') return `${year}-${String(month).padStart(2, '0')}`;
  if (g === 'quarterly') return `${year}-Q${Math.ceil(month / 3)}`;
  return String(year);
}

function getPeriodLabel(key: string, g: Granularity): string {
  if (g === 'monthly') {
    const [year, month] = key.split('-').map(Number);
    return new Date(year, month - 1, 1).toLocaleDateString('es-ES', { month: 'short', year: '2-digit' });
  }
  if (g === 'quarterly') {
    const [year, q] = key.split('-');
    return `${q} '${year.slice(2)}`;
  }
  return key;
}

function aggregateByPeriod(points: BasketInflationPoint[], g: Granularity): AggregatedPeriod[] {
  const groups = new Map<string, BasketInflationPoint[]>();
  for (const p of points) {
    const key = getPeriodKey(p.date, g);
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key)!.push(p);
  }
  return Array.from(groups.entries()).map(([key, tickets]) => {
    const mean = tickets.reduce((s, t) => s + t.inflationPercent, 0) / tickets.length;
    return { key, label: getPeriodLabel(key, g), value: Math.round(mean * 100) / 100, ticketCount: tickets.length, tickets };
  });
}

function formatShortDate(dateStr: string): string {
  const [year, month, day] = dateStr.split('-').map(Number);
  return new Date(year, month - 1, day).toLocaleDateString('es-ES', { day: 'numeric', month: 'short', year: '2-digit' });
}

interface ProductBreakdownRowProps {
  item: BasketProductInflation;
  rank: number;
  onSelectProduct: (id: string) => void;
}

function ProductBreakdownRow({ item, rank, onSelectProduct }: ProductBreakdownRowProps) {
  const modifier = item.inflationPercent > 0 ? 'increase' : item.inflationPercent < 0 ? 'decrease' : 'flat';
  return (
    <li
      className="analytics-row"
      onClick={() => onSelectProduct(item.productId)}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') onSelectProduct(item.productId); }}
      aria-label={`Ver detalle de ${item.productName}`}
    >
      <span className="analytics-row__rank">{rank}</span>
      <ProductImage productId={item.productId} imageUrl={item.imageUrl} category={undefined} size="sm" />
      <span className="analytics-row__name">{item.productName}</span>
      <span className="analytics-row__meta">
        <span className={`analytics-row__badge analytics-row__badge--${modifier}`}>
          {formatInflation(item.inflationPercent)}
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

interface BasketInflationChartProps {
  points: BasketInflationPoint[];
  onSelectProduct: (id: string) => void;
}

function BasketInflationChart({ points, onSelectProduct }: BasketInflationChartProps) {
  const [granularity, setGranularity] = useState<Granularity>('monthly');
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [selectedTicketDate, setSelectedTicketDate] = useState<string | null>(null);

  const periods = aggregateByPeriod(points, granularity);
  const latest = periods[periods.length - 1];

  const selectedPeriod = selectedKey ? periods.find((p) => p.key === selectedKey) ?? null : null;
  const selectedTicket = selectedTicketDate
    ? selectedPeriod?.tickets.find((t) => t.date === selectedTicketDate) ?? null
    : null;
  const sortedProducts = selectedTicket
    ? [...selectedTicket.products].sort((a, b) => b.inflationPercent - a.inflationPercent)
    : null;

  const handleGranularityChange = (g: Granularity) => {
    setGranularity(g);
    setSelectedKey(null);
    setSelectedTicketDate(null);
  };

  const xAxisHeight = granularity === 'monthly' ? 52 : 24;

  return (
    <>
      <div className="analytics__inflation-header">
        <div className="analytics__inflation-summary">
          <span className="analytics__inflation-label">Media periodo más reciente</span>
          <span className={`analytics__inflation-badge analytics__inflation-badge--${latest.value >= 0 ? 'up' : 'down'}`}>
            {formatInflation(latest.value)}
          </span>
        </div>
        <div className="analytics__period-selector" role="group" aria-label="Seleccionar periodo">
          {(['monthly', 'quarterly', 'yearly'] as Granularity[]).map((g) => (
            <button
              key={g}
              className={`analytics__period-btn${granularity === g ? ' analytics__period-btn--active' : ''}`}
              onClick={() => handleGranularityChange(g)}
              aria-pressed={granularity === g}
            >
              {g === 'monthly' ? 'Mensual' : g === 'quarterly' ? 'Trimestral' : 'Anual'}
            </button>
          ))}
        </div>
      </div>

      <div className="chart-container" aria-label="Gráfico de inflación de la cesta">
        <ResponsiveContainer width="100%" height={230}>
          <BarChart data={periods} margin={{ top: 8, right: 16, bottom: 4, left: 4 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" vertical={false} />
            <XAxis
              dataKey="label"
              tick={granularity === 'monthly'
                ? ({ x, y, payload }: { x: number; y: number; payload: { value: string } }) => (
                    <g transform={`translate(${x},${y})`}>
                      <text x={0} y={0} dy={4} textAnchor="end" fill="#64748b" fontSize={11} transform="rotate(-35)">
                        {payload.value}
                      </text>
                    </g>
                  )
                : { fontSize: 11, fill: '#64748b' }
              }
              height={xAxisHeight}
              tickLine={false}
              axisLine={false}
              interval={0}
            />
            <YAxis
              tickFormatter={(v: number) => `${v > 0 ? '+' : ''}${v.toFixed(0)}%`}
              tick={{ fontSize: 11, fill: '#64748b' }}
              tickLine={false}
              axisLine={false}
              width={40}
            />
            <Tooltip
              cursor={{ fill: 'rgba(100,116,139,0.08)' }}
              formatter={(value: number, _name: string, entry: { payload?: AggregatedPeriod }) => [
                formatInflation(value),
                `Media inflación (${entry.payload?.ticketCount ?? 0} tiquet${(entry.payload?.ticketCount ?? 0) !== 1 ? 's' : ''})`,
              ]}
              labelFormatter={(label: string) => label}
              contentStyle={{ fontSize: '0.82rem', borderRadius: '6px' }}
            />
            <ReferenceLine y={0} stroke="#94a3b8" strokeDasharray="4 2" />
            <Bar
              dataKey="value"
              radius={[3, 3, 0, 0]}
              cursor="pointer"
              maxBarSize={56}
              onClick={(data: AggregatedPeriod) => {
                setSelectedKey((prev) => (prev === data.key ? null : data.key));
                setSelectedTicketDate(null);
              }}
            >
              {periods.map((entry) => (
                <Cell
                  key={entry.key}
                  fill={entry.key === selectedKey
                    ? (entry.value >= 0 ? '#991b1b' : '#166534')
                    : (entry.value >= 0 ? '#dc2626' : '#16a34a')}
                  opacity={selectedKey && entry.key !== selectedKey ? 0.35 : 1}
                />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </div>

      {selectedPeriod && (
        <div className="analytics__breakdown" aria-label={`Tiquets de ${selectedPeriod.label}`}>
          <div className="analytics__breakdown-header">
            <span className="analytics__breakdown-title">Tiquets de {selectedPeriod.label}</span>
            <button
              className="analytics__breakdown-close"
              onClick={() => { setSelectedKey(null); setSelectedTicketDate(null); }}
              aria-label="Cerrar desglose"
            >
              ✕
            </button>
          </div>
          <ul className="analytics__ticket-list" aria-label="Tiquets del periodo">
            {selectedPeriod.tickets.map((ticket) => {
              const isSelected = selectedTicketDate === ticket.date;
              const mod = ticket.inflationPercent >= 0 ? 'up' : 'down';
              return (
                <li key={ticket.date}>
                  <button
                    className={`analytics__ticket-row${isSelected ? ' analytics__ticket-row--open' : ''}`}
                    onClick={() => setSelectedTicketDate((prev) => (prev === ticket.date ? null : ticket.date))}
                    aria-expanded={isSelected}
                  >
                    <span className="analytics__ticket-date">{formatShortDate(ticket.date)}</span>
                    <span className={`analytics__inflation-badge analytics__inflation-badge--${mod}`}>
                      {formatInflation(ticket.inflationPercent)}
                    </span>
                    <span className="analytics__ticket-count">{ticket.productsCount} productos</span>
                    <ChevronIcon open={isSelected} />
                  </button>
                  {isSelected && sortedProducts && (
                    <ol className="analytics__list analytics__ticket-products" aria-label="Productos del tiquet">
                      {sortedProducts.map((item, i) => (
                        <ProductBreakdownRow
                          key={item.productId}
                          item={item}
                          rank={i + 1}
                          onSelectProduct={onSelectProduct}
                        />
                      ))}
                    </ol>
                  )}
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </>
  );
}

// ── Main component ────────────────────────────────────────────────────────────

export default function Analytics({ onSelectProduct }: AnalyticsProps) {
  const [data, setData] = useState<AnalyticsResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [openSection, setOpenSection] = useState<SectionKey>('mostPurchased');

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

  const toggle = (key: SectionKey) =>
    setOpenSection((prev) => (prev === key ? key : key));

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

  const sections: { key: SectionKey; icon: React.ReactNode; title: string; titleId: string; content: React.ReactNode }[] = [
    {
      key: 'mostPurchased',
      icon: <ShoppingBagIcon />,
      title: 'Productos más comprados',
      titleId: 'analytics-most-purchased-title',
      content: data.mostPurchased.length === 0 ? (
        <p className="analytics__empty">Aún no hay datos suficientes.</p>
      ) : (
        <ol className="analytics__list" aria-label="Productos más comprados">
          {data.mostPurchased.map((item, i) => (
            <MostPurchasedRow key={item.id} item={item} rank={i + 1} onSelectProduct={onSelectProduct} />
          ))}
        </ol>
      ),
    },
    {
      key: 'biggestIncreases',
      icon: <TrendUpIcon />,
      title: 'Mayor subida de precio',
      titleId: 'analytics-price-increase-title',
      content: data.biggestIncreases.length === 0 ? (
        <p className="analytics__empty">Aún no hay datos suficientes.</p>
      ) : (
        <ol className="analytics__list" aria-label="Productos con mayor subida de precio">
          {data.biggestIncreases.map((item, i) => (
            <PriceIncreaseRow key={item.id} item={item} rank={i + 1} onSelectProduct={onSelectProduct} />
          ))}
        </ol>
      ),
    },
    {
      key: 'basketInflation',
      icon: <InflationIcon />,
      title: 'Inflación de tu cesta',
      titleId: 'analytics-basket-inflation-title',
      content: data.basketInflation.length < 2 ? (
        <p className="analytics__empty">Aún no hay datos suficientes.</p>
      ) : (
        <BasketInflationChart points={data.basketInflation} onSelectProduct={onSelectProduct} />
      ),
    },
  ];

  return (
    <div className="analytics">
      {sections.map(({ key, icon, title, titleId, content }) => {
        const isOpen = openSection === key;
        return (
          <section key={key} className="analytics__section" aria-labelledby={titleId}>
            <SectionHeader
              icon={icon}
              title={title}
              titleId={titleId}
              isOpen={isOpen}
              onToggle={() => toggle(key)}
            />
            <div
              id={`section-body-${titleId}`}
              className={`analytics__section-body${isOpen ? ' analytics__section-body--open' : ''}`}
            >
              <div>{content}</div>
            </div>
          </section>
        );
      })}
    </div>
  );
}
