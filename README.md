# Home Assistant EV Charging Bridge

[![Go](https://github.com/dphbfs/ha-ev-charging-poc/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/dphbfs/ha-ev-charging-poc/actions/workflows/go.yml?query=branch%3Amain)
[![Coverage](https://img.shields.io/github/actions/workflow/status/dphbfs/ha-ev-charging-poc/go.yml?branch=main&label=coverage)](https://github.com/dphbfs/ha-ev-charging-poc/actions/workflows/go.yml?query=branch%3Amain)

Home Assistant EV Charging Bridge is a Go application that listens to Home Assistant websocket events and turns configured charger-related entity updates into EV charging session events.

The project is currently a proof of concept. V1 focuses on Home Assistant ingress, charger/session detection, meter values, and local persistence.

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
CONFIG_FILE=config.yaml
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

## Configuration

V1 application settings are configured in `config.yaml`, including charger entity mappings. Home Assistant connection values use environment interpolation, so secrets such as `HA_TOKEN` stay in `.env` or the process environment.

Connection settings and runtime paths can be provided through `.env` or process environment variables. Do not commit real Home Assistant tokens.

## HTTP API

The bridge exposes a read-only sessions API for the V2 frontend. Configure the listen address with `API_ADDR` or `-api-addr`; use an empty value to disable it. The default is `127.0.0.1:8080`.

Endpoints:

- `GET /api/v1/sessions?limit=50&offset=0&sort=newest&search=&charger_id=&status=&started_after=&started_before=` lists completed sessions. Date filters use RFC3339 timestamps.
- `GET /api/v1/sessions/active` lists active sessions.
- `GET /api/v1/sessions/{session_id}` returns a completed session with meter values and lifecycle events.
- `GET /api/v1/sessions/{session_id}/meter-values` returns raw persisted meter values.
- `GET /api/v1/sessions/{session_id}/events` returns persisted session lifecycle events.

Current limitations: status is derived from stored session state, so completed rows are reported as `completed` and active rows as `charging`. More specific stopped/interrupted status mapping will require persisted stop classification.

## License

MIT
