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

The current POC writes runtime data under `var/` and logs under `log/` by default.

## Run From Source With Docker Compose

After creating `.env`, run:

```sh
docker compose up --build
```

Use this mode when developing or testing the app from source in a containerized environment.

## Configuration

V1 application settings are configured in `config.yaml`. Home Assistant connection values use environment interpolation, so secrets such as `HA_TOKEN` stay in `.env` or the process environment.

The current runtime still reads `devices.yaml` for the legacy smart-plug POC path while the v1 package split is underway.

Connection settings and runtime paths can be provided through `.env` or process environment variables. Do not commit real Home Assistant tokens.

## License

MIT
