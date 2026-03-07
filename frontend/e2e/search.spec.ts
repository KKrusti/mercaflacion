import { test, expect } from '@playwright/test';
import { stubProducts, loginViaStorage } from './helpers';

test.beforeEach(async ({ page }) => {
  await loginViaStorage(page);
  await stubProducts(page);
  await page.goto('/');
});

test('the search bar is visible', async ({ page }) => {
  await expect(page.getByPlaceholder(/buscar producto/i)).toBeVisible();
});

test('typing in the search box hides the catalog and shows results', async ({ page }) => {
  // Stub the search response
  await page.route('/api/products?q=leche', (route) =>
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
        },
      ]),
    }),
  );
  const searchInput = page.getByPlaceholder(/buscar producto/i);
  await searchInput.fill('leche');
  // Wait for debounce + result
  await expect(page.getByText('LECHE ENTERA HACENDADO 1L')).toBeVisible({ timeout: 2000 });
  // ProductBrowser should be hidden
  await expect(page.locator('.browser-grid')).not.toBeVisible();
});

test('shows "Sin resultados" when the search finds nothing', async ({ page }) => {
  await page.route('/api/products?q=xyz', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }),
  );
  await page.getByPlaceholder(/buscar producto/i).fill('xyz');
  await expect(page.getByText(/sin resultados/i)).toBeVisible({ timeout: 2000 });
});

test('clearing the search box shows the catalog again', async ({ page }) => {
  const searchInput = page.getByPlaceholder(/buscar producto/i);
  await searchInput.fill('leche');
  await searchInput.clear();
  await expect(page.locator('.browser-grid')).toBeVisible({ timeout: 2000 });
});

test('clicking a result navigates to the product detail page', async ({ page }) => {
  await page.route('/api/products?q=leche', (route) =>
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
        },
      ]),
    }),
  );
  await page.route('/api/products/leche-entera-hacendado-1l', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        id: 'leche-entera-hacendado-1l',
        name: 'LECHE ENTERA HACENDADO 1L',
        category: 'Lácteos',
        currentPrice: 0.89,
        imageUrl: null,
        priceHistory: [
          { date: '2025-09-22T00:00:00Z', price: 0.89, store: 'Mercadona' },
        ],
      }),
    }),
  );
  await page.getByPlaceholder(/buscar producto/i).fill('leche');
  await page.getByText('LECHE ENTERA HACENDADO 1L').first().click();
  await expect(page.locator('.product-detail')).toBeVisible({ timeout: 3000 });
});

test('the back button from detail returns to the search state', async ({ page }) => {
  await page.route('/api/products?q=leche', (route) =>
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
        },
      ]),
    }),
  );
  await page.route('/api/products/leche-entera-hacendado-1l', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        id: 'leche-entera-hacendado-1l',
        name: 'LECHE ENTERA HACENDADO 1L',
        category: 'Lácteos',
        currentPrice: 0.89,
        imageUrl: null,
        priceHistory: [{ date: '2025-09-22T00:00:00Z', price: 0.89, store: 'Mercadona' }],
      }),
    }),
  );
  await page.getByPlaceholder(/buscar producto/i).fill('leche');
  await page.getByText('LECHE ENTERA HACENDADO 1L').first().click();
  await expect(page.locator('.product-detail')).toBeVisible({ timeout: 3000 });
  await page.getByRole('button', { name: /volver a la búsqueda/i }).click();
  await expect(page.locator('.product-detail')).not.toBeVisible();
});
