# api

Go REST API for the EV Charge Controller. Owns all business logic, SQLite
persistence, background workers, MQTT integration, and Tasmota device control.

See **[docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md)** for the full design.

## Package Layout

```
api/
  cmd/
    createuser/   - CLI: create a user account
    genkeys/      - CLI: generate VAPID keys for push notifications
    seed/         - CLI: seed/reset the database with demo data
  carbonintensity/ - UK Carbon Intensity API client (30-minute cached forecasts)
  chargeestimate/  - conservative charge-duration estimator for scheduling
  database/        - SQLite init, migrations, catalog seed
  handlers/        - HTTP handlers (one per resource)
  internal/        - shared interfaces, config, logging, context helpers
    workers/       - background pollers (energy, schedule, auto-stop, SOC)
  middleware/      - JWT auth, CORS, request ID, body limit
  models/          - domain structs and constants
  mqtt/            - client, dispatcher, LWT manager, plug cache, dynsec
  repository/      - DB access (one repo per model)
  services/        - business logic (auth, session lifecycle, scheduling,
                     provisioning, tariffs, carbon, SOC, seeding)
  tasmota/         - Tasmota HTTP client + circuit breaker + test helpers
```

## Commands

All commands run inside Docker via `make` from the repo root.

```bash
make test-api           # Run all Go tests
make test-api-file FILE=./services/vehicle_service_test.go  # Run a specific test file
make test-race          # Run tests under the race detector
make cover-api          # Tests with coverage report
make lint-api           # golangci-lint
make vet-api            # go vet
make fix-api            # Auto-fix lint issues (golangci-lint --fix)
make deadcode-api       # Detect unreachable code
```
