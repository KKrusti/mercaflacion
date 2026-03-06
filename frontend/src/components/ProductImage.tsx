import { useState, useEffect } from 'react';
import { getProductImageUrl, getCategoryEmoji } from '../utils/productImages';

interface ProductImageProps {
  productId: string;
  imageUrl?: string;
  category: string | undefined;
  size?: 'sm' | 'md' | 'lg';
}

function BrokenImageIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"
      strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="3" y="3" width="18" height="18" rx="2" />
      <path d="M3 16l5-5 4 4 3-3 6 6" />
      <line x1="2" y1="2" x2="22" y2="22" stroke="currentColor" strokeWidth="1.5" />
    </svg>
  );
}

export default function ProductImage({ productId, imageUrl, category, size = 'md' }: ProductImageProps) {
  // Priority: imageUrl from backend → static map fallback → category emoji
  const url = imageUrl || getProductImageUrl(productId);
  const [imgFailed, setImgFailed] = useState(false);

  // Reset failure state whenever the URL changes (e.g. after a manual image update).
  useEffect(() => {
    setImgFailed(false);
  }, [url]);

  const sizeClass = size === 'sm' ? 'product-img-sm' : size === 'lg' ? 'product-img-lg' : 'product-img-md';

  // imageUrl was explicitly set but failed to load → show broken indicator
  if (imgFailed && imageUrl) {
    return (
      <div
        className={`product-img-broken ${sizeClass}`}
        title="La imagen guardada no pudo cargarse"
        aria-label="Imagen no disponible"
      >
        <BrokenImageIcon />
      </div>
    );
  }

  // No URL at all → show category emoji fallback
  if (!url || imgFailed) {
    return (
      <div className={`product-img-emoji ${sizeClass}`} aria-hidden="true">
        {getCategoryEmoji(category)}
      </div>
    );
  }

  return (
    <img
      src={url}
      alt=""
      className={`product-img ${sizeClass}`}
      onError={() => setImgFailed(true)}
      loading="lazy"
    />
  );
}
