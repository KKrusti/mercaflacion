# Sesión de Planificación — 2026-03-05

**Estado de la rama:** `refactor/security-solid-dry-improvements` con cambios sin commitear en backend.

---

## ¿Qué se completó en esta sesión?

### 1. Análisis de plataforma E2E ✅

Se evaluaron 3 opciones: **Playwright**, Cypress, Puppeteer.

**Decisión final:** **Playwright**
- Emulación móvil real (viewport 393×852px, touch, DPR) para Poco X6 Pro
- Sin Electron (~150 MB vs 500 MB de Cypress)
- WSL compatible (Cypress tiene problemas conocidos)
- `webServer` nativo para arrancar Go+Vite automáticamente
- API async/await pura (consistente con el proyecto)

### 2. Planificación de tests E2E ✅

**Scope:** Happy path únicamente — 7 flujos críticos, 1 test por flujo.
Casos de error se cubren con tests unitarios (Vitest).

**Flujos a testear:**
1. Auth: registro, login, bloqueo sin sesión
2. Catálogo: paginación, pageSize, columnas
3. Búsqueda: debounce, resultados, estado preservado
4. Detalle de producto: gráfico, historial, navegación
5. Upload ticket PDF: progreso, toast éxito
6. Imagen manual: URL Mercadona → PATCH → visual
7. Analítica: datos, navegación a detalle

### 3. Issues creados ✅

**En Beads (9 tareas E2E + 18 frontend pendientes):**
- `basket-cost-7sy`: Setup Playwright
- `basket-cost-abk` a `basket-cost-3fo`: 7 tests E2E happy path
- `basket-cost-2ev`: Task `test:e2e` en Taskfile

**En GitHub (2 issues):**
- [#10](https://github.com/KKrusti/basket-cost/issues/10): E2E Decisión y setup de Playwright
- [#11](https://github.com/KKrusti/basket-cost/issues/11): E2E Tests happy path (detalle de cada test)

---

## Estado actual del proyecto

### Backend — COMPLETADO (sin commitear)

**Cambios en working tree:**
- Multi-tenancy completa (userID=0 anónimo, userID>0 autenticado)
- JWT auth: bcrypt + HS256 (72h TTL)
- Rutas registradas: `/api/auth/register`, `/api/auth/login`
- `ProductImageHandler` implementado pero **ruta PATCH NO registrada** (issue `basket-cost-jhu`)
- Migraciones: m4 (users), m5 (user_id), m6 (image_url_locked)
- Tests: todos pasan ✅

**Archivos modificados:**
```
backend/
  cmd/seed/main.go
  cmd/server/main.go
  go.mod / go.sum
  internal/
    auth/auth.go (NUEVO)
    auth/auth_test.go (NUEVO)
    database/db.go
    handlers/handlers.go
    handlers/handlers_test.go
    models/models.go
    store/store.go
    store/store_test.go
    ticket/importer.go
    ticket/importer_test.go
```

### Frontend — PENDIENTE (18 tareas Beads)

**Orden de implementación recomendado:**
1. `basket-cost-h1z`: Tipos TypeScript (User, AuthState, imageUrlLocked)
2. `basket-cost-9rj`: API auth (register, login, logout)
3. `basket-cost-f86`: LoginModal component
4. `basket-cost-lgb`: App.tsx integración
5. `basket-cost-0v1`: Auth headers en products.ts
6. `basket-cost-68e`: updateProductImage función
7. `basket-cost-zkj`: Botón imagen manual en ProductDetail
8. `basket-cost-wuu`: Tests (LoginModal, auth API, imagen)
9. `basket-cost-9`: Mobile-first CSS refactor (GitHub #9, baja prioridad)

---

## Notas de implementación importantes

### Auth flow

1. Usuario se registra en LoginModal con email+password
2. `POST /api/auth/register` → backend devuelve JWT
3. Frontend guarda JWT en `localStorage` bajo clave `jwt`
4. `LoginModal` se cierra automáticamente
5. `App.tsx` lee JWT y lo adjunta a cada llamada: `Authorization: Bearer <token>`
6. Sin JWT → `LoginModal` visible y bloquea la app

### Imagen manual

1. Backend: `ProductImageHandler` existe pero falta registrar en `cmd/server/main.go`
   - Ruta: `PATCH /api/products/{id}/image`
   - Body: `{ "imageURL": "https://www.mercadona.es/..." }`

2. Frontend: ProductDetail necesita botón editar
   - Click → modal/input para URL
   - Validación: `mercadona.es`
   - Confirmar → `updateProductImage(id, url)`
   - Imagen se actualiza en la UI

### Tests E2E

Estructura esperada:
```
frontend/e2e/
  playwright.config.ts
  fixtures/sample-ticket.pdf
  tests/
    auth.spec.ts
    catalog.spec.ts
    search.spec.ts
    product-detail.spec.ts
    ticket-upload.spec.ts
    product-image.spec.ts
    analytics.spec.ts
```

Notas:
- Usar `storageState` para persistir JWT entre tests
- BD separada para tests (`TEST_DB_PATH=e2e-test.db`)
- webServer arranca backend + frontend automáticamente
- No usar mocks — contra backend real

---

## Dependencias entre tareas

**Frontend** (orden secuencial):
```
h1z (tipos) → 9rj (auth API) → f86 (LoginModal) → lgb (App.tsx) → 0v1 (headers)
                                                                      ↓
                                                    68e (updateImage) → zkj (botón)
                                                                           ↓
                                                                216 (test imagen)
                                                                [depende también de jhu]
```

**E2E**:
```
basket-cost-7sy (setup) → [abk, 1iv, b5p, azc, t1y, 216, 3fo] (tests) → 2ev (task:e2e)
```

---

## Próxima sesión: Pasos inmediatos

1. **Commit backend** (si no se hizo): 100% completo y testeado
2. **Implementar frontend**: tipos → auth API → LoginModal → App.tsx → headers → imagen
3. **Setup Playwright**: estructura, fixtures, config
4. **Tests E2E**: 1 por 1 (auth primero, analítica último)
5. **Taskfile**: `task test:e2e`

---

**Última actualización:** 2026-03-05 22:45 UTC
