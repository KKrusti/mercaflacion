import { test, expect } from '@playwright/test';
import { stubProducts, stubProductDetail, loginViaStorage } from './helpers';

const PRODUCT_ID = 'leche-entera-hacendado-1l';

test.beforeEach(async ({ page }) => {
  await loginViaStorage(page);
  await stubProducts(page);
  await stubProductDetail(page, PRODUCT_ID);
  await page.goto('/');
  // Navigate to product detail by clicking on the product card
  await page.getByText('LECHE ENTERA HACENDADO 1L').first().click();
  await expect(page.locator('.product-detail')).toBeVisible({ timeout: 5000 });
});

test('shows the product name in the detail view', async ({ page }) => {
  await expect(page.getByRole('heading', { name: 'LECHE ENTERA HACENDADO 1L' })).toBeVisible();
});

test('shows the product category', async ({ page }) => {
  await expect(page.getByText('Lácteos')).toBeVisible();
});

test('shows the formatted current price', async ({ page }) => {
  const priceEl = page.locator('.detail-header .price');
  await expect(priceEl).toContainText('0,89 €');
});

test('shows the price history in the table', async ({ page }) => {
  await expect(page.getByRole('heading', { name: 'Historial de precios' })).toBeVisible();
  await expect(page.getByText('Mercadona').first()).toBeVisible();
});

test('shows the price chart', async ({ page }) => {
  await expect(page.getByText('Evolución del precio')).toBeVisible();
});

test('shows the price variation badge', async ({ page }) => {
  // 0.79 → 0.89 = +12.7%
  await expect(page.locator('.price-change-badge')).toBeVisible();
  await expect(page.locator('.price-change-badge')).toContainText('+12,7%');
});

test('the back button navigates back to the catalog', async ({ page }) => {
  await page.getByRole('button', { name: /volver a la búsqueda/i }).click();
  await expect(page.locator('.product-detail')).not.toBeVisible();
  await expect(page.locator('.browser-grid')).toBeVisible();
});

test('the app logo navigates to the home and shows the catalog', async ({ page }) => {
  await page.getByRole('button', { name: 'Ir a la página principal' }).click();
  await expect(page.locator('.product-detail')).not.toBeVisible();
  await expect(page.locator('.browser-grid')).toBeVisible();
});
