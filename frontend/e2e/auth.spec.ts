import { test, expect } from '@playwright/test';
import { stubEmptyProducts, stubAuth } from './helpers';

test.beforeEach(async ({ page }) => {
  await stubEmptyProducts(page);
  await stubAuth(page);
  // Clear localStorage before each test
  await page.goto('/');
  await page.evaluate(() => localStorage.clear());
  await page.reload();
});

test('muestra el botón "Entrar" cuando el usuario no está autenticado', async ({ page }) => {
  await expect(page.getByRole('button', { name: 'Iniciar sesión' }).first()).toBeVisible();
});

test('abre el modal de login al pulsar el botón "Entrar"', async ({ page }) => {
  await page.getByRole('button', { name: 'Iniciar sesión' }).first().click();
  await expect(page.getByRole('dialog')).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Iniciar sesión' })).toBeVisible();
});

test('cierra el modal al pulsar el botón X', async ({ page }) => {
  await page.getByRole('button', { name: 'Iniciar sesión' }).first().click();
  await page.getByRole('button', { name: 'Cerrar' }).click();
  await expect(page.getByRole('dialog')).not.toBeVisible();
});

test('cierra el modal al hacer click en el overlay', async ({ page }) => {
  await page.getByRole('button', { name: 'Iniciar sesión' }).first().click();
  // Click on the overlay (outside the modal card)
  await page.getByRole('dialog').click({ position: { x: 10, y: 10 } });
  await expect(page.getByRole('dialog')).not.toBeVisible();
});

test('hace login correctamente y muestra el nombre de usuario', async ({ page }) => {
  await page.getByRole('button', { name: 'Iniciar sesión' }).first().click();
  await page.getByLabel('Usuario').fill('testuser');
  await page.getByLabel('Contraseña').fill('password123');
  await page.getByRole('button', { name: 'Entrar' }).last().click();
  await expect(page.getByRole('dialog')).not.toBeVisible();
  await expect(page.getByRole('button', { name: /testuser/i })).toBeVisible();
});

test('hace logout correctamente y vuelve a mostrar el botón Entrar', async ({ page }) => {
  // Login first
  await page.getByRole('button', { name: 'Iniciar sesión' }).first().click();
  await page.getByLabel('Usuario').fill('testuser');
  await page.getByLabel('Contraseña').fill('password123');
  await page.getByRole('button', { name: 'Entrar' }).last().click();
  await expect(page.getByRole('button', { name: /testuser/i })).toBeVisible();

  // Logout
  await page.getByRole('button', { name: /testuser/i }).click();
  await page.getByRole('menuitem', { name: 'Cerrar sesión' }).click();
  await expect(page.getByRole('button', { name: 'Iniciar sesión' }).first()).toBeVisible();
});

test('cambia al modo registro al pulsar la pestaña "Registrarse"', async ({ page }) => {
  await page.getByRole('button', { name: 'Iniciar sesión' }).first().click();
  await page.getByRole('button', { name: 'Registrarse' }).click();
  await expect(page.getByRole('heading', { name: 'Crear cuenta' })).toBeVisible();
});

test('registra un nuevo usuario y muestra su nombre', async ({ page }) => {
  await page.getByRole('button', { name: 'Iniciar sesión' }).first().click();
  await page.getByRole('button', { name: 'Registrarse' }).click();
  await page.getByLabel('Usuario').fill('testuser');
  await page.getByLabel('Contraseña').fill('password123');
  await page.getByRole('button', { name: 'Crear cuenta' }).click();
  await expect(page.getByRole('button', { name: /testuser/i })).toBeVisible();
});

test('muestra error cuando el login falla (401)', async ({ page }) => {
  await page.route('/api/auth/login', (route) =>
    route.fulfill({ status: 401, body: 'Unauthorized' }),
  );
  await page.getByRole('button', { name: 'Iniciar sesión' }).first().click();
  await page.getByLabel('Usuario').fill('wronguser');
  await page.getByLabel('Contraseña').fill('wrongpass12');
  await page.getByRole('button', { name: 'Entrar' }).last().click();
  await expect(page.getByRole('alert')).toContainText('Usuario o contraseña incorrectos');
});
