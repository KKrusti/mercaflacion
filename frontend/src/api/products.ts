import type { SearchResult, Product, TicketUploadResult, TicketUploadSummary, AnalyticsResult } from '../types';

const API_BASE = '/api';
const AUTH_STORAGE_KEY = 'mercaflacion_auth';

const READ_TIMEOUT_MS = 15_000;
const UPLOAD_TIMEOUT_MS = 60_000;

function authHeaders(): Record<string, string> {
  try {
    const raw = localStorage.getItem(AUTH_STORAGE_KEY);
    if (raw) {
      const { token } = JSON.parse(raw) as { token?: string };
      if (token) return { Authorization: `Bearer ${token}` };
    }
  } catch {
    // ignore corrupt storage
  }
  return {};
}

// Returns an AbortSignal that fires after `ms` milliseconds, plus a cleanup function.
function withTimeout(ms: number): { signal: AbortSignal; clear: () => void } {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), ms);
  return { signal: controller.signal, clear: () => clearTimeout(timer) };
}

// Maps HTTP error status codes to user-friendly messages,
// avoiding exposure of internal server error details.
function friendlyUploadError(status: number, body: string): string {
  if (status === 409 || body.toLowerCase().includes('already imported')) {
    return 'Este ticket ya fue importado anteriormente';
  }
  if (status === 413) return 'El archivo supera el tamaño máximo permitido (10 MB)';
  if (status === 422) return 'El PDF no es un ticket de Mercadona válido';
  if (status === 429) return 'Demasiadas solicitudes. Espera unos segundos e inténtalo de nuevo';
  if (status >= 500) return 'Error del servidor. Inténtalo de nuevo más tarde';
  return 'No se pudo procesar el ticket. Inténtalo de nuevo';
}

export async function searchProducts(query: string): Promise<SearchResult[]> {
  const { signal, clear } = withTimeout(READ_TIMEOUT_MS);
  try {
    const res = await fetch(`${API_BASE}/products?q=${encodeURIComponent(query)}`, {
      signal,
      headers: authHeaders(),
    });
    if (!res.ok) throw new Error(`Search failed: ${res.statusText}`);
    return res.json();
  } finally {
    clear();
  }
}

export async function getAllProducts(): Promise<SearchResult[]> {
  return searchProducts('');
}

export async function getProduct(id: string): Promise<Product> {
  const { signal, clear } = withTimeout(READ_TIMEOUT_MS);
  try {
    const res = await fetch(`${API_BASE}/products/${encodeURIComponent(id)}`, {
      signal,
      headers: authHeaders(),
    });
    if (!res.ok) throw new Error(`Product not found: ${res.statusText}`);
    return res.json();
  } finally {
    clear();
  }
}

export async function uploadTicket(file: File): Promise<TicketUploadResult> {
  const form = new FormData();
  form.append('file', file);
  const { signal, clear } = withTimeout(UPLOAD_TIMEOUT_MS);
  try {
    const res = await fetch(`${API_BASE}/tickets`, {
      method: 'POST',
      body: form,
      headers: { 'X-Requested-With': 'XMLHttpRequest', ...authHeaders() },
      signal,
    });
    if (!res.ok) {
      const body = await res.text().catch(() => '');
      throw new Error(friendlyUploadError(res.status, body));
    }
    return res.json();
  } finally {
    clear();
  }
}

// Uploads multiple files concurrently. Individual failures are captured in the
// summary without aborting the batch. onProgress is called after each file completes.
export async function uploadTickets(
  files: File[],
  onProgress?: (done: number, total: number) => void,
): Promise<TicketUploadSummary> {
  let done = 0;
  const total = files.length;

  const results = await Promise.all(
    files.map(async (file) => {
      try {
        const result = await uploadTicket(file);
        onProgress?.(++done, total);
        return { file: file.name, ok: true as const, result };
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        onProgress?.(++done, total);
        return { file: file.name, ok: false as const, error: message };
      }
    }),
  );

  return {
    total: results.length,
    succeeded: results.filter((r) => r.ok).length,
    failed: results.filter((r) => !r.ok).length,
    items: results,
  };
}

// Returns the resolved image URL saved by the backend (may differ from the
// input when the backend auto-resolves a Mercadona product page URL).
export async function updateProductImage(id: string, imageUrl: string): Promise<string> {
  const { signal, clear } = withTimeout(READ_TIMEOUT_MS);
  try {
    const res = await fetch(`${API_BASE}/products/${encodeURIComponent(id)}/image`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: JSON.stringify({ imageUrl }),
      signal,
    });
    if (!res.ok) {
      const body = await res.text().catch(() => '');
      throw new Error(body || `Update image failed: ${res.statusText}`);
    }
    const data = await res.json() as { imageUrl: string };
    return data.imageUrl;
  } finally {
    clear();
  }
}

export async function deletePriceRecord(productId: string, recordId: number): Promise<void> {
  const { signal, clear } = withTimeout(READ_TIMEOUT_MS);
  try {
    const res = await fetch(
      `${API_BASE}/products/${encodeURIComponent(productId)}/prices/${recordId}`,
      { method: 'DELETE', headers: authHeaders(), signal },
    );
    if (!res.ok) {
      const body = await res.text().catch(() => '');
      throw new Error(body || `Delete failed: ${res.statusText}`);
    }
  } finally {
    clear();
  }
}

export interface IPCResult {
  from_year: number;
  to_year: number;
  accumulated_rate: number;
}

export async function getAccumulatedIPC(fromYear: number): Promise<IPCResult> {
  const { signal, clear } = withTimeout(READ_TIMEOUT_MS);
  try {
    const res = await fetch(`${API_BASE}/ipc?from=${fromYear}`, { signal });
    if (!res.ok) throw new Error(`IPC fetch failed: ${res.statusText}`);
    return await res.json() as IPCResult;
  } finally {
    clear();
  }
}

export async function getAnalytics(): Promise<AnalyticsResult> {
  const { signal, clear } = withTimeout(READ_TIMEOUT_MS);
  try {
    const res = await fetch(`${API_BASE}/analytics`, {
      signal,
      headers: authHeaders(),
    });
    if (!res.ok) throw new Error(`Analytics failed: ${res.statusText}`);
    return res.json();
  } finally {
    clear();
  }
}
