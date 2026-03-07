import { test, expect } from '@playwright/test';
import { stubProducts, stubEmptyProducts, loginViaStorage } from './helpers';

test.beforeEach(async ({ page }) => {
  await loginViaStorage(page);
  await stubProducts(page);
  await page.goto('/');
});

test('shows the product catalog on initial load', async ({ page }) => {
  await expect(page.getByText('LECHE ENTERA HACENDADO 1L')).toBeVisible();
  await expect(page.getByText('PAN DE MOLDE HACENDADO')).toBeVisible();
  await expect(page.getByText('YOGUR NATURAL HACENDADO')).toBeVisible();
});

test('shows formatted prices on the products', async ({ page }) => {
  await expect(page.getByText('0,89 €')).toBeVisible();
  await expect(page.getByText('1,35 €')).toBeVisible();
});

test('shows product categories', async ({ page }) => {
  const lacteosItems = page.getByText('Lácteos');
  await expect(lacteosItems.first()).toBeVisible();
});

test('the 4-column button changes the grid layout', async ({ page, isMobile }) => {
  test.skip(isMobile, 'El selector de columnas está oculto en móvil');
  const grid = page.locator('[data-testid="browser-grid"]');
  await page.getByRole('button', { name: '4 columnas' }).click();
  await expect(grid).toHaveClass(/browser-grid--4/);
});

test('the 3-column button restores the default layout', async ({ page, isMobile }) => {
  test.skip(isMobile, 'El selector de columnas está oculto en móvil');
  const grid = page.locator('[data-testid="browser-grid"]');
  await page.getByRole('button', { name: '4 columnas' }).click();
  await page.getByRole('button', { name: '3 columnas' }).click();
  await expect(grid).toHaveClass(/browser-grid--3/);
});

test('shows empty state when the API returns an empty list', async ({ page }) => {
  await stubEmptyProducts(page);
  await page.reload();
  await expect(page.getByText(/no hay productos/i)).toBeVisible();
});

test('pagination appears when there are more products than the page size', async ({ page }) => {
  // Generate 60 products to exceed default pageSize of 48
  const manyProducts = Array.from({ length: 60 }, (_, i) => ({
    id: `producto-${i}`,
    name: `PRODUCTO ${i}`,
    category: 'Varios',
    currentPrice: 1.0 + i * 0.01,
    imageUrl: null,
    lastPurchaseDate: '2025-09-01T00:00:00Z',
  }));
  await page.route('/api/products*', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(manyProducts),
    }),
  );
  await page.reload();
  await expect(page.getByRole('button', { name: /siguiente/i })).toBeVisible();
});

test('navigates to the next page when the next button is clicked', async ({ page }) => {
  const manyProducts = Array.from({ length: 60 }, (_, i) => ({
    id: `producto-${i}`,
    name: `PRODUCTO ${i}`,
    category: 'Varios',
    currentPrice: 1.0,
    imageUrl: null,
    lastPurchaseDate: '2025-09-01T00:00:00Z',
  }));
  await page.route('/api/products*', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(manyProducts),
    }),
  );
  await page.reload();
  await page.getByRole('button', { name: /siguiente/i }).click();
  await expect(page.getByRole('button', { name: /anterior/i })).toBeEnabled();
});
