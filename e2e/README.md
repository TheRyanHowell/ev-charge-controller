# e2e

Playwright end-to-end tests for the EV Charge Controller. Tests run against
the full live stack (UI + API + mock Tasmota) inside Docker. Chromium and
Firefox are both covered.

## Test Structure

```
tests/
  stateless/   - read-only tests; run fully in parallel; see seed data only
  stateful/    - mutation tests; run serially (workers=1); reset DB each test
  helpers/     - shared auth setup and global setup
  pages/       - Page Object Models (login, dashboard, vehicles, history)
```

**Stateless** tests never write to the database, so they can run in parallel
across workers and browsers without interfering with each other.

**Stateful** tests mutate shared state (sessions, vehicles, schedules). They
run with `workers=1` to guarantee serial execution and reset the database
before each test via the seed API.

## Commands

All commands run inside Docker via `make` from the repo root.

```bash
make test-e2e                  # Full suite: stateless then stateful (Chromium + Firefox)
make test-e2e-chromium         # Full suite, Chromium only
make test-e2e-firefox          # Full suite, Firefox only

make test-e2e-file FILE=tests/stateless/vehicles.spec.ts      # Single file
make test-e2e-chromium-file FILE=tests/stateful/vehicle-crud.spec.ts
make test-e2e-firefox-file  FILE=tests/stateless/login-flow.spec.ts

make lint-e2e                  # Prettier + ESLint + TypeScript type check
make fix-e2e                   # Auto-fix lint issues
```

## Projects

| Project | Test dir | Parallel | Browsers |
|---------|----------|----------|---------|
| `chromium-stateless` | `tests/stateless` | yes | Chromium |
| `firefox-stateless` | `tests/stateless` | yes | Firefox |
| `chromium-stateful` | `tests/stateful` | no (workers=1) | Chromium |
| `firefox-stateful` | `tests/stateful` | no (workers=1) | Firefox |

## Writing Tests

- Use semantic locators: `getByRole`, `getByText`, `getByLabel`. No CSS selectors or XPath.
- Always pass a custom message to `expect()` describing the business logic being checked.
- Stateful tests must reset state in `beforeEach` via the seed API before asserting anything.
- New stateful test files go in `tests/stateful/`; new stateless files go in `tests/stateless/`.

See `AGENTS.md` for the debugging workflow (DOM inspection, browser error capture, MCP usage).
