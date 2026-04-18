import { describe, it, expect, vi, beforeEach } from 'vitest';
import { searchProducts, getAllProducts, getProduct, uploadTicket, uploadTickets, getAnalytics } from './products';
import type { SearchResult, Product, AnalyticsResult } from '../types';

const mockSearchResults: SearchResult[] = [
  { id: '1', name: 'LECHE ENTERA HACENDADO 1L', category: 'Lácteos', currentPrice: 0.89, minPrice: 0.79, maxPrice: 0.89 },
];

const mockProduct: Product = {
  id: '1',
  name: 'LECHE ENTERA HACENDADO 1L',
  category: 'Lácteos',
  currentPrice: 0.89,
  priceHistory: [
    { date: '2025-01-15T00:00:00Z', price: 0.79, store: 'Mercadona' },
    { date: '2025-09-22T00:00:00Z', price: 0.89, store: 'Mercadona' },
  ],
};

beforeEach(() => {
  vi.restoreAllMocks();
});

describe('searchProducts', () => {
  it('returns results when the response is OK', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockSearchResults),
    }));

    const results = await searchProducts('leche');
    expect(results).toEqual(mockSearchResults);
  });

  it('calls the correct endpoint with the encoded query', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([]),
    });
    vi.stubGlobal('fetch', fetchMock);

    await searchProducts('aceite oliva');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/products?q=aceite%20oliva',
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });

  it('throws an Error when the response is not OK', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      statusText: 'Internal Server Error',
    }));

    await expect(searchProducts('leche')).rejects.toThrow('Search failed: Internal Server Error');
  });

  it('returns an empty array when the server returns []', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([]),
    }));

    const results = await searchProducts('xyznonexistent');
    expect(results).toEqual([]);
  });
});

describe('getAllProducts', () => {
  it('calls the correct endpoint with an empty query', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockSearchResults),
    });
    vi.stubGlobal('fetch', fetchMock);

    await getAllProducts();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/products?q=',
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });

  it('returns all products from the response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockSearchResults),
    }));

    const results = await getAllProducts();
    expect(results).toEqual(mockSearchResults);
  });

  it('throws an Error when the response is not OK', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      statusText: 'Internal Server Error',
    }));

    await expect(getAllProducts()).rejects.toThrow('Search failed: Internal Server Error');
  });
});

describe('getProduct', () => {
  it('returns the product when the response is OK', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockProduct),
    }));

    const product = await getProduct('1');
    expect(product).toEqual(mockProduct);
  });

  it('calls the correct endpoint with the encoded ID', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockProduct),
    });
    vi.stubGlobal('fetch', fetchMock);

    await getProduct('42');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/products/42',
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });

  it('encodes special characters in the product ID', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockProduct),
    });
    vi.stubGlobal('fetch', fetchMock);

    await getProduct('leche entera');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/products/leche%20entera',
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });

  it('throws an Error when the product is not found', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      statusText: 'Not Found',
    }));

    await expect(getProduct('9999')).rejects.toThrow('Product not found: Not Found');
  });
});

describe('uploadTicket', () => {
  it('returns the result when the response is OK', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ invoiceNumber: '1234', linesImported: 5 }),
    }));

    const file = new File(['%PDF'], 'ticket.pdf', { type: 'application/pdf' });
    const result = await uploadTicket(file);
    expect(result).toEqual({ invoiceNumber: '1234', linesImported: 5 });
  });

  it('calls POST /api/tickets with multipart form data and X-Requested-With header', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ invoiceNumber: 'X', linesImported: 1 }),
    });
    vi.stubGlobal('fetch', fetchMock);

    const file = new File(['%PDF'], 'ticket.pdf', { type: 'application/pdf' });
    await uploadTicket(file);

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/tickets',
      expect.objectContaining({
        method: 'POST',
        body: expect.any(FormData),
        headers: expect.objectContaining({ 'X-Requested-With': 'XMLHttpRequest' }),
        signal: expect.any(AbortSignal),
      }),
    );
  });

  it('returns a friendly error message for 422 Unprocessable Entity', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 422,
      statusText: 'Unprocessable Entity',
      text: () => Promise.resolve('Unprocessable entity: could not parse PDF'),
    }));

    const file = new File(['bad'], 'ticket.pdf', { type: 'application/pdf' });
    await expect(uploadTicket(file)).rejects.toThrow('El PDF no es un ticket de Mercadona válido');
  });

  it('returns a friendly error message for 409 Conflict (duplicate file)', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      statusText: 'Conflict',
      text: () => Promise.resolve('Conflict: file already imported'),
    }));

    const file = new File(['%PDF'], 'ticket.pdf', { type: 'application/pdf' });
    await expect(uploadTicket(file)).rejects.toThrow('Este ticket ya fue importado anteriormente');
  });

  it('returns a friendly error message for 429 Too Many Requests', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 429,
      statusText: 'Too Many Requests',
      text: () => Promise.resolve('Too many requests'),
    }));

    const file = new File(['%PDF'], 'ticket.pdf', { type: 'application/pdf' });
    await expect(uploadTicket(file)).rejects.toThrow('Demasiadas solicitudes');
  });

  it('returns a friendly error message for 500 server errors', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      text: () => Promise.resolve('internal Go error details'),
    }));

    const file = new File(['%PDF'], 'ticket.pdf', { type: 'application/pdf' });
    await expect(uploadTicket(file)).rejects.toThrow('Error del servidor. Inténtalo de nuevo más tarde');
  });
});

describe('uploadTickets', () => {
  it('returns a summary with all items succeeded', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ invoiceNumber: 'A1', linesImported: 3 }),
    }));

    const files = [
      new File(['%PDF'], 'a.pdf', { type: 'application/pdf' }),
      new File(['%PDF'], 'b.pdf', { type: 'application/pdf' }),
    ];
    const summary = await uploadTickets(files);
    expect(summary.total).toBe(2);
    expect(summary.succeeded).toBe(2);
    expect(summary.failed).toBe(0);
    expect(summary.items.every((i) => i.ok)).toBe(true);
  });

  it('captures individual failures without aborting the batch', async () => {
    let callCount = 0;
    vi.stubGlobal('fetch', vi.fn().mockImplementation(() => {
      callCount++;
      if (callCount === 1) {
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve({ invoiceNumber: 'OK', linesImported: 2 }),
        });
      }
      return Promise.resolve({
        ok: false,
        status: 422,
        statusText: 'Unprocessable Entity',
        text: () => Promise.resolve('Unprocessable entity: could not parse'),
      });
    }));

    const files = [
      new File(['%PDF'], 'ok.pdf', { type: 'application/pdf' }),
      new File(['bad'], 'fail.pdf', { type: 'application/pdf' }),
    ];
    const summary = await uploadTickets(files);
    expect(summary.total).toBe(2);
    expect(summary.succeeded).toBe(1);
    expect(summary.failed).toBe(1);

    const okItem = summary.items.find((i) => i.file === 'ok.pdf');
    const failItem = summary.items.find((i) => i.file === 'fail.pdf');
    expect(okItem?.ok).toBe(true);
    expect(failItem?.ok).toBe(false);
    if (failItem && !failItem.ok) {
      // The message must be the friendly one, not the raw server message
      expect(failItem.error).toBe('El PDF no es un ticket de Mercadona válido');
    }
  });

  it('returns empty summary for empty file array', async () => {
    const summary = await uploadTickets([]);
    expect(summary.total).toBe(0);
    expect(summary.succeeded).toBe(0);
    expect(summary.failed).toBe(0);
    expect(summary.items).toHaveLength(0);
  });

  it('calls onProgress after each file completes', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ invoiceNumber: 'X', linesImported: 1 }),
    }));

    const files = [
      new File(['%PDF'], 'a.pdf', { type: 'application/pdf' }),
      new File(['%PDF'], 'b.pdf', { type: 'application/pdf' }),
      new File(['%PDF'], 'c.pdf', { type: 'application/pdf' }),
    ];
    const onProgress = vi.fn();
    await uploadTickets(files, onProgress);

    // Called once per file; total is always 3
    expect(onProgress).toHaveBeenCalledTimes(3);
    // Each call receives (done, 3) with done incrementing
    const totals = onProgress.mock.calls.map(([, total]) => total);
    expect(totals.every((t) => t === 3)).toBe(true);
    const dones = onProgress.mock.calls.map(([done]) => done);
    expect(dones).toContain(1);
    expect(dones).toContain(2);
    expect(dones).toContain(3);
  });

  it('calls onProgress even when a file fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 422,
      statusText: 'Unprocessable Entity',
      text: () => Promise.resolve('error'),
    }));

    const files = [new File(['bad'], 'fail.pdf', { type: 'application/pdf' })];
    const onProgress = vi.fn();
    await uploadTickets(files, onProgress);

    expect(onProgress).toHaveBeenCalledTimes(1);
    expect(onProgress).toHaveBeenCalledWith(1, 1);
  });
});

describe('getAnalytics', () => {
  const mockAnalytics: AnalyticsResult = {
    mostPurchased: [
      { id: 'leche-entera', name: 'LECHE ENTERA', purchaseCount: 5, currentPrice: 0.89 },
    ],
    biggestIncreases: [
      { id: 'aceite-oliva', name: 'ACEITE OLIVA', increasePercent: 42.5, firstPrice: 4.0, currentPrice: 5.7 },
    ],
    basketInflation: [
      { date: '2024-01-01', inflationPercent: 0.0, productsCount: 5, products: [] },
      { date: '2024-06-01', inflationPercent: 4.2, productsCount: 6, products: [] },
    ],
  };

  it('returns analytics data when the response is OK', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockAnalytics),
    }));

    const result = await getAnalytics();
    expect(result).toEqual(mockAnalytics);
  });

  it('calls GET /api/analytics with a signal', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockAnalytics),
    });
    vi.stubGlobal('fetch', fetchMock);

    await getAnalytics();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/analytics',
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
  });

  it('throws an Error when the response is not OK', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      statusText: 'Internal Server Error',
    }));

    await expect(getAnalytics()).rejects.toThrow('Analytics failed: Internal Server Error');
  });
});
