# ui

Next.js 16 (App Router) frontend for the EV Charge Controller. TypeScript,
Tailwind, React Query, and Zustand. Server Components fetch initial data; React
Query keeps live data fresh in the browser.

See **[docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md)** for the full design.

## Directory Structure

```
ui/src/
  app/              - App Router routes (dashboard, vehicles, history, login)
    features/       - route-specific feature components
    api/            - thin proxy route handlers forwarding to the Go backend
  components/       - reusable UI primitives (gauge, charts, modals)
  hooks/            - React Query hooks (one per resource)
  lib/              - API client, Zod schemas, query keys
  stores/           - Zustand stores (gauge state)
```

## Commands

All commands run inside Docker via `make` from the repo root.

```bash
make test-ui            # Run all Vitest unit tests
make test-ui-file FILE=src/app/vehicles/page.test.tsx  # Run a specific test file
make cover-ui           # Tests with coverage report
make build-ui           # Production build (catches type errors, duplicate exports)
make lint-ui            # Prettier + ESLint + TypeScript type check
make fix-ui             # Auto-fix lint issues (Prettier --write, ESLint --fix)
```
