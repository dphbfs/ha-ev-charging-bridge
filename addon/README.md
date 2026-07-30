# EV Charging Bridge Add-on

This proof-of-concept add-on runs the Home Assistant EV Charging Bridge backend and serves the V2 sessions UI through Home Assistant Ingress.

The add-on uses `homeassistant_api: true`, so users do not need to create a long-lived access token. The bridge connects to Home Assistant through the Supervisor API proxy using the injected `SUPERVISOR_TOKEN`.

## Install

Add this repository as a custom Home Assistant add-on repository, install **EV Charging Bridge**, configure the entity IDs, then start the add-on.

After startup, click **OPEN WEB UI** to inspect charging sessions.
