import type { Page } from '@playwright/test';

const AUTH_STORAGE_KEY = 'mercaflacion_auth';

// Injects a fake authenticated session into localStorage via addInitScript so
// React reads it before mounting. Must be called before page.goto().
export async function loginViaStorage(page: Page) {
  await page.addInitScript((key) => {
    localStorage.setItem(key, JSON.stringify({
      user: { userId: 1, username: 'testuser' },
      token: 'test-token',
    }));
  }, AUTH_STORAGE_KEY);
}

// Stubs the /api/products endpoint with an empty array by default.
export async function stubEmptyProducts(page: Page) {
  await page.route('/api/products*', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }),
  );
}

// Stubs /api/products with a small product list.
export async function stubProducts(page: Page) {
  await page.route('/api/products*', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify([
        {
          id: 'leche-entera-hacendado-1l',
          name: 'LECHE ENTERA HACENDADO 1L',
          category: 'Lácteos',
          currentPrice: 0.89,
          imageUrl: null,
          lastPurchaseDate: '2025-09-22T00:00:00Z',
        },
        {
          id: 'pan-de-molde-hacendado',
          name: 'PAN DE MOLDE HACENDADO',
          category: 'Panadería',
          currentPrice: 1.35,
          imageUrl: null,
          lastPurchaseDate: '2025-09-20T00:00:00Z',
        },
        {
          id: 'yogur-natural-hacendado',
          name: 'YOGUR NATURAL HACENDADO',
          category: 'Lácteos',
          currentPrice: 0.45,
          imageUrl: null,
          lastPurchaseDate: '2025-09-18T00:00:00Z',
        },
      ]),
    }),
  );
}

// Stubs /api/products/<id> with a single product detail.
export async function stubProductDetail(page: Page, productId: string) {
  await page.route(`/api/products/${productId}`, (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        id: productId,
        name: 'LECHE ENTERA HACENDADO 1L',
        category: 'Lácteos',
        currentPrice: 0.89,
        imageUrl: null,
        priceHistory: [
          { date: '2025-01-15T00:00:00Z', price: 0.79, store: 'Mercadona' },
          { date: '2025-09-22T00:00:00Z', price: 0.89, store: 'Mercadona' },
        ],
      }),
    }),
  );
}

// Stubs auth endpoints.
export async function stubAuth(page: Page) {
  await page.route('/api/auth/register', (route) =>
    route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({ token: 'test-token', userId: 1, username: 'testuser' }),
    }),
  );
  await page.route('/api/auth/login', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ token: 'test-token', userId: 1, username: 'testuser' }),
    }),
  );
}

// Stubs /api/analytics.
export async function stubAnalytics(page: Page) {
  await page.route('/api/analytics', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        mostPurchased: [
          { id: 'leche-entera-hacendado-1l', name: 'LECHE ENTERA HACENDADO 1L', purchaseCount: 12, currentPrice: 0.89, imageUrl: null },
          { id: 'pan-de-molde-hacendado', name: 'PAN DE MOLDE HACENDADO', purchaseCount: 8, currentPrice: 1.35, imageUrl: null },
        ],
        biggestIncreases: [
          { id: 'yogur-natural-hacendado', name: 'YOGUR NATURAL HACENDADO', firstPrice: 0.35, currentPrice: 0.45, increasePercent: 28.57 },
        ],
      }),
    }),
  );
}
