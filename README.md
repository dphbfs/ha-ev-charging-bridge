# Home Assistant EV Charging Bridge

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
HA_URL=http://homeassistant.local:8123
HA_TOKEN=your-home-assistant-token
DEVICE_CONFIG=devices.yaml
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

Charger-related Home Assistant entities are configured in `devices.yaml`.

Connection settings and runtime paths can be provided through `.env` or process environment variables. Do not commit real Home Assistant tokens.

## License

MIT
