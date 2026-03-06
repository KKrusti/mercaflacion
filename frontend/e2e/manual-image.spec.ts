import { test, expect } from '@playwright/test';
import { stubProducts, stubProductDetail } from './helpers';

const PRODUCT_ID = 'leche-entera-hacendado-1l';
const NEW_IMAGE_URL = 'https://prod.mercadona.com/images/leche.jpg';

test.beforeEach(async ({ page }) => {
  await stubProducts(page);
  await stubProductDetail(page, PRODUCT_ID);
  await page.goto('/');
  await page.getByText('LECHE ENTERA HACENDADO 1L').first().click();
  await expect(page.locator('.product-detail')).toBeVisible({ timeout: 5000 });
});

test('the edit image button is visible in the product detail', async ({ page }) => {
  await expect(
    page.getByRole('button', { name: 'Cambiar imagen del producto' }),
  ).toBeVisible();
});

test('pressing the button shows the URL input', async ({ page }) => {
  await page.getByRole('button', { name: 'Cambiar imagen del producto' }).click();
  await expect(page.getByLabel('URL de imagen del producto')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Guardar imagen' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Cancelar' })).toBeVisible();
});

test('cancelling hides the input and shows the edit button again', async ({ page }) => {
  await page.getByRole('button', { name: 'Cambiar imagen del producto' }).click();
  await page.getByRole('button', { name: 'Cancelar' }).click();
  await expect(page.getByLabel('URL de imagen del producto')).not.toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Cambiar imagen del producto' }),
  ).toBeVisible();
});

test('saving calls PATCH and closes the input', async ({ page }) => {
  let patchCalled = false;
  await page.route(`/api/products/${PRODUCT_ID}/image`, (route) => {
    patchCalled = true;
    route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
  });

  await page.getByRole('button', { name: 'Cambiar imagen del producto' }).click();
  await page.getByLabel('URL de imagen del producto').fill(NEW_IMAGE_URL);
  await page.getByRole('button', { name: 'Guardar imagen' }).click();

  await expect(page.getByLabel('URL de imagen del producto')).not.toBeVisible({ timeout: 3000 });
  expect(patchCalled).toBe(true);
});

test('shows an error if the server returns an error when saving', async ({ page }) => {
  await page.route(`/api/products/${PRODUCT_ID}/image`, (route) =>
    route.fulfill({ status: 500, body: 'Internal Server Error' }),
  );

  await page.getByRole('button', { name: 'Cambiar imagen del producto' }).click();
  await page.getByLabel('URL de imagen del producto').fill(NEW_IMAGE_URL);
  await page.getByRole('button', { name: 'Guardar imagen' }).click();

  await expect(page.getByRole('alert')).toContainText(/No se pudo guardar/i);
});

test('shows validation error when trying to save with an empty URL', async ({ page }) => {
  await page.getByRole('button', { name: 'Cambiar imagen del producto' }).click();
  await page.getByRole('button', { name: 'Guardar imagen' }).click();
  await expect(page.getByRole('alert')).toContainText(/URL/i);
});
