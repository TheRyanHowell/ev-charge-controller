# Architecture Guide

This document describes how the EV Charge Controller is put together: the domain
model, the concurrency strategy, the MQTT and Tasmota integration, carbon-aware
scheduling, and the frontend data flow.

## System Overview

The system controls and monitors the charging of [Maeving](https://maeving.com)
electric motorbikes (and their removable batteries) through [Tasmota](https://tasmota.github.io)
smart plugs. It has four runtime components:

```
+-------------+        HTTP/JSON         +-----------------+
|  Next.js UI |  <-------------------->  |     Go API      |
| (App Router)|     (proxy routes)       |  (net/http mux) |
+-------------+                          +--------+--------+
                                                  |
                                          MQTT    |   HTTP (provisioning,
                                       (telemetry) |    power on/off)
                                                  v
                                         +-----------------+      +--------------+
                                         |    Mosquitto    | <--> | Tasmota plug |
                                         |  (MQTT broker)  |      |  (hardware)  |
                                         +-----------------+      +--------------+
```

- **Next.js UI** renders the dashboard. Server Components fetch initial data and
  hydrate Client Components; React Query keeps live data fresh.
- **Go API** owns all business logic and persistence (SQLite). It exposes a
  resource-oriented REST API and runs background workers.
- **Mosquitto** brokers MQTT telemetry between plugs and the API. Plug energy
  readings and availability (LWT) flow in; relay commands flow out.
- **Tasmota plugs** are the physical hardware. The API also talks to them over
  HTTP once, during provisioning, to push their MQTT configuration.

## Domain Model

| Entity | Purpose |
|--------|---------|
| **VehicleModel** | Global catalog row (e.g. Maeving RM1/RM1S/RM2): capacity, charger output, efficiency, charge-time curve, range. Read-only. The `generic` entry (listed first) has no battery data (`capacity_kwh = 0`) and represents e.g. a petrol vehicle that only uses a 12V maintenance charger; EV charge sessions are rejected for its instances (`400 vehicle-has-no-battery`). |
| **Vehicle** | A user's instance of a model. Holds live state (current/target SoC), lifetime aggregates, and per-vehicle notification preferences. API responses merge the catalog fields in so consumers see a full shape. |
| **Plug** | A Tasmota smart plug owned by a user. Type is `charging` (driven by charge sessions) or `maintenance` (a 12V trickle charger toggled directly). Carries its MQTT namespace, topic, online state, and the vehicle it is assigned to. |
| **ChargeSession** | A charging run for a vehicle. Tracks start/target/end SoC, wall- and battery-side energy, average carbon intensity, CO2 grams, cost in pence, and off-peak energy. |
| **Schedule** | A per-plug schedule. Either `daily` (fire at a fixed time) or `carbon_aware` (charge inside a window so the vehicle is ready by a deadline at the lowest carbon cost). |
| **TariffSettings** | A user's electricity tariff: a base (peak) rate plus zero or more off-peak windows, in pence per kWh. Drives session cost accounting. |

Charge sessions are **vehicle-centric**: a vehicle has at most one in-progress
session, and the API resolves the relevant plug from the vehicle's assignment.

## Concurrency Model

The API serialises all mutations to in-progress charge sessions through a shared
`sessionLock` (`api/services/session_lock.go`). This prevents races between three
independent execution paths:

- **HTTP handlers** (user-initiated Start / Stop / Update target)
- **Background workers** (energy poller, schedule activator, auto-stop checker)
- **MQTT goroutines** (LWT manager cancels a session when its plug drops offline)

### sessionLock protocol

`sessionLock` is a named type (not a bare `*sync.Mutex`) shared by pointer across
`ChargeSessionService` and its lifecycle and monitoring sub-services, which makes
the shared ownership explicit:

```go
type sessionLock struct {
    mu sync.Mutex
}
```

The discipline is: only fast, in-process work and the session's own
check-then-act DB section belong inside the locked region. Long or external I/O
(HTTP to Tasmota, carbon-intensity lookups, push notifications) must happen
**outside** the lock so a slow dependency never stalls every lifecycle operation:

```go
// Correct: external I/O outside the lock, DB mutation inside.
_ = tasmotaClient.SetPower(ctx, true) // outside lock
lock.Lock()
// ... check active session, update status, persist
lock.Unlock()
```

### Invariants protected

- **One active session per vehicle**: at most one session in an in-progress
  status (`active`, `pending`, `conditioning`).
- **Atomic state transitions**: no invalid intermediate states.
- **Energy monotonicity**: cumulative wall energy never regresses; monotonic
  clamping absorbs sensor drift (noise floor `epsilonKwh = 0.002`).

Race detection runs the whole suite under `-race`:

```bash
make test-race
```

## Background Workers

All workers run on a shared ticker helper (`RunTickerWorker`) and poll every
`PollIntervalSec` (5 s). They receive a context marked with
`internal.WithSystemContext` so repository helpers can tell authorised system
access apart from a handler that accidentally bypassed auth middleware.

| Worker | Responsibility |
|--------|----------------|
| **EnergyPoller** | Polls each online plug's Tasmota energy data, computes battery-side energy from wall energy x efficiency, persists power readings, and detects charge completion. |
| **ScheduleActivator** | Evaluates every enabled schedule each tick and starts charging when a daily time fires or a carbon-aware window reaches its latest safe start. |
| **AutoStopChecker** | Stops sessions that have reached their target SoC and times out `pending` sessions that never confirmed a relay state. |
| **SOCWorker** | Drains a buffered channel of SOC snapshot requests, offloading the (relatively expensive) SoC calculation and DB write off the polling hot path. |

## Carbon-Aware Scheduling

A `carbon_aware` schedule defines a window (`windowStart` to `windowEnd`, the
"ready-by" deadline) rather than a fixed start time. The goal is to finish
charging by the deadline while consuming the greenest electricity available.

1. **Estimate duration** (`api/chargeestimate`): from the vehicle's spec and the
   gap between current and target SoC, estimate charge minutes `D`. The estimate
   is deliberately conservative (a safety margin plus a CV-phase penalty) so the
   computed latest start is pulled earlier and the deadline guarantee is never
   missed.
2. **Fetch forecast** (`api/carbonintensity`): pull the 30-minute carbon
   intensity forecast from the UK Carbon Intensity API (cached 30 min in memory).
3. **Pick the start**: find the lowest-carbon contiguous block of length `D`
   inside the window, but never start later than `windowEnd - D`.

If forecast data or vehicle spec is unavailable, the schedule degrades safely
rather than risk missing the deadline.

## Tariffs & Cost Accounting

Cost is computed on **wall-side** energy (what the meter bills), not battery-side.
A `TariffSettings` row holds a base/peak rate and any number of off-peak windows
(`HH:MM` ranges, which may wrap past midnight). At session completion the API
applies the rate in effect at the session's start time, recording `costPence` and
flagging `offPeakKwh`. The same logic powers the live cost-per-kWh readout on the
dashboard gauge and the lifetime cost aggregates per vehicle.

## MQTT Architecture

Plug telemetry flows through Mosquitto. The Go API connects as a superuser
(`api-backend`) and subscribes to all `evcc/#` messages.

### Namespace isolation

Each plug gets a unique namespace at creation. All of its topics are prefixed
with that namespace:

```
evcc/<namespace>/tele/<topic>/SENSOR   - energy telemetry
evcc/<namespace>/tele/<topic>/LWT      - last will (Online/Offline)
evcc/<namespace>/cmnd/<topic>/POWER    - relay command
evcc/<namespace>/stat/<topic>/POWER    - relay state response
```

A Mosquitto dynamic-security (dynsec) integration enforces that a user's MQTT
credentials can only touch namespaces belonging to plugs they own.

### Message routing: Dispatcher

`api/mqtt/dispatcher.go` receives every message from the subscription goroutine
and routes it:

- **SENSOR** -> energy accounting on the active session
- **LWT** -> `LWTManager`

The dispatcher resolves a namespace+topic pair to a plug ID via an in-memory
`PlugCache` (refreshed on miss) before dispatching.

### Plug availability: LWTManager

`api/mqtt/lwt.go` tracks online/offline state per plug using Tasmota LWT messages:

```
Online received        -> cancel any debounce timer, mark plug online
Offline received       -> start a 60 s debounce timer (flap suppression)
Debounce fires         -> mark offline, cancel the plug's active session,
                          push a notification (15 min per-plug cooldown)
Online during debounce -> cancel timer, no offline transition
```

Retained LWT messages are skipped: they reflect stale broker state, not the
device's current state. `LWTManager` depends on a narrow `lwtSessionReader`
interface (`GetActiveByPlug` only) rather than the full session reader, following
Interface Segregation.

## Tasmota Integration & Resilience

`api/tasmota` is the HTTP client used for one-time device provisioning and as a
fallback for energy reads. A **circuit breaker** (`circuitbreaker.go`) wraps calls
to each device:

- **Closed**: requests pass through; consecutive failures increment a counter.
- **Open**: once `FailureThreshold` consecutive failures is hit, requests are
  rejected immediately with `ErrCircuitOpen` for `ResetTimeout`.
- **Half-open**: after the timeout a single probe is allowed; success closes the
  circuit, failure re-opens it.

This stops a dead or flapping device from stalling the energy poller.

## Auth & Multi-User

JWT-based authentication scopes all data to the authenticated user:

- **Access token**: 1-hour JWT (HS256, `JWT_SECRET`, min 32 bytes), sent as
  `Authorization: Bearer <token>`.
- **Refresh token**: 30-day opaque token stored as a SHA-256 hash, rotated on
  each use.
- **Middleware** (`middleware.AuthMiddleware`) validates the JWT and injects
  `userID` into the request context.

All repositories filter by the `user_id` extracted from context, so data is never
shared across users. Background workers run under `internal.WithSystemContext` for
their cross-user access; if neither a user ID nor the system marker is present,
`WarnIfMissingPrincipal` logs an error-level warning.

## Plug Provisioning

When a user creates a plug, `MqttProvisioningService` assigns its identity and can
configure the physical device:

1. **Namespace**: `ns-` + 16 random hex chars, unique per plug, fixes the MQTT
   topic prefix.
2. **MQTT credentials**: one credential set per user (shared across their plugs),
   created on the first plug and reused thereafter; the password is an argon2id
   hash, returned in plaintext only once at creation.
3. **Tasmota auto-configure** (`POST /api/plugs/{id}/configure`): when given a
   device IP, the API sends the MQTT config commands over HTTP (`MQTTHost`,
   `MQTTPort`, `MQTTUser`, `MQTTPassword`, `FullTopic`, `Topic`, `Restart 1`) with
   a short delay between each. Without an IP, it returns the console commands for
   the user to paste manually.

## Frontend Architecture

The UI is a Next.js App Router app (TypeScript, Tailwind):

- **Server Components** (`app/**/page.tsx`) fetch initial data with the user's
  access token and pass it to Client Components via `initialData` prop drilling,
  along with a `renderTimeMs` stamp.
- **React Query** hydrates from that `initialData` and refetches per
  `initialDataUpdatedAt`. Real-time data (active session, power readings, SOC) is
  treated as immediately stale so it refetches in the background; config data
  (vehicles, plugs, models) respects `staleTime`.
- **Proxy routes** (`app/api/**/route.ts`) forward to the Go backend and stay
  thin: no business logic, errors passed through as RFC 7807 Problem Details.
- **The speedometer gauge** (`components/SpeedometerGauge.tsx`) is a canvas
  component with draggable start/current/target markers. A Zustand store
  (`gaugeStore`) holds the live percentages so dragging stays smooth and
  decoupled from server round-trips.
- **Token refresh** is centralised in Next.js middleware, which transparently
  rotates the access token on the way through rather than in individual
  components.

## Structured Logging & Request Tracing

Every HTTP request is assigned a request ID (`middleware.RequestIDMiddleware`)
that flows through the request lifecycle and appears in all structured logs.

```go
internal.LogErrorContext(r.Context(), r.URL.Path, "operation failed", err, "entity_id", sessionID)
// req_id=req-0000000000000001 path=/api/charge-sessions err=... entity_id=...
```

Helpers (`api/internal/logger.go`): `LogAttrs`, `LogErrorContext`,
`LogInfoContext`, `LogDebugContext`. `InitLogger` reads `LOG_LEVEL`
(`debug`/`info`/`warn`/`error`, default `warn`) and `LOG_FORMAT` (`json` for
structured output, recommended in production) at startup.

## Shared Validation Schemas

`GET /api/schemas` exports Go struct definitions via reflection as machine-readable
JSON (`api/handlers/schema_handler.go`), intended as a future source of truth for
generating frontend Zod schemas. Today the frontend keeps manual Zod schemas in
`ui/src/lib/schemas.ts`; the export exists but codegen tooling is not yet wired up.

## Package Layout

```
api/
  cmd/
    createuser/   - CLI: create a user
    genkeys/      - CLI: generate VAPID keys for push
    seed/         - CLI: seed/reset the database with demo data
  carbonintensity/- UK Carbon Intensity API client (cached forecasts)
  chargeestimate/ - conservative charge-duration estimator for scheduling
  database/       - SQLite init, migrations, catalog seed
  handlers/       - HTTP handlers (one per resource)
  internal/       - shared interfaces, config, logging, context helpers
    workers/      - background pollers (energy, schedule, auto-stop)
  middleware/     - JWT auth, CORS, request ID, body limit
  models/         - domain structs and constants
  mqtt/           - client, dispatcher, LWT manager, plug cache, dynsec
  repository/     - DB access (one repo per model)
  services/       - business logic (auth, session lifecycle, scheduling,
                    provisioning, tariffs, carbon, SOC, seeding)
  tasmota/        - Tasmota HTTP client + circuit breaker + test helpers

ui/
  src/app/        - App Router routes (dashboard, vehicles, history, login)
    features/     - route-specific feature components
    api/          - thin proxy route handlers to the Go backend
  src/components/ - reusable UI primitives (gauge, charts, modals)
  src/hooks/      - React Query hooks (one per resource)
  src/lib/        - API client, Zod schemas, query keys
  src/stores/     - Zustand stores (gauge state)
```
