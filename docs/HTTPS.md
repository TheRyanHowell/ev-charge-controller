# HTTPS / TLS Setup

Push notifications and PWA install require a secure context. Optional HTTPS is
provided by a Caddy reverse proxy using the Let's Encrypt DNS-01 challenge via
Cloudflare. Caddy gets its own MAC address and IP on the physical LAN via a
Docker macvlan network, so it is reachable on your subnet without host port
binding.

Without HTTPS, the app runs fine on `http://localhost:3000` -- only push
notifications and PWA install are unavailable.

## Prerequisites

- A domain managed by Cloudflare
- A Cloudflare API token with **DNS:Edit** and **Zone:Read** permissions for
  that domain
- An available IP address on your LAN subnet to assign to Caddy
- A spare MAC address to assign to the Caddy container (pick any locally
  administered address, e.g. `02:42:c0:a8:00:XX`)

## Steps

### 1. Create the macvlan network (one-time)

Adjust `--subnet`, `--gateway`, and `-o parent` to match your network
interface and router:

```bash
docker network create -d macvlan \
  --subnet=192.168.0.0/24 \
  --gateway=192.168.0.1 \
  -o parent=enp39s0 \
  docker-macvlan
```

### 2. Configure `.env`

```dotenv
ENABLE_HTTPS=true
CADDY_DOMAIN=ev.example.com          # your domain
CLOUDFLARE_API_KEY=your-token-here
CADDY_IP=192.168.0.20                # unused IP on your LAN
CADDY_MAC=02:42:c0:a8:00:20          # unique MAC for the Caddy container
CORS_ORIGIN=https://ev.example.com   # update from http://localhost:3000
```

### 3. Reserve the IP in your router

Create a static DHCP lease for `CADDY_MAC` -> `CADDY_IP` in your router so
the address is stable across reboots.

### 4. Add a DNS record

Create an `A` record pointing `CADDY_DOMAIN` to `CADDY_IP` in Cloudflare.

### 5. Start

```bash
make start
```

`make start` reads `ENABLE_HTTPS` from `.env` at parse time. When `true`, it
includes `docker-compose.caddy.yml`, which adds Caddy to the macvlan network
and starts the Let's Encrypt DNS-01 challenge automatically.

## How it works

Caddy joins two networks:

- **macvlan** (`docker-macvlan`) -- gives Caddy its own MAC/IP on the physical
  LAN, making it directly reachable at `CADDY_IP` without host port binding.
- **internal** (Docker bridge) -- lets Caddy resolve `ui:3000` via Docker DNS
  and proxy requests to the Next.js server.

The UI and API containers are only on the `internal` bridge and are not
directly exposed on the LAN.

## Compose file layout

| File | Purpose |
|------|---------|
| `docker-compose.yml` | Base services (mosquitto, api, ui) |
| `docker-compose.caddy.yml` | Caddy service + macvlan network (HTTPS only) |
| `docker-compose.no-caddy.yml` | Host port bindings for api/ui (no-HTTPS) |
| `docker-compose.dev.yml` | Dev overrides (hot-reload, mock Tasmota) |

`make start` automatically selects between `caddy` and `no-caddy` overlays
based on `ENABLE_HTTPS`.
