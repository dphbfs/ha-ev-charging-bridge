# Gathering Details

This document captures pre-plan discovery notes for the Home Assistant EV charging bridge. It is not an implementation plan yet.

## High-Level Concept

The complete concept has two larger parts:

1. Home Assistant ingress bridge.
2. Charging history UI and charger configuration surface.

The current discovery focus is the Home Assistant ingress bridge.

## Home Assistant Ingress Bridge

The bridge should answer these core questions:

1. What configured Home Assistant entities represent a charger?
2. What is the current charger state?
3. When did a session start?
4. What meter readings happened during the session?
5. When did the session end?
6. What neutral events should be emitted?

The bridge should emit or expose:

1. Completed sessions.
2. Sessions that are in progress.
3. Lifecycle events as they happen.
4. Meter updates.

For now, one charger maps to one connector.

Configuration should use industry-style names where useful, such as `charger_id`, `evse_id`, `connector_id`, and `meter_id`, because Home Assistant entities are being mapped into charging-domain concepts.

The bridge should persist data with configurable retention. SQLite is preferred over JSONL files for flexibility.

## Standard Direction

The bridge should not implement full OCPI or OCPP at this stage.

The current direction is:

1. Use a neutral internal model.
2. Use OCPP-inspired lifecycle concepts where useful.
3. Use OCPI-inspired naming/history concepts where useful.
4. Avoid committing to either protocol until the surrounding system is better defined.

## Core Concepts

The bridge works mainly with two core concepts:

1. Charger.
2. Session.

### Charger

A charger is a configured set of Home Assistant entities.

Example entity categories:

1. Power consumption in watts.
2. Cumulative energy in kWh.
3. Availability or online/offline state.
4. Optional future fault state.
5. Optional future connector plugged/unplugged state.

### Session

A session is a charging session detected from charger events.

The v1 session lifecycle only needs these states:

1. `charging`
2. `ended`

No separate charger-state/session-state pairing is needed yet.

## Event Model

Home Assistant entity state changes should be mapped to charger events. Charger events then drive session behavior.

Flow:

```text
HA state_changed
  -> configured entity mapping
  -> charger event detection
  -> session lifecycle logic
  -> meter aggregation
  -> SQLite persistence
  -> neutral session/event output
```

## V1 Active Events

The active v1 event vocabulary is:

1. `charger_available`
2. `charger_unavailable`
3. `charging_started`
4. `charging_stopped`
5. `meter_value`
6. `session_started`
7. `session_updated`
8. `session_ended`

## Reserved Future Events

These should be kept in mind for configuration/schema design, but do not need v1 behavior yet:

1. `charger_faulted`
2. `connector_plugged`
3. `connector_unplugged`
4. `charging_suspended`
5. `charging_resumed`

## Event Requirements

### `charger_available`

The smart plug or charger Home Assistant entity is online/available.

This should be configurable per charger.

### `charger_unavailable`

The smart plug or charger Home Assistant entity is offline/unavailable for a configurable amount of time.

If a charger becomes unavailable for the configured duration while a session is active, the active session should end.

Potential end reason:

```text
charger_unavailable
```

### `charger_faulted`

Reserved in config/schema, but not applicable for now.

### `connector_plugged`

Reserved in config/schema, but not applicable for now.

### `connector_unplugged`

Reserved in config/schema, but not applicable for now.

### `charging_started`

Default v1 behavior should use power consumption in watts.

Charging starts when the configured Home Assistant power entity reports consumption above a configurable threshold, optionally for a configurable duration.

Implementation logic should only start charging if there is no existing active session for that charger.

Default detection type:

```text
power threshold
```

Future/comment-only detection type:

```text
energy delta
```

Energy-delta start detection would support smart plugs that do not report power consumption but do report cumulative energy. Example concept: energy changed from X to Y within N minutes, so charging started.

### `charging_suspended`

Reserved in config/schema for now.

This may later represent consumption staying low for a configurable amount of time without ending the session immediately.

### `charging_resumed`

Reserved in config/schema for now.

### `charging_stopped`

Default v1 behavior should use power consumption in watts.

Charging stops when the configured Home Assistant power entity reports consumption below a configurable threshold for a configurable amount of time.

Future/comment-only detection type:

```text
energy delta / no energy increase
```

This would support smart plugs that only report cumulative energy.

### `meter_value`

Meter values come from configured Home Assistant entities.

Power meter values should default to average aggregation.

Cumulative energy meter values should default to last-value aggregation.

Meter value capture should have configurable debounce or aggregation duration. For example, if Home Assistant sends power values every second, the bridge may aggregate for one minute and save only the average.

Meter values outside a session should be configurable:

1. Save outside session.
2. Drop outside session.

### `session_started`

For v1, `session_started` uses the same trigger as `charging_started`.

Later these may become separate concepts.

### `session_updated`

Derived from accepted meter values during a session.

### `session_ended`

For v1, `session_ended` uses the same trigger as `charging_stopped`.

An active session should also end if the charger becomes unavailable for the configured duration.

## Detection Defaults

Power-based detection is the default because it is a more direct signal for active charging.

Start detection should eventually be configurable for either:

1. Power threshold.
2. Energy delta.

Stop detection should eventually be configurable for either:

1. Power threshold.
2. Energy delta / no energy increase.

For v1, power threshold detection is enough. Energy-based detection should be kept as a documented future concept.

## Meter Aggregation Defaults

Defaults:

1. `power_w`: `avg`
2. `energy_kwh`: `last`

Rationale:

1. Average makes sense for power over an interval.
2. Last-value makes sense for cumulative energy because averaging cumulative kWh is not useful for session totals.

## Example Configuration Shape

This is illustrative only and not a final schema.

```yaml
chargers:
  - charger_id: garage_charger
    evse_id: garage_evse
    connector_id: garage_connector
    meter_id: garage_meter

    availability:
      entity_id: sensor.charger_power
      unavailable_states: ["unknown", "unavailable"]
      duration: 2m

    start_detection:
      type: power_threshold
      entity_id: sensor.charger_power
      above_w: 200
      duration: 10s
      # Future option:
      # type: energy_delta
      # entity_id: sensor.charger_energy
      # min_delta_kwh: 0.01
      # window: 2m

    stop_detection:
      type: power_threshold
      entity_id: sensor.charger_power
      below_w: 50
      duration: 5m
      # Future option:
      # type: energy_delta
      # entity_id: sensor.charger_energy
      # max_delta_kwh: 0.001
      # window: 10m

    meter_values:
      - metric: power_w
        entity_id: sensor.charger_power
        interval: 1m
        aggregation: avg
        save_outside_session: false

      - metric: energy_kwh
        entity_id: sensor.charger_energy
        interval: 1m
        aggregation: last
        save_outside_session: false

    reserved_events:
      charger_faulted: []
      connector_plugged: []
      connector_unplugged: []
      charging_suspended: []
      charging_resumed: []
```

## Open Questions Remaining

1. What exact fields should be stored for each session?
2. What exact fields should be stored for each meter value?
3. What exact fields should be stored for each emitted event?
4. Should session IDs be generated from timestamps, UUIDs, or deterministic charger/time values?
5. What retention settings should be configurable?
6. Should raw Home Assistant events be stored in SQLite, or only normalized events?
7. Should the bridge expose an HTTP API in v1, or only persist to SQLite?
8. Should SQLite schema migrations be introduced immediately or deferred?
