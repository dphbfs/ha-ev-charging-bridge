#!/usr/bin/env bash
set -euo pipefail

source /usr/lib/bashio/bashio

CONFIG_FILE=/data/bridge.yaml

charger_id="$(bashio::config 'charger_id')"
charger_name="$(bashio::config 'charger_name')"
evse_id="$(bashio::config 'evse_id')"
connector_id="$(bashio::config 'connector_id')"
meter_id="$(bashio::config 'meter_id')"
power_meter_id="$(bashio::config 'power_meter_id')"
energy_meter_id="$(bashio::config 'energy_meter_id')"
power_entity_id="$(bashio::config 'power_entity_id')"
energy_entity_id="$(bashio::config 'energy_entity_id')"
availability_entity_id="$(bashio::config 'availability_entity_id')"
availability_available_state="$(bashio::config 'availability_available_state')"
availability_unavailable_state="$(bashio::config 'availability_unavailable_state')"
availability_unavailable_after="$(bashio::config 'availability_unavailable_after')"
plug_entity_id="$(bashio::config 'plug_entity_id')"
start_threshold_w="$(bashio::config 'start_threshold_w')"
start_duration="$(bashio::config 'start_duration')"
stop_threshold_w="$(bashio::config 'stop_threshold_w')"
stop_duration="$(bashio::config 'stop_duration')"
stop_plug_state="$(bashio::config 'stop_plug_state')"

cat > "${CONFIG_FILE}" <<EOF
home_assistant:
  url: http://supervisor/core
  token: \${HA_TOKEN}
  event_types:
    - state_changed

chargers:
  - charger_id: "${charger_id}"
    charger_name: "${charger_name}"
    evse_id: "${evse_id}"
    connector_id: "${connector_id}"
    meter_id: "${meter_id}"
    entities:
      power_w: "${power_entity_id}"
      energy_kwh: "${energy_entity_id}"
      availability: "${availability_entity_id}"
      plug: "${plug_entity_id}"
    availability:
      entity_id: "${availability_entity_id}"
      available_state: "${availability_available_state}"
      unavailable_state: "${availability_unavailable_state}"
      unavailable_after: ${availability_unavailable_after}
    start:
      type: power_threshold
      entity_id: "${power_entity_id}"
      threshold_w: ${start_threshold_w}
      duration: ${start_duration}
    stop:
      - type: power_threshold
        entity_id: "${power_entity_id}"
        threshold_w: ${stop_threshold_w}
        duration: ${stop_duration}
      - type: device_offline
        entity_id: "${plug_entity_id}"
        state: "${stop_plug_state}"
        duration: ${stop_duration}
    meters:
      - meter_id: "${power_meter_id}"
        entity_id: "${power_entity_id}"
        unit: W
        aggregation: average
        outside_session_storage: drop
      - meter_id: "${energy_meter_id}"
        entity_id: "${energy_entity_id}"
        unit: kWh
        aggregation: last
        outside_session_storage: drop

retention:
  meter_values: 90d
  lifecycle_events: 365d
  raw_events: 7d

runtime:
  database_path: /data/bridge.db
  log_file: /data/app.log
  api_addr: 0.0.0.0:8099
EOF

export HA_URL="http://supervisor/core"
export HA_TOKEN="${SUPERVISOR_TOKEN}"
export CONFIG_FILE
export DATABASE_PATH="/data/bridge.db"
export LOG_FILE="/data/app.log"
export API_ADDR="0.0.0.0:8099"
export FRONTEND_DIR="/app/frontend"

exec /usr/local/bin/ha-ev-charging-bridge
