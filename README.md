# basket-cost

A grocery price tracker for the Spanish market — built as a playground for experimenting with AI coding agents ([OpenCode](https://opencode.ai) and [Claude Code](https://claude.ai/code)).

Upload Mercadona PDF receipts to populate the database, then search for any product to see its current price and a historical evolution chart.

---

## What this actually is

This repo is a **sandbox for vibe-coding with AI agents**. Every feature, refactor, test, and config tweak in this codebase was driven through natural-language prompts to the agent. The project is intentionally contained so the interesting part is watching the agent navigate a real two-tier codebase — routing, a Postgres-backed API, React components, PDF parsing, JWT auth, tests, a task runner — not the app itself.

If you want to play with an AI coding agent on a project that has real structure without building something from scratch, clone this and start prompting.

---

## Stack

| Layer | Tech |
|---|---|
| Backend | Go 1.26 · PostgreSQL via [Neon](https://neon.tech) (`pgx/v5`) · PDF parsing (`ledongthuc/pdf`) · JWT auth (`golang-jwt/jwt`) |
| Frontend | React 18 · TypeScript · Vite · Recharts |
| Deployment | [Vercel](https://vercel.com) — Go backend as a single serverless function, React SPA as static assets |
| Tests | Go `testing` + `net/http/httptest` · Vitest + Testing Library · Playwright (E2E) |
| Task runner | [go-task](https://taskfile.dev) |

---

## Features

- **Upload tickets** — import one or several Mercadona PDF receipts at once. Each file is processed independently; partial failures are reported per-file without aborting the batch.
- **Search products** — live search with 300 ms debounce across your catalogue.
- **Browse catalogue** — grid view of all products with configurable page size and column count.
- **Price history** — interactive line chart plus a scrollable price table for any selected product, with a badge showing overall price change since first purchase.
- **Analytics** — top products by purchase frequency and biggest price increases over time.
- **Product images** — enriched from the public Mercadona catalogue API; falls back to a category emoji when unavailable. Supports manual image URL override.
- **Household sharing** — invite family members to share a catalogue. Accepting an invite merges your data under one household.
- **User accounts** — register and log in; login is required to access the app.
- **Light/dark mode** — toggle in the header; persists across sessions and respects `prefers-color-scheme` on first load.

---

## Project structure

```
basket-cost/
├── Taskfile.yml
├── vercel.json                       # Vercel build config + security headers
├── api/index.go                      # Vercel serverless entry point: routing, middleware, handler wiring
├── cmd/
│   ├── devserver/main.go             # local dev server (delegates to api.Handler)
│   ├── seed/main.go                  # CLI: bulk-import PDF receipts
│   └── enrich/main.go                # CLI: download product images from Mercadona API
├── pkg/
│   ├── auth/                         # bcrypt password hashing + HS256 JWT (24 h TTL)
│   ├── database/db.go                # PostgreSQL connection pool (pgx/v5), schema migrations
│   ├── models/models.go              # domain types: User, Product, PriceRecord, SearchResult…
│   ├── store/                        # Store interface + PostgresStore (household-scoped queries)
│   ├── handlers/                     # HTTP handlers (Auth, Search, Product, Ticket, Analytics, Household)
│   ├── middleware/                   # JWT auth middleware, injects userID into request context
│   ├── enricher/                     # image-URL enrichment from Mercadona public API
│   ├── ratelimit/                    # per-IP rate limiting (reads X-Forwarded-For for Vercel proxy)
│   └── ticket/                       # PDF import pipeline: extract → parse → persist
├── frontend/
│   └── src/
│       ├── App.tsx                   # app shell: header, tabs, auth state, theme toggle
│       ├── index.css                 # design system: CSS variables (light + dark), all component styles
│       ├── types/index.ts            # shared TypeScript interfaces
│       ├── api/                      # fetch-based API clients (auth headers)
│       └── components/               # co-located *.test.tsx for every component
└── sample-tickets/                   # bundled sample Mercadona PDF receipts
```

---

## Running locally

**Required environment variables** (put in `.env`):

```
DATABASE_URL=<Neon PostgreSQL DSN>
JWT_SECRET=<random string, min 32 chars>   # generate with: openssl rand -base64 32
```

```bash
# Install frontend deps (WSL-safe — run once)
task dev:deps

# Start everything
task dev
# → backend:  http://localhost:8080
# → frontend: http://localhost:5173
```

To seed the database with the bundled sample receipts:

```bash
go run ./cmd/seed -dir ./sample-tickets
```

---

## API

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/auth/register` | — | Create a new user account |
| `POST` | `/api/auth/login` | — | Authenticate and receive a JWT |
| `POST` | `/api/auth/logout` | required | Revoke the current token server-side |
| `PATCH` | `/api/auth/password` | required | Change password |
| `GET` | `/api/products?q=<query>` | required | Search products scoped to household; empty `q` returns all |
| `GET` | `/api/products/<id>` | required | Full product detail with price history |
| `PATCH` | `/api/products/<id>/image` | required | Set a manual image URL for a product |
| `POST` | `/api/tickets` | required | Upload a Mercadona PDF receipt (`multipart/form-data`, field `file`, max 10 MB) |
| `GET` | `/api/analytics` | required | Top purchased products and biggest price increases |
| `GET` | `/api/household` | required | List household members |
| `DELETE` | `/api/household` | required | Leave the current household |
| `POST` | `/api/household/invite` | required | Create a 24 h invitation link (replaces any previous one) |
| `POST` | `/api/household/accept?token=<tok>` | required | Accept an invitation and join the household |

The frontend uploads multiple files by calling `POST /api/tickets` once per file in parallel via `Promise.all`. There is no dedicated batch endpoint.

---

## Tests

```bash
task test:backend   # Go integration tests (requires DATABASE_URL)
task test:frontend  # Vitest unit/component tests
task test           # both
task test:e2e       # Playwright E2E (Chromium + mobile)
```

---

## Notes

- Product names, categories and store names are in Spanish (the app targets the Spanish market).
- All other code, comments, and identifiers are in English.
- See `CLAUDE.md` for the full coding guidelines used to prompt the agent.
