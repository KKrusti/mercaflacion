# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Communication

Always communicate with the user in **Spanish**, regardless of the language used in code, comments, or commit messages.

## Skills

Before writing any code, **read** the corresponding SKILL.md file and **print the skill name visibly** in the response using the format `[skill: <name>]`. This must appear before any code or implementation detail.

| Situation | Skills to load | Path                                                                                            |
|-----------|---------------|-------------------------------------------------------------------------------------------------|
| Backend changes | `golang-pro` | `.claude/skills/golang-pro/SKILL.md`                                                            |
| Frontend / UI changes | `vercel-react-best-practices` **+** `ui-ux-pro-max` | `.claude/skills/vercel-react-best-practices/SKILL.md` · `.claude/skills/ui-ux-pro-max/SKILL.md` |

`vercel-react-best-practices` and `ui-ux-pro-max` are complementary and must always be loaded together for any frontend or UI work.

**This is mandatory and non-negotiable.** The user explicitly requires seeing `[skill: <name>]` printed in the response to verify correct skill usage.

## Commands

```bash
# Full dev stack (backend :8080 + frontend :5173)
task dev

# Install frontend deps (WSL-safe symlink setup — run once)
task dev:deps

# Backend only
task backend
# or: cd backend && go run ./cmd/server/main.go

# Frontend only
cd frontend && npm run dev

# Type-check frontend (no emit)
cd frontend && npx tsc --noEmit

# Tests
task test                                                        # both suites
task test:backend                                                # Go tests
task test:frontend                                               # Vitest
cd backend && go test ./internal/handlers/                       # single package
cd backend && go test -run TestFunctionName ./internal/handlers/ # single test
cd frontend && npm run test:watch                                 # watch mode
cd frontend && npm run test:coverage

# E2E tests (Playwright — Chromium + mobile Poco X6 Pro)
task test:e2e
task test:e2e:headed   # with visible browser window

# Seed the DB with sample receipts
cd backend && go run ./cmd/seed/main.go -dir ./seed

# Enrich product images from the Mercadona API
task enrich
# or: cd backend && go run ./cmd/enrich/main.go -db basket-cost.db

# DB utilities
task db:reset     # wipe all data (prompts for confirmation)
task db:sanitize  # detect and remove duplicate price records
task kill         # kill the backend process on :8080
```

## Architecture

Two-tier SPA: Go REST API on `:8080` + React/Vite SPA on `:5173` (proxies `/api` to backend).

**Backend layout (`backend/`):**
- `cmd/server/main.go` — wiring only: routing, CORS middleware, `ListenAndServe`
- `cmd/seed/` and `cmd/enrich/` — standalone CLIs; no business logic in `cmd/`
- `internal/models/` — pure data structs, no logic
- `internal/store/` — `Store` interface + `SQLiteStore` implementation (repository layer)
- `internal/handlers/` — HTTP layer only, delegates to other packages
- `internal/ticket/` — full PDF import pipeline: `extractor` → `parser` → `importer` (persists via store)
- `internal/enricher/` — product image enrichment from the public Mercadona catalogue API
- `internal/database/db.go` — SQLite connection (CGO-free `modernc.org/sqlite`), WAL pragmas, schema migrations

**Frontend layout (`frontend/src/`):**
- `App.tsx` — app shell; owns `browserState` for persistence across navigation
- `api/products.ts` — all `fetch` calls; throws `Error` on non-OK responses
- `types/index.ts` — shared TypeScript interfaces
- `index.css` — design system: CSS variables + all component styles (no CSS modules)
- Components are co-located with their `*.test.tsx` files

**API:**
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/auth/register` | Register new user |
| `POST` | `/api/auth/login` | Login → returns JWT |
| `PATCH` | `/api/auth/password` | Change password (auth required) |
| `GET` | `/api/products?q=<query>` | Search; empty `q` returns all, ordered by most-recently-purchased |
| `GET` | `/api/products/<id>` | Product detail + full price history |
| `PATCH` | `/api/products/<id>/image` | Set product image URL manually (auth required) |
| `POST` | `/api/tickets` | Upload one Mercadona PDF (`multipart/form-data`, field `file`, max 10 MB) |
| `GET` | `/api/analytics` | Top price increases + most-purchased products |
| `GET` | `/api/household` | List household members (auth required) |
| `DELETE` | `/api/household` | Leave household (auth required) |
| `POST` | `/api/household/invite` | Create 24h invitation token (auth required) |
| `POST` | `/api/household/accept?token=<tok>` | Accept invitation and join household (auth required) |

Multiple files are uploaded by calling `POST /api/tickets` once per file in parallel (`Promise.all`). No batch endpoint exists.

**Auth & multi-tenancy:**
- JWT (HS256, 72 h TTL) issued at login; sent as `Authorization: Bearer <token>` on every authenticated request.
- `cmd/server/main.go` middleware extracts the user ID from the token and stores it in `context` via `handlers.UserIDContextKey`. Handlers call `handlers.UserIDFromContext(r)` — returns `0` for unauthenticated requests.
- All read queries scope data by household: `store.householdUserIDs(userID)` expands one user to all co-members, then `userIDsInClause()` builds `user_id IN (?,?)` or `user_id IS NULL` for seed/anonymous data. Write queries (ticket upload, mark processed) still use the individual `userID`.
- `CreateHouseholdInvitation` auto-creates a household for the inviter if they don't have one yet.

**Enricher pipeline (`internal/enricher/`):**
- `normalise` → `translateCatalan` (Catalan→Spanish dict, `catalan_dict.go`) → `keywords` (stop-word filter + trailing-`s` stem) → `bestMatch` (Dice coefficient ≥ 0.5).
- `bestMatch` additionally requires `matched ≥ 2` when the local product has ≥ 2 keywords, preventing single-token false positives (e.g. "patatas" matching "Patatas 3 kg").
- The Mercadona catalogue index is cached in memory for 24 h; `Schedule()` coalesces concurrent upload signals so at most one catalogue download runs at a time.

## Key Design Patterns

**ProductBrowser controlled/uncontrolled state:** accepts optional `browserState: ProductBrowserState` + `onBrowserStateChange` props. When provided, the parent owns state; otherwise the component manages it internally. `App.tsx` uses this to preserve pagination when navigating to `ProductDetail` and back.

**useEffect cancellation pattern** (mandatory for all async data fetching in components):
```ts
useEffect(() => {
  let cancelled = false;
  getProduct(id).then((data) => { if (!cancelled) setData(data); });
  return () => { cancelled = true; };
}, [id]);
```

**Debounce:** `setTimeout`/`clearTimeout` inside `useEffect` — no external debounce library (SearchBar uses 300 ms).

**Product IDs:** derived slugs generated by `store.UpsertPriceRecord` — never generate in handlers.

**PriceChangeBadge:** shows `((current - first) / first) × 100%`; only rendered when ≥ 2 price records. CSS classes: `--up` (red), `--down` (green), `--flat` (neutral).

## Testing Policy

Tests are mandatory for every new piece of code.

- **Go:** `_test.go` in the same package; use `net/http/httptest` for handler tests.
- **React/TS:** co-located `*.test.tsx` per component, `*.test.ts` per API/util module; use Vitest + `@testing-library/react`.
- Mock third-party libs that don't render in jsdom (e.g. `recharts`) with `vi.mock(...)`.
- Prefer semantic queries (`getByRole`, `getByLabelText`); fall back to `document.querySelector` with a CSS class only when the same text appears in multiple nodes.
- Run both suites and confirm they pass before considering a task done.

## Go Style Guidelines

- Guard-clause / early-return error handling; `http.Error(w, msg, code)` + `return` in handlers; `log.Fatal` for startup failures in `main()`.
- `json.NewEncoder(w).Encode(v)` for JSON responses; always set `Content-Type: application/json` first.
- CORS is handled by a hand-rolled middleware in `cmd/server/main.go` — do not add external CORS libraries.
- All exported struct fields must have `json:"..."` tags; use `omitempty` on optional fields.
- Imports: single grouped block, stdlib first then internal (goimports order), no blank-line separation between stdlib and internal. Prefer stdlib `errors` and `context` — do not use `github.com/pkg/errors` or `golang.org/x/net/context`.
- Acronyms: `GetProductByID` not `GetProductById`. No Hungarian notation or type suffixes.
- Schema changes only through the migration table in `internal/database/db.go` — never raw `ALTER TABLE`.

## TypeScript / React Style Guidelines

- Function components only; `export default function ComponentName(props: ComponentNameProps)`.
- Props defined as a local `interface ComponentNameProps` in the same file.
- Shared domain types in `src/types/index.ts` as `interface`; discriminated unions as `type`.
- `import type { ... }` for type-only imports (`isolatedModules: true` enforced).
- Local `useState` only — no Redux, Zustand, Context API, or external state libraries.
- Raw `fetch` only — no axios or other HTTP libraries.
- TypeScript strict mode + `noUnusedLocals`, `noUnusedParameters` are enforced; no `@ts-ignore` without explanation.
- SVG icons are defined as small inline function components in the same file — do not use emoji as UI icons.
- Inline arrow formatters (`formatPrice`, `formatDate`) are fine inside a component; extract to `src/utils/` only when used in more than one component.
- Code identifiers and comments may be in English or Spanish; be consistent within a file. UI text and labels must always be in Spanish.


## Task Tracking

Use `bd` (Beads) for task tracking:
- `bd new "title"` — create an issue before starting any planned task
- `bd state <id> in_progress` — mark in-progress when beginning (`bd start` does not exist)
- `bd close <id>` — mark as done
- `bd ready` — review what's pending at end of session
- Update `PROGRESS.md` after each completed task so work can be resumed from scratch if the session is interrupted.
