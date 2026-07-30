# Home Assistant EV Charging Bridge

[![Go](https://github.com/dphbfs/ha-ev-charging-bridge/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/dphbfs/ha-ev-charging-bridge/actions/workflows/go.yml?query=branch%3Amain)
[![Coverage](https://img.shields.io/github/actions/workflow/status/dphbfs/ha-ev-charging-bridge/go.yml?branch=main&label=coverage)](https://github.com/dphbfs/ha-ev-charging-bridge/actions/workflows/go.yml?query=branch%3Amain)
[![Latest Release](https://img.shields.io/github/v/release/dphbfs/ha-ev-charging-bridge)](https://github.com/dphbfs/ha-ev-charging-bridge/releases/latest)

Home Assistant EV Charging Bridge is a Go application that listens to Home Assistant websocket events and turns configured charger-related entity updates into EV charging session events.

The project is currently a proof of concept. V1 focuses on Home Assistant ingress, charger/session detection, meter values, and local persistence.

## Install From Release

Install the latest released Linux binary and systemd service:

```sh
curl -fsSL https://github.com/dphbfs/ha-ev-charging-bridge/releases/latest/download/install.sh | sh
```

Install a specific release:

```sh
curl -fsSL https://github.com/dphbfs/ha-ev-charging-bridge/releases/latest/download/install.sh | sh -s -- v0.0.1-alpha
```

The installer downloads the matching release asset for the current Linux architecture, verifies its SHA-256 checksum, installs the binary to `/usr/local/bin`, creates `~/.ha-ev-charging-bridge/config.yaml` if needed, and enables the `ha-ev-charging-bridge` systemd service.

Edit `~/.ha-ev-charging-bridge/config.yaml`, then start the service:

```sh
sudo systemctl start ha-ev-charging-bridge
```

## Run From Binary

Build the binary:

```sh
go build -o ha-ev-charging-bridge .
```

Create local configuration from the example:

```sh
cp .env.example .env
```

Set at least:

```sh
HA_URL=http://home-assistant.example.local:8123
HA_TOKEN=your-home-assistant-token
CONFIG_FILE=bridge.yaml
```

Run the bridge:

```sh
./ha-ev-charging-bridge
```

The current POC writes SQLite runtime data under `var/` and logs under `log/` by default.

## Run From Source With Docker Compose

After creating `.env`, run:

```sh
docker compose up --build
```

Use this mode when developing or testing the app from source in a containerized environment.

## Home Assistant Add-on POC

This repository includes a proof-of-concept Home Assistant add-on under `addon/`. Home Assistant OS or Supervised users can add this repository as a custom add-on repository from **Settings > Add-ons > Add-on Store > Repositories**.

For the current development branch, add the repository with the branch suffix:

```text
https://github.com/dphbfs/ha-ev-charging-bridge#dev
```

The add-on uses Home Assistant Ingress and `homeassistant_api: true`, so it does not require a user-created long-lived access token. It uses the Supervisor-provided token internally and stores runtime data in the add-on persistent `/data` volume.

## Configuration

V1 application settings are configured in `bridge.yaml`, including charger entity mappings. Home Assistant connection values use environment interpolation, so secrets such as `HA_TOKEN` stay in `.env` or the process environment.

Connection settings and runtime paths can be provided through `.env` or process environment variables. Do not commit real Home Assistant tokens.

## HTTP API

The bridge exposes a read-only sessions API for the V2 frontend. Configure the listen address with `API_ADDR` or `-api-addr`; use an empty value to disable it. The default is `127.0.0.1:8080`.

Set `FRONTEND_DIR` or `-frontend-dir` to a built frontend directory, such as `frontend/dist`, to serve the V2 UI from the same HTTP listener.

Endpoints:

- `GET /api/v1/sessions?limit=50&offset=0&sort=newest&search=&charger_id=&status=&started_after=&started_before=` lists completed sessions. Date filters use RFC3339 timestamps.
- `GET /api/v1/sessions/active` lists active sessions.
- `GET /api/v1/sessions/{session_id}` returns a completed session with meter values and lifecycle events.
- `GET /api/v1/sessions/{session_id}/meter-values` returns raw persisted meter values.
- `GET /api/v1/sessions/{session_id}/events` returns persisted session lifecycle events.

Current limitations: status is derived from stored session state, so completed rows are reported as `completed` and active rows as `charging`. More specific stopped/interrupted status mapping will require persisted stop classification.

## License

MIT
