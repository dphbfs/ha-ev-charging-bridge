# EV Charging Bridge Add-on Documentation

## Configuration

Set the charger identity and Home Assistant entity IDs in the add-on options.

- `charger_id`: stable internal charger ID.
- `charger_name`: display name shown in the UI.
- `evse_id`: EVSE identifier.
- `connector_id`: connector identifier.
- `meter_id`: meter identifier.
- `power_entity_id`: Home Assistant entity reporting charger power in watts.
- `energy_entity_id`: Home Assistant entity reporting cumulative charger energy in kWh.
- `availability_entity_id`: entity used for availability checks.
- `plug_entity_id`: entity used for the device-offline stop rule.
- `start_threshold_w`: power threshold that starts a charging session.
- `start_duration`: duration power must remain above threshold.
- `stop_threshold_w`: power threshold that can stop a charging session.
- `stop_duration`: duration power/offline state must remain before stopping.

## Runtime Data

The add-on stores SQLite data in `/data/bridge.db` and logs in `/data/app.log`, both inside the add-on persistent data volume.

## Authentication

No Home Assistant long-lived token is required. The add-on uses the Supervisor-provided `SUPERVISOR_TOKEN` and calls Home Assistant through `http://supervisor/core`.

## Current POC Limitations

- Only one charger is configurable through add-on options.
- UI is served through ingress on port `8099`.
- The add-on has not yet been validated on a real Supervisor install in this branch.
