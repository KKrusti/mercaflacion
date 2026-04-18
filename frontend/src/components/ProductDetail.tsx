import { useState, useEffect, useRef } from 'react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import type { Product } from '../types';
import { getProduct, updateProductImage, deletePriceRecord, getAccumulatedIPC } from '../api/products';
import type { IPCResult } from '../api/products';
import ProductImage from './ProductImage';

interface ProductDetailProps {
  productId: string;
  onBack: () => void;
  token?: string | null;
}

interface ChartDataPoint {
  date: string;
  price: number;
  store: string;
}

function BackArrowIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <line x1="19" y1="12" x2="5" y2="12" />
      <polyline points="12 19 5 12 12 5" />
    </svg>
  );
}

function ArrowUpIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" width="12" height="12" aria-hidden="true">
      <polyline points="8 3 13 8 3 8" strokeLinejoin="round" />
      <line x1="8" y1="3" x2="8" y2="13" stroke="currentColor" strokeWidth="2" strokeLinecap="round" fill="none" />
    </svg>
  );
}

function ArrowDownIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" width="12" height="12" aria-hidden="true">
      <polyline points="8 13 13 8 3 8" strokeLinejoin="round" />
      <line x1="8" y1="13" x2="8" y2="3" stroke="currentColor" strokeWidth="2" strokeLinecap="round" fill="none" />
    </svg>
  );
}

function TrashIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" width="14" height="14">
      <polyline points="3 6 5 6 21 6" />
      <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" />
      <path d="M10 11v6M14 11v6" />
      <path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2" />
    </svg>
  );
}

function EditImageIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
      <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
    </svg>
  );
}

interface ImageEditorProps {
  productId: string;
  onSaved: (newUrl: string) => void;
}

// Matches Mercadona product page URLs, e.g.:
//   https://tienda.mercadona.es/product/60722/chocolate-negro-85-cacao-hacendado-tableta
const MERCADONA_PRODUCT_URL_RE = /^https?:\/\/tienda\.mercadona\.es\/products?\/\d+/i;

function ImageEditor({ productId, onSaved }: ImageEditorProps) {
  const [open, setOpen] = useState(false);
  const [url, setUrl] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [previewFailed, setPreviewFailed] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const trimmedUrl = url.trim();
  const isMercadonaProductUrl = MERCADONA_PRODUCT_URL_RE.test(trimmedUrl);
  // Show live preview only for direct image URLs, not for Mercadona product pages
  // (which are HTML pages and would always fail the image load check).
  const showPreview = trimmedUrl.startsWith('http') && !isMercadonaProductUrl;

  function handleOpen() {
    setOpen(true);
    setUrl('');
    setError(null);
    setPreviewFailed(false);
    setTimeout(() => inputRef.current?.focus(), 0);
  }

  function handleCancel() {
    setOpen(false);
    setError(null);
  }

  function handleUrlChange(e: React.ChangeEvent<HTMLInputElement>) {
    setUrl(e.target.value);
    setPreviewFailed(false);
    setError(null);
  }

  async function handleSave() {
    if (!trimmedUrl) {
      setError('Introduce una URL de imagen válida');
      return;
    }
    if (!isMercadonaProductUrl && previewFailed) {
      setError('La imagen no pudo cargarse. Comprueba que la URL apunta directamente a un archivo de imagen.');
      return;
    }
    setSaving(true);
    setError(null);
    try {
      // Backend resolves Mercadona product page URLs to the actual thumbnail URL.
      // Always use the URL returned by the server, not the input.
      const resolvedUrl = await updateProductImage(productId, trimmedUrl);
      onSaved(resolvedUrl);
      setOpen(false);
    } catch {
      setError('No se pudo guardar la imagen. Inténtalo de nuevo.');
    } finally {
      setSaving(false);
    }
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter') handleSave();
    if (e.key === 'Escape') handleCancel();
  }

  if (open) {
    return (
      <div className="image-editor image-editor--open">
        <input
          ref={inputRef}
          className="image-editor__input"
          type="url"
          placeholder="https://tienda.mercadona.es/product/60722/... o URL directa de imagen"
          value={url}
          onChange={handleUrlChange}
          onKeyDown={handleKeyDown}
          disabled={saving}
          aria-label="URL de imagen del producto"
        />
        {isMercadonaProductUrl && (
          <p className="image-editor__mercadona-hint">
            Enlace de Mercadona detectado — la imagen se obtendrá automáticamente del catálogo al guardar.
          </p>
        )}
        {showPreview && (
          <div className={`image-editor__preview${previewFailed ? ' image-editor__preview--error' : ''}`}>
            {previewFailed ? (
              <p className="image-editor__preview-msg">
                La imagen no carga desde esta URL. Asegúrate de que es un enlace directo a la imagen (termina en .jpg, .png, .webp…) o pega la URL del producto de Mercadona.
              </p>
            ) : (
              <img
                src={trimmedUrl}
                alt="Vista previa"
                className="image-editor__preview-img"
                onError={() => setPreviewFailed(true)}
              />
            )}
          </div>
        )}
        <div className="image-editor__actions">
          <button
            className="image-editor__save"
            onClick={handleSave}
            disabled={saving || (!isMercadonaProductUrl && previewFailed)}
            aria-label="Guardar imagen"
          >
            {saving ? 'Obteniendo imagen…' : 'Guardar'}
          </button>
          <button
            className="image-editor__cancel"
            onClick={handleCancel}
            disabled={saving}
            aria-label="Cancelar"
          >
            Cancelar
          </button>
        </div>
        {error && <p className="image-editor__error" role="alert">{error}</p>}
      </div>
    );
  }

  return (
    <button
      className="image-editor__trigger"
      onClick={handleOpen}
      aria-label="Cambiar imagen del producto"
      title="Cambiar imagen"
    >
      <EditImageIcon />
    </button>
  );
}

interface PriceChangeBadgeProps {
  firstPrice: number;
  currentPrice: number;
}

function PriceChangeBadge({ firstPrice, currentPrice }: PriceChangeBadgeProps) {
  if (firstPrice === 0) return null;
  const pct = ((currentPrice - firstPrice) / firstPrice) * 100;
  const isUp = pct > 0;
  const isDown = pct < 0;
  const modifier = isUp ? 'up' : isDown ? 'down' : 'flat';
  const label = isUp
    ? `Subida del ${pct.toFixed(1).replace('.', ',')}% desde el primer registro`
    : isDown
    ? `Bajada del ${Math.abs(pct).toFixed(1).replace('.', ',')}% desde el primer registro`
    : 'Sin variación desde el primer registro';

  return (
    <span
      className={`price-change-badge price-change-badge--${modifier}`}
      aria-label={label}
      title={label}
    >
      {isUp && <ArrowUpIcon />}
      {isDown && <ArrowDownIcon />}
      {pct === 0 ? '0%' : `${pct > 0 ? '+' : ''}${pct.toFixed(1).replace('.', ',')}%`}
    </span>
  );
}

function ProductDetailSkeleton({ onBack }: { onBack: () => void }) {
  return (
    <div className="product-detail" aria-busy="true" aria-label="Cargando producto">
      <button className="back-btn" onClick={onBack} aria-label="Volver a la búsqueda">
        <BackArrowIcon />
        Volver
      </button>
      <div className="detail-header">
        <div className="skeleton" style={{ width: 80, height: 80, borderRadius: 'var(--radius-md)', flexShrink: 0 }} />
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
          <div className="skeleton" style={{ height: 20, width: '70%', borderRadius: 'var(--radius-sm)' }} />
          <div className="skeleton" style={{ height: 14, width: '40%', borderRadius: 'var(--radius-sm)' }} />
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem', alignItems: 'flex-end' }}>
          <div className="skeleton" style={{ height: 26, width: 72, borderRadius: 'var(--radius-sm)' }} />
          <div className="skeleton" style={{ height: 14, width: 52, borderRadius: 'var(--radius-sm)' }} />
        </div>
      </div>
      <div className="skeleton" style={{ height: 80, marginTop: '1rem', borderRadius: 'var(--radius-md)' }} />
      <div className="skeleton" style={{ height: 280, marginTop: '1rem', borderRadius: 'var(--radius-md)' }} />
      <div className="skeleton" style={{ height: 180, marginTop: '1rem', borderRadius: 'var(--radius-md)' }} />
    </div>
  );
}

export default function ProductDetail({ productId, onBack, token }: ProductDetailProps) {
  const [product, setProduct] = useState<Product | null>(null);
  const [loading, setLoading] = useState(true);
  const [imageUrl, setImageUrl] = useState<string | undefined>(undefined);
  const [ipc, setIpc] = useState<IPCResult | null>(null);
  const [confirmingRecordId, setConfirmingRecordId] = useState<number | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    getProduct(productId)
      .then((data) => {
        if (!cancelled) {
          setProduct(data);
          setImageUrl(data.imageUrl);
        }
      })
      .catch((err) => { if (import.meta.env.DEV) console.error('Error loading product:', err); })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [productId]);

  useEffect(() => {
    if (!product || product.priceHistory.length === 0) return;
    let cancelled = false;
    const fromYear = new Date(product.priceHistory[0].date).getFullYear();
    getAccumulatedIPC(fromYear)
      .then((data) => { if (!cancelled) setIpc(data); })
      .catch(() => { /* IPC is optional context; silently ignore failures */ });
    return () => { cancelled = true; };
  }, [product]);

  async function handleDeleteRecord(recordId: number) {
    if (!product) return;
    setDeleting(true);
    setDeleteError(null);
    try {
      await deletePriceRecord(product.id, recordId);
      setConfirmingRecordId(null);
      setProduct((prev) => {
        if (!prev) return prev;
        const updated = {
          ...prev,
          priceHistory: prev.priceHistory.filter((r) => r.recordId !== recordId),
        };
        updated.currentPrice =
          updated.priceHistory.length > 0
            ? updated.priceHistory[updated.priceHistory.length - 1].price
            : 0;
        return updated;
      });
    } catch {
      setDeleteError('No se pudo eliminar el registro. Inténtalo de nuevo.');
    } finally {
      setDeleting(false);
    }
  }

  if (loading) {
    return <ProductDetailSkeleton onBack={onBack} />;
  }

  if (!product) {
    return (
      <div className="empty-state">
        <p>Producto no encontrado</p>
        <button className="back-btn" onClick={onBack}>
          <BackArrowIcon />
          Volver a la búsqueda
        </button>
      </div>
    );
  }

  const formatPrice = (price: number) =>
    price.toFixed(2).replace('.', ',') + ' \u20AC';

  const formatDateShort = (dateStr: string) => {
    const d = new Date(dateStr);
    return d.toLocaleDateString('es-ES', { month: 'short', year: '2-digit' });
  };

  const formatDateFull = (dateStr: string) => {
    const d = new Date(dateStr);
    return d.toLocaleDateString('es-ES', {
      day: 'numeric',
      month: 'long',
      year: 'numeric',
    });
  };

  const chartData: ChartDataPoint[] = product.priceHistory.map((record) => ({
    date: record.date,
    price: record.price,
    store: record.store || '',
  }));

  const prices = product.priceHistory.map((r) => r.price);
  const minPrice = Math.min(...prices);
  const maxPrice = Math.max(...prices);
  const yMin = Math.floor((minPrice - 0.1) * 10) / 10;
  const yMax = Math.ceil((maxPrice + 0.1) * 10) / 10;

  return (
    <div className="product-detail">
      <button className="back-btn" onClick={onBack} aria-label="Volver a la búsqueda">
        <BackArrowIcon />
        Volver
      </button>

      <div className="detail-header">
        <div className="detail-header__image-wrapper">
          <ProductImage
            productId={product.id}
            imageUrl={imageUrl}
            category={product.category}
            size="lg"
          />
          <ImageEditor
            productId={product.id}
            onSaved={(newUrl) => setImageUrl(newUrl)}
          />
        </div>
        <div className="detail-header__info">
          <h2>{product.name}</h2>
          {product.category && (
            <span className="category">{product.category}</span>
          )}
        </div>
        <div className="detail-header__price">
          <div className="price">{formatPrice(product.currentPrice)}</div>
          <div className="detail-header__price-label">precio actual</div>
          {product.priceHistory.length >= 2 && (
            <PriceChangeBadge
              firstPrice={product.priceHistory[0].price}
              currentPrice={product.priceHistory[product.priceHistory.length - 1].price}
            />
          )}
        </div>
      </div>

      {product.priceHistory.length >= 1 && (() => {
        const totalSpent = product.priceHistory.reduce((sum, r) => sum + r.price, 0);
        const maxRecord = product.priceHistory.length >= 2
          ? product.priceHistory.reduce((a, b) => b.price > a.price ? b : a)
          : null;
        return (
          <div className="detail-stats">
            <div className="detail-stat">
              <span className="detail-stat__label">Total gastado</span>
              <span className="detail-stat__value">{formatPrice(totalSpent)}</span>
              <span className="detail-stat__sub">{product.priceHistory.length} {product.priceHistory.length === 1 ? 'compra' : 'compras'}</span>
            </div>
            {maxRecord && (
              <div className="detail-stat">
                <span className="detail-stat__label">Precio máximo</span>
                <span className="detail-stat__value">{formatPrice(maxRecord.price)}</span>
                <span className="detail-stat__sub">{formatDateFull(maxRecord.date)}</span>
              </div>
            )}
            {ipc !== null && ipc.accumulated_rate !== 0 && (
              <div className="detail-stat">
                <span className="detail-stat__label">IPC Catalunya {ipc.from_year}–{ipc.to_year}</span>
                <span className="detail-stat__value">
                  {ipc.accumulated_rate > 0 ? '+' : ''}
                  {(ipc.accumulated_rate * 100).toFixed(1).replace('.', ',')}%
                </span>
                <span className="detail-stat__sub">inflación acumulada</span>
              </div>
            )}
          </div>
        );
      })()}

      <hr className="detail-divider" />

      <div className="chart-container">
        <h3 className="chart-container__title">Evolución del precio</h3>
        <ResponsiveContainer width="100%" height={280}>
          <LineChart data={chartData} margin={{ top: 5, right: 20, bottom: 5, left: 10 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
            <XAxis
              dataKey="date"
              tickFormatter={formatDateShort}
              tick={{ fontSize: 12, fill: 'var(--color-text-muted)' }}
              axisLine={{ stroke: 'var(--color-border)' }}
              tickLine={false}
            />
            <YAxis
              domain={[yMin, yMax]}
              tickFormatter={(v: number) => formatPrice(v)}
              tick={{ fontSize: 12, fill: 'var(--color-text-muted)' }}
              width={56}
              axisLine={false}
              tickLine={false}
            />
            <Tooltip
              formatter={(value: number) => [formatPrice(value), 'Precio']}
              labelFormatter={(label: string) => formatDateFull(label)}
              contentStyle={{
                borderRadius: '10px',
                border: '1.5px solid var(--color-border)',
                boxShadow: 'var(--shadow-md)',
                fontSize: '0.875rem',
              }}
            />
            <Line
              type="monotone"
              dataKey="price"
              stroke="var(--color-primary)"
              strokeWidth={2.5}
              dot={{ r: 4, fill: 'var(--color-primary)', strokeWidth: 0 }}
              activeDot={{ r: 6, fill: 'var(--color-primary-dark)', strokeWidth: 0 }}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>

      <hr className="detail-divider" />

      <div>
        <h3 className="price-table-section__title">Historial de precios</h3>
        {deleteError && (
          <p className="price-record-delete-error" role="alert">{deleteError}</p>
        )}
        <div className="price-table-wrapper">
          <table className="price-table">
            <thead>
              <tr>
                <th>Fecha</th>
                <th>Precio</th>
                <th>Tienda</th>
                {token && <th className="action-col"></th>}
              </tr>
            </thead>
            <tbody>
              {[...product.priceHistory].reverse().map((record, i) => {
                const isConfirming = record.recordId != null && confirmingRecordId === record.recordId;
                return (
                  <tr key={record.recordId ?? i} className={isConfirming ? 'price-record--confirming' : undefined}>
                    <td>{formatDateFull(record.date)}</td>
                    <td className="price-cell">{formatPrice(record.price)}</td>
                    <td className="store-cell">{record.store || '—'}</td>
                    {token && (
                      <td className="action-col">
                        {record.recordId != null && !isConfirming && (
                          <button
                            className="price-record-delete-btn"
                            onClick={() => { setConfirmingRecordId(record.recordId!); setDeleteError(null); }}
                            aria-label={`Eliminar registro de ${formatDateFull(record.date)}`}
                            title="Eliminar este registro"
                          >
                            <TrashIcon />
                          </button>
                        )}
                        {isConfirming && (
                          <span className="price-record-confirm">
                            <button
                              className="price-record-confirm__yes"
                              onClick={() => void handleDeleteRecord(record.recordId!)}
                              disabled={deleting}
                            >
                              {deleting ? '…' : 'Eliminar'}
                            </button>
                            <button
                              className="price-record-confirm__no"
                              onClick={() => setConfirmingRecordId(null)}
                              disabled={deleting}
                            >
                              Cancelar
                            </button>
                          </span>
                        )}
                      </td>
                    )}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
