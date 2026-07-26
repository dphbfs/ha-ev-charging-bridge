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

## License

MIT
