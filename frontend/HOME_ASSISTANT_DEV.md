# Home Assistant Dev Loading Workflow

This document describes how to load the V2 Lit + TypeScript frontend inside Home Assistant while the UI is still in development.

## Recommendation

Use a temporary Home Assistant app/add-on with Ingress enabled.

Ingress is the closest development match for the intended product direction because:

- The UI opens from Home Assistant with the normal `OPEN WEB UI` flow.
- Home Assistant handles user authentication before the app is shown.
- The app is tested behind the same style of proxied path that production add-on packaging will use.
- The workflow exposes path, asset, websocket, and refresh issues earlier than a plain browser tab.

Use direct Vite browser access for fast layout work, but validate important changes through Ingress before closing UI tasks.

## Prerequisites

- A Home Assistant instance with Supervisor support, such as Home Assistant OS or Supervised.
- Local access to the Home Assistant `addons` folder or a development add-on repository configured in Home Assistant.
- Docker available to Home Assistant/Supervisor.
- This repository checked out somewhere you can copy or mount from.

Do not commit real Home Assistant tokens, private hostnames, or local-only IP addresses.

## Frontend Development Server

From this repository:

```sh
cd frontend
npm install
npm run dev -- --host 0.0.0.0 --port 8099
```

Port `8099` is recommended because Home Assistant Ingress defaults to `8099` when `ingress_port` is not set. The production add-on can choose a different port, but the add-on `config.yaml` must match it.

## Preferred Workflow: Dev Add-On Running Vite

Create a temporary add-on folder in your Home Assistant add-ons location. The exact host path depends on your installation, but the add-on directory should contain at least these files:

```text
ha-ev-charging-bridge-ui-dev/
  config.yaml
  Dockerfile
  run.sh
  frontend/
```

Example `config.yaml`:

```yaml
name: "EV Charging Bridge UI Dev"
version: "0.0.1-dev"
slug: "ev_charging_bridge_ui_dev"
description: "Development shell for the EV Charging Bridge frontend"
arch:
  - amd64
  - aarch64
startup: application
boot: manual
ingress: true
ingress_port: 8099
panel_title: "EV Charging"
panel_icon: "mdi:ev-station"
init: false
```

Example `Dockerfile`:

```dockerfile
FROM node:22-alpine

WORKDIR /app
COPY frontend/ /app/frontend/
COPY run.sh /run.sh
RUN chmod +x /run.sh

CMD ["/run.sh"]
```

Example `run.sh`:

```sh
#!/usr/bin/env sh
set -eu

cd /app/frontend
npm install
npm run dev -- --host 0.0.0.0 --port 8099
```

Copy this repository's `frontend/` folder into the temporary add-on folder before building or rebuilding the add-on.

One simple manual option during development is:

```sh
mkdir -p /path/to/home-assistant/addons/ha-ev-charging-bridge-ui-dev/frontend
rsync -a --delete frontend/ /path/to/home-assistant/addons/ha-ev-charging-bridge-ui-dev/frontend/
```

Replace `/path/to/home-assistant` with the correct local Home Assistant data path for your development machine. If your Home Assistant instance reads add-ons from a separate repository folder, use that folder instead.

After creating the add-on:

1. In Home Assistant, go to **Settings > Add-ons > Add-on Store**.
2. Use the menu to reload local add-ons if needed.
3. Install **EV Charging Bridge UI Dev**.
4. Start the add-on.
5. Open the add-on logs and confirm Vite is listening on `0.0.0.0:8099`.
6. Click **OPEN WEB UI**.

## Verifying Dev Loading

Inside Home Assistant, verify:

- The overview route loads.
- The detail route can be reached from overview navigation.
- `components.html` can be opened by appending the filename to the ingress URL if needed.
- Theme switching works.
- Mobile widths can be inspected with browser dev tools.
- Browser refresh still loads the app.
- No asset requests escape the ingress path.

The frontend uses relative script paths and Vite `base: "./"` so built assets and dev entrypoints are more likely to work behind Home Assistant's ingress path.

## Development Loop

For fast iteration:

1. Edit files in `frontend/`.
2. Sync the folder into the temporary add-on `frontend/` folder.
3. Rebuild and restart the add-on.
4. Refresh the Home Assistant Web UI tab.

Vite hot reload may work when the websocket path survives ingress proxying, but do not rely on it as the only validation. Browser refresh is the dependable path.

## Alternative Workflows Considered

### Direct Vite Browser Tab

Run `npm run dev` and open the Vite URL directly. This is fastest for layout work but does not test Home Assistant ingress, path rewriting, or the Home Assistant shell.

Use this for quick component development only.

### Iframe Panel

An iframe-style panel can point Home Assistant at an external development server, but this is not recommended as the primary workflow for this project. It does not model add-on ingress closely, may run into browser security/CORS/frame constraints, and the previously documented `panel_iframe` integration is not a reliable forward-looking target.

### Custom Panel Or Lovelace Card

Custom panels and Lovelace cards are useful for frontend integrations that run inside Home Assistant's frontend resource system. This project is currently targeting an app/add-on UI, so using a custom panel first would optimize for the wrong packaging model.

## Common Issues

### Blank Page Or Missing JavaScript

Check browser dev tools for asset URLs that start at `/src/...` or `/assets/...` without the ingress prefix. Frontend entrypoints should use relative paths and Vite should keep `base: "./"`.

### Add-On Starts But OPEN WEB UI Fails

Confirm the server listens on `0.0.0.0:8099`, not `localhost`. Confirm `ingress_port` matches the Vite port.

### Vite HMR Does Not Work

Use a browser refresh. HMR websocket behavior can be affected by ingress path rewriting. This is acceptable for the dev workflow.

### Stale UI

Rerun the sync command, rebuild the add-on, and restart it. If only browser assets appear stale, hard-refresh the Home Assistant tab.

### CORS Errors

The frontend should not need CORS for static mock data. Later API integration should prefer same-origin ingress paths or explicitly configured backend CORS for local development.

## Production Packaging Notes

The dev add-on runs Vite and installs dependencies at startup. Production packaging should not do that.

For production:

- Run `npm run build` during image build.
- Serve `frontend/dist/` from the Go application or a small static server.
- Keep `ingress: true` and set `ingress_port` to the production HTTP server port.
- Ensure frontend API calls use relative same-origin paths when running behind ingress.
- Do not include development-only source syncing in the production image.

## Validation Status

This document captures the recommended workflow and configuration shape from Home Assistant app/add-on ingress documentation. It still needs to be validated against a real Home Assistant Supervisor instance before production packaging work begins.
