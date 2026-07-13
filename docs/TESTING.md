# Manual Testing Guide

This guide is written for an **LLM agent doing manual, exploratory testing** of a
running EV Charge Controller stack with `curl` (the API) and a browser via Chrome
DevTools MCP (the UI). It documents how to bring the stack up, authenticate, drive
every endpoint, simulate hardware with the mock Tasmota server, and verify the UI.

For how the system is built, see [`ARCHITECTURE.md`](ARCHITECTURE.md).

> **Convention:** examples assume the dev stack (`make dev`), with the API on
> `http://localhost:8080`, the UI on `http://localhost:3000`, and mock Tasmota
> instances on `http://localhost:8081`+. The API is also reachable through the UI's
> proxy at `http://localhost:3000/api/...`.

---

## 1. Bring up a known-good environment

```bash
make dev      # API + UI + mock Tasmota + Mosquitto, hot-reload
make seed     # bootstrap: create the test user + demo history
```

There are two seeders. **For manual testing, use `POST /api/reset` as your
canonical baseline**: it is deterministic (stable IDs), and it is the dataset every
ID in this guide refers to. `make seed` is a lighter CLI bootstrap (random IDs, no
12V plug / carbon-aware schedule / off-peak window); run it once on a fresh DB so
the test user exists, then authenticate ([section 2](#2-authenticate-for-curl)) and
reset:

```bash
curl -s -X POST -H "$AUTH" http://localhost:8080/api/reset   # 200, deterministic baseline
```

**`POST /api/reset` dataset (canonical, deterministic IDs):**

| Thing | Value |
|-------|-------|
| User | `test@example.com` / `password123` |
| Vehicles | My RM1 (`...0020`), My RM1S (`...0021`), My RM2 (`...0022`), all at 20% current / 80% target |
| Plugs | Garage Plug (`...0010`, charging -> My RM1), Driveway Plug (`...0011`, charging -> My RM1S), My RM1 12V (`...0012`, maintenance -> My RM1) |
| Schedules | Daily 06:00 on Garage Plug; carbon-aware 22:00 window / ready-by 07:00 on Driveway Plug |
| Tariff | 24.83 p/kWh peak, 7.50 p/kWh off-peak (00:30-05:30) |
| History | ~370 completed + 2 cancelled sessions over 180 days, with power readings, SOC snapshots, carbon and cost data |

(Full UUIDs are `00000000-0000-0000-0000-0000000000XX`; only the last bytes are shown
above.) `POST /api/reset` preserves the `users` and `refresh_tokens` tables, so any
token you already hold stays valid across a reset.

---

## 2. Authenticate for curl

Login returns the access token in the JSON body (the refresh token is set as an
httpOnly cookie). Capture the token into a shell variable:

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","password":"password123"}' \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["accessToken"])')

AUTH="Authorization: Bearer $TOKEN"
```

Every endpoint except `/health`, `/api/auth/register`, `/api/auth/login`, and
`/api/auth/refresh` requires `Authorization: Bearer <token>`. The access token
lasts 1 hour; if calls start returning `401`, log in again.

Useful auth checks:

```bash
curl -s -H "$AUTH" http://localhost:8080/api/auth/me            # 200 + user
curl -s -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@example.com","password":"wrong"}'          # 401 invalid-credentials
```

---

## 3. API reference & curl recipes

All responses use JSON. Errors follow RFC 7807 Problem Details (see
[section 6](#6-error-format)). IDs in path examples are placeholders; substitute
real IDs from list responses.

### Bikes (vehicles)

The catalog (`vehicle-models`) is global and read-only; a `vehicle` is the user's
instance of a model and carries live SoC and preferences.

| Method | Endpoint | Body / Params | Success |
|--------|----------|---------------|---------|
| GET | `/api/vehicle-models` | - | 200 + catalog array |
| GET | `/api/vehicles` | - | 200 + array |
| POST | `/api/vehicles` | `{modelId, name?}` | 201 + vehicle |
| GET | `/api/vehicles/{id}` | - | 200 + vehicle (catalog merged in) |
| PATCH | `/api/vehicles/{id}` | `{name?, currentPercent?, targetPercent?, notifyChargeComplete?, notifyChargerOffline?, notifyMaintenanceOffline?}` | 204 |
| DELETE | `/api/vehicles/{id}` | - | 204 |
| GET | `/api/vehicles/{id}/stats` | `?range=` | 200 + stats |
| GET | `/api/vehicles/stats` | - | 200 + all-vehicle stats |

```bash
# List bikes and grab the first id
curl -s -H "$AUTH" http://localhost:8080/api/vehicles | python3 -m json.tool
VID=00000000-0000-0000-0000-000000000020   # My RM1 (seeded, deterministic)

# Set current SoC to 30%
curl -s -X PATCH -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"currentPercent":30}' http://localhost:8080/api/vehicles/$VID   # 204
```

Edge cases to probe: `currentPercent > targetPercent` -> `400 target-must-be-higher`;
changing `currentPercent` while a session is active -> `409 session-active`;
duplicate name -> `409 duplicate-name`.

### Plugs

| Method | Endpoint | Body / Params | Success |
|--------|----------|---------------|---------|
| GET | `/api/plugs` | - | 200 + array |
| POST | `/api/plugs` | `{name, mqttTopic?, vehicleId?, type?}` (`type`: `charging`\|`maintenance`) | 201 + `{plug}` |
| GET | `/api/plugs/{id}` | - | 200 + plug |
| PATCH | `/api/plugs/{id}` | `{name?, vehicleId?}` | 200 + plug |
| DELETE | `/api/plugs/{id}` | - | 204 |
| PATCH | `/api/plugs/{id}/power` | `{on}` (maintenance plugs only) | 200 + plug |
| POST | `/api/plugs/{id}/configure` | `{tasmotaIP?, tasmotaPassword?}` | 200 + `{consoleCommands}` |

```bash
PID=00000000-0000-0000-0000-000000000010   # Garage Plug (seeded)
curl -s -H "$AUTH" http://localhost:8080/api/plugs/$PID | python3 -m json.tool
```

Notes:
- `type` defaults to `charging`. A `maintenance` plug is the only type whose relay
  you toggle directly via `/power`; charging plugs are controlled through charge
  sessions. Toggling power on a charging plug -> `400 wrong-plug-type`.
- `POST /api/plugs/{id}/configure` with a `tasmotaIP` (e.g. `localhost:8081`)
  pushes MQTT config to that device over HTTP; without one it just returns the
  console commands to paste manually.

### Charge sessions (vehicle-centric)

Sessions are keyed by **vehicle**. There is at most one in-progress session per
bike. Stop and update operations take the vehicle id as a query parameter.

| Method | Endpoint | Body / Params | Success |
|--------|----------|---------------|---------|
| GET | `/api/charge-sessions` | `?vehicleId=` | 200 + session, or 204 (none active) |
| POST | `/api/charge-sessions` | `{vehicleId, plugId?, startPercent, targetPercent}` | 201 + session |
| PATCH | `/api/charge-sessions` | `?vehicleId=` + `{targetPercent}` (update) or `{status:"stopped"}` (stop) | 204 |
| DELETE | `/api/charge-sessions/{id}` | - | 204 (completed/cancelled only) |

```bash
# Start charging My RM1 from 20% to 80%
curl -s -X POST -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"vehicleId\":\"$VID\",\"startPercent\":20,\"targetPercent\":80}" \
  http://localhost:8080/api/charge-sessions                              # 201 + session

# Poll the active session
curl -s -H "$AUTH" "http://localhost:8080/api/charge-sessions?vehicleId=$VID"

# Raise the target to 90%
curl -s -X PATCH -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"targetPercent":90}' \
  "http://localhost:8080/api/charge-sessions?vehicleId=$VID"             # 204

# Stop charging
curl -s -X PATCH -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"status":"stopped"}' \
  "http://localhost:8080/api/charge-sessions?vehicleId=$VID"             # 204
```

Edge cases: starting a second session for the same bike -> `409 session-already-active`;
`startPercent >= targetPercent` -> `400 target-must-be-higher`; missing `vehicleId`
on stop/update -> `400 missing-vehicle-id`; deleting an active session -> `409`.

### Schedules (per plug)

| Method | Endpoint | Body | Success |
|--------|----------|------|---------|
| GET | `/api/plugs/{id}/schedule` | - | 200 + schedule, or 404 |
| PATCH | `/api/plugs/{id}/schedule` | daily: `{type:"daily", time:"HH:MM", enabled}`; carbon-aware: `{type:"carbon_aware", windowStart:"HH:MM", windowEnd:"HH:MM", enabled}` | 200 + schedule |

```bash
# Daily timer at 06:30
curl -s -X PATCH -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"type":"daily","time":"06:30","enabled":true}' \
  http://localhost:8080/api/plugs/$PID/schedule

# Carbon-aware: charge any time after 22:00, ready by 07:00
curl -s -X PATCH -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"type":"carbon_aware","windowStart":"22:00","windowEnd":"07:00","enabled":true}' \
  http://localhost:8080/api/plugs/$PID/schedule
```

`windowEnd` is the ready-by deadline. Maintenance plugs reject schedules
(`400 maintenance-plug-schedule`). Bad time format -> `400 invalid-schedule-time`.

### Tariff, carbon, history, charts

| Method | Endpoint | Params | Purpose |
|--------|----------|--------|---------|
| GET | `/api/tariff-settings` | - | Current tariff |
| PUT | `/api/tariff-settings` | `{baseRatePence, offPeakWindows:[{start,end,ratePence}]}` | Replace tariff |
| GET | `/api/carbon-intensity` | - | `{forecast, actual, index}` from the UK API (live) |
| GET | `/api/history` | `?vehicleId=&date=&limit=&offset=` | Paginated session history |
| GET | `/api/power-readings` | `?sessionId=` or `?vehicleId=` | Power chart series |
| GET | `/api/soc-snapshots` | `?sessionId=` or `?vehicleId=` | SoC chart series |
| GET | `/api/schemas` | - | Go struct schemas (reflection export) |

```bash
curl -s -H "$AUTH" http://localhost:8080/api/tariff-settings | python3 -m json.tool
curl -s -H "$AUTH" "http://localhost:8080/api/history?vehicleId=$VID&limit=5" | python3 -m json.tool
curl -s -H "$AUTH" http://localhost:8080/api/carbon-intensity      # 503 if API offline
```

### Push notifications

| Method | Endpoint | Body | Notes |
|--------|----------|------|-------|
| POST | `/api/push-subscriptions` | `{endpoint, p256dhKey, authKey}` | 201 |
| DELETE | `/api/push-subscriptions` | `{endpoint}` | 204 |

Push routes only exist when `VAPID_PUBLIC_KEY` and `VAPID_PRIVATE_KEY` are set;
otherwise they return `404`. Push delivery needs a secure context (HTTPS).

### Health & reset

```bash
curl -s http://localhost:8080/health                 # liveness (no auth)
curl -s -X POST -H "$AUTH" http://localhost:8080/api/reset   # dev-only: reset to seed
```

---

## 4. Simulating hardware with mock Tasmota

The mock Tasmota server stands in for a physical plug. With MQTT configured (the
seed does this automatically), the API drives the relay over MQTT; the HTTP
surface below is for direct inspection and forcing state during a test.

| Endpoint | Purpose |
|----------|---------|
| `GET /health` | Mock liveness |
| `GET /reset` | Reset relay off, zero cumulative energy |
| `GET /cm?cmnd=Power%20ON` / `OFF` / `TOGGLE` | Force the relay |
| `GET /cm?cmnd=ENERGY` | Read energy/power |
| `GET /cm?cmnd=STATUS+10` | Detailed sensor readings |
| `GET /cm?cmnd=EnergyTotal%202.5` | Set the cumulative meter (kWh) - advance a charge session deterministically in tests |
| `GET /cm?cmnd=SensorRetain%201` | Publish tele/SENSOR retained (also accepted over MQTT cmnd, like `Status 10`) |
| `GET /status` | Legacy status page |

```bash
curl -s "http://localhost:8081/cm?cmnd=ENERGY"
curl -s "http://localhost:8081/reset"
```

**Energy model:** the mock follows a CC/CV curve (peak power 0-15%, taper 15-80%,
strong taper 80-100%) with a small sinusoidal fluctuation, configurable via
`POWER_WATTS`, `VOLTAGE`, and `FREQUENCY`. This is what makes the live charts and
SoC estimate look realistic.

**Simulating an offline plug:** stop one of the mock-tasmota containers (or block
its MQTT). After the 60 s LWT debounce, the plug is marked offline, its active
session is cancelled, and an offline notification fires. Bring it back to verify
recovery.

---

## 5. Browser testing with Chrome DevTools MCP

Use the browser to verify rendering, interaction, and live updates that curl
cannot exercise (the gauge, charts, modals, charging animation).

### Connect and authenticate

1. `make dev && make seed` first.
2. Navigate to `http://localhost:3000`. If redirected to `/login`, sign in with
   `test@example.com` / `password123`.
3. Take a snapshot (`take_snapshot`) to get element uids before interacting; take a
   screenshot (`take_screenshot`) to verify appearance.

### Pages to verify

| Route | What to check |
|-------|---------------|
| `/dashboard` | Vehicle chips, speedometer gauge, status bar, charge control, live charts |
| `/vehicles` | Bike list with lifetime stats (energy, cost, CO2) |
| `/vehicles/{id}` | Per-bike detail, stats, charts |
| `/history` | Session list, status badges, click a session for detail charts |
| `/login` | Form validation, error messaging on bad credentials |

### Interacting

- **Buttons / modals:** `click` with a uid from `take_snapshot`. Open Settings
  (gear) to manage plugs and notification prefs; open the schedule modal to set a
  daily or carbon-aware schedule.
- **Speedometer gauge:** the gauge has draggable start, current, and target
  markers. Use `drag` between coordinates on the canvas, or set values via the
  vehicle PATCH endpoint and reload. Verify markers cannot cross illegally (target
  must stay above current) and that values snap.
- **Charts:** `hover` over a chart to surface the tooltip; confirm values track the
  underlying readings.
- **Live charging:** start a session (UI START button or curl), then watch the
  dashboard. Within ~5 s (the poll interval) power draw, energy added, and the
  needle should begin updating.

### Hiding dev overlays for clean screenshots

In dev mode two floating widgets sit at the bottom of the viewport: the **Next.js
dev indicator** (bottom-right) and the **React Query Devtools** button
(bottom-left). Hide them before capturing screenshots by injecting CSS with
`evaluate_script`:

```js
const css = `nextjs-portal,[data-nextjs-toolbar],.tsqd-parent-container,
  .tsqd-open-btn-container,[aria-label*="Tanstack"],[aria-label*="React Query"]{
  display:none !important;visibility:hidden !important;}`;
const s = document.createElement('style');
s.textContent = css;
document.head.appendChild(s);
```

---

## 6. Error format

All error responses use RFC 7807 Problem Details:

```json
{"type":"about:blank#session-already-active","title":"Conflict","status":409,"detail":"..."}
```

The `type` fragment is a stable error id you can assert on. Common ids:

| `type` fragment | Status | Meaning |
|-----------------|--------|---------|
| `#missing-fields` / `#missing-vehicle-id` / `#missing-name` | 400 | Required field absent |
| `#invalid-start-percent` / `#invalid-target-percent` / `#invalid-percent` | 400 | Percent out of `[0,100]` |
| `#target-must-be-higher` | 400 | start >= target |
| `#invalid-schedule-time` / `#window-required` / `#window-equal` | 400 | Bad schedule input |
| `#wrong-plug-type` / `#maintenance-plug-schedule` | 400 | Operation not valid for plug type |
| `#invalid-credentials` | 401 | Bad email/password |
| `#invalid-token` / `#missing-token` | 401 | Refresh token bad/absent |
| `#unauthorized` | 401 | Missing/invalid access token |
| `#vehicle-not-found` / `#not-found` / `#user-not-found` | 404 | Unknown resource |
| `#no-active-session` | 404 | No session to stop/update |
| `#email-taken` / `#duplicate-name` | 409 | Uniqueness conflict |
| `#session-already-active` / `#session-active` | 409 | Concurrent-state conflict |
| `#relay-control-failed` / `#mqtt-unavailable` | 503 | Hardware/MQTT unreachable |
| `#internal-error` | 500 | Unexpected server error |

---

## 7. Resetting between scenarios

A test that mutates state should start clean. Either:

```bash
make seed                                    # full reset + reseed (CLI)
curl -s -X POST -H "$AUTH" http://localhost:8080/api/reset   # same, over the API
```

Both reset mock Tasmota energy state and re-provision MQTT, then wait for plugs to
report online, so the next scenario starts from a known, fully-connected baseline.
