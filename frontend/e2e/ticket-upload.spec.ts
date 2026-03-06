import { test, expect } from '@playwright/test';
import { stubProducts } from './helpers';
import path from 'path';
import fs from 'fs';

// Minimal valid PDF bytes (just the header magic; the backend will reject it but we
// test the frontend behaviour, intercepting the network at the route level).
const FAKE_PDF = Buffer.from('%PDF-1.4 fake content for testing');

test.beforeEach(async ({ page }) => {
  await stubProducts(page);
  await page.goto('/');
});

test('the ticket upload button is visible', async ({ page }) => {
  await expect(page.getByRole('button', { name: /subir ticket/i })).toBeVisible();
});

test('shows progress when uploading a file and a success toast', async ({ page }) => {
  // Small delay so the progress panel is visible before the upload completes.
  await page.route('/api/tickets', (route) =>
    new Promise<void>((resolve) => setTimeout(resolve, 200)).then(() =>
      route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ invoiceNumber: '4144-017-284404', linesImported: 23 }),
      }),
    ),
  );

  // Write temp file
  const tmpPath = path.join('/tmp', 'ticket-test.pdf');
  fs.writeFileSync(tmpPath, FAKE_PDF);

  // Intercept file chooser
  const [fileChooser] = await Promise.all([
    page.waitForEvent('filechooser'),
    page.getByRole('button', { name: /subir ticket/i }).click(),
  ]);
  await fileChooser.setFiles(tmpPath);

  // Progress panel should appear
  await expect(page.locator('.ticket-uploader__progress')).toBeVisible({ timeout: 3000 });

  // Toast should appear after completion
  await expect(page.locator('.ticket-uploader__toast')).toBeVisible({ timeout: 5000 });
  await expect(page.locator('.ticket-uploader__toast')).toContainText(/23/);

  fs.unlinkSync(tmpPath);
});

test('shows an error toast when the server rejects the ticket', async ({ page }) => {
  await page.route('/api/tickets', (route) =>
    route.fulfill({ status: 422, body: 'Unprocessable entity' }),
  );

  const tmpPath = path.join('/tmp', 'bad-ticket.pdf');
  fs.writeFileSync(tmpPath, FAKE_PDF);

  const [fileChooser] = await Promise.all([
    page.waitForEvent('filechooser'),
    page.getByRole('button', { name: /subir ticket/i }).click(),
  ]);
  await fileChooser.setFiles(tmpPath);

  await expect(page.locator('.ticket-uploader__toast')).toBeVisible({ timeout: 5000 });
  await expect(page.locator('.ticket-uploader__toast')).toContainText(/válido|procesar/i);

  fs.unlinkSync(tmpPath);
});

test('the toast disappears when the close button is pressed', async ({ page }) => {
  await page.route('/api/tickets', (route) =>
    route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({ invoiceNumber: '4144-017-000001', linesImported: 5 }),
    }),
  );

  const tmpPath = path.join('/tmp', 'ticket-close.pdf');
  fs.writeFileSync(tmpPath, FAKE_PDF);

  const [fileChooser] = await Promise.all([
    page.waitForEvent('filechooser'),
    page.getByRole('button', { name: /subir ticket/i }).click(),
  ]);
  await fileChooser.setFiles(tmpPath);

  await expect(page.locator('.ticket-uploader__toast')).toBeVisible({ timeout: 5000 });
  await page.getByRole('button', { name: /cerrar/i }).last().click();
  await expect(page.locator('.ticket-uploader__toast')).not.toBeVisible();

  fs.unlinkSync(tmpPath);
});
