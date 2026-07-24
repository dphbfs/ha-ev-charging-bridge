# Roadmap

## Phase 1: V1 Home Assistant Ingress Bridge

Phase 1 maps to `functional-requirements-v1.md` and `implementation-requirements-v1.md`.

Goal: build a Go-based Home Assistant ingress bridge that maps configured Home Assistant entities into charger/session events, tracks simple charging sessions, records meter values, and persists state in SQLite.

V1 should remain protocol-neutral. It should use OCPP-inspired lifecycle concepts and OCPI-inspired naming where useful, but it should not implement either protocol.

### Task 1: Normalize V1 Configuration

Define the v1 YAML configuration shape for:

1. Home Assistant connection settings with environment variable injection.
2. Charger identity: `charger_id`, `evse_id`, `connector_id`, `meter_id`.
3. Home Assistant entity mappings.
4. Availability detection.
5. Power-threshold start detection.
6. Power-threshold stop detection.
7. Meter value aggregation.
8. Retention settings.

Verification:

1. Unit tests load a valid YAML config.
2. Unit tests reject missing required charger fields.
3. Unit tests resolve values from `.env` or process environment.
4. Example config contains no real secrets or personal hostnames.

### Task 2: Split Code Into Practical Go Packages

Refactor the current POC into small packages around logical responsibilities:

1. Configuration loading.
2. Home Assistant websocket client and event types.
3. Event routing.
4. Charger event detection.
5. Session lifecycle processing.
6. Meter aggregation.
7. SQLite persistence.
8. Application orchestration.

Cross-package domain types and interfaces should be public where they represent reusable concepts.

Verification:

1. `go test ./...` passes.
2. Public types exist for charger config, HA events, charger events, session events, sessions, and meter values.
3. Package-private implementation details remain private.

### Task 3: Implement Home Assistant Websocket Listener

Implement the websocket listener as the runtime ingress point.

The listener should:

1. Connect to the configured Home Assistant websocket URL.
2. Authenticate with a configured token.
3. Subscribe to configured event types.
4. Parse raw messages into typed Home Assistant event objects.
5. Forward typed events into internal routing.

Verification:

1. Unit tests cover websocket URL conversion.
2. Unit tests cover auth response parsing.
3. Integration test simulates Home Assistant websocket auth, subscription, and event delivery.

### Task 4: Route Events To Entity Channels

Implement entity-specific channels for configured Home Assistant entities.

The router should:

1. Match typed Home Assistant events by `entity_id`.
2. Send each matching event to that entity's channel.
3. Ignore unrelated entities safely.
4. Surface routing errors without crashing on ignorable payloads.

The downstream processor should use `select` to consume across entity channels and switch concurrently between event sources.

Verification:

1. Unit tests route matching entities to the expected channel.
2. Unit tests ignore unrelated entities.
3. Unit tests verify multiple entity channels can be consumed without blocking under normal conditions.

### Task 5: Implement Charger Event Detection

Map Home Assistant entity events into v1 charger events.

Active v1 charger events:

1. `charger_available`
2. `charger_unavailable`
3. `charging_started`
4. `charging_stopped`
5. `meter_value`

Reserved events should be represented in config/schema where practical, but not implemented as active behavior:

1. `charger_faulted`
2. `connector_plugged`
3. `connector_unplugged`
4. `charging_suspended`
5. `charging_resumed`

Verification:

1. Unit tests emit `charging_started` when power is above threshold for configured duration.
2. Unit tests emit `charging_stopped` when power is below threshold for configured duration.
3. Unit tests emit `charger_unavailable` after configured unavailable duration.
4. Unit tests do not emit start events when a charger already has an active session.
5. Unit tests document future energy-delta detection as unsupported in v1.

### Task 6: Implement Session Lifecycle Processing

Implement the v1 session lifecycle with two states:

1. `charging`
2. `ended`

For v1:

1. `session_started` uses the same trigger as `charging_started`.
2. `session_ended` uses the same trigger as `charging_stopped`.
3. `charger_unavailable` during an active session ends the session.

Verification:

1. Unit tests start a session from `charging_started`.
2. Unit tests end a session from `charging_stopped`.
3. Unit tests end a session from `charger_unavailable`.
4. Unit tests reject duplicate active sessions for the same charger.
5. Unit tests calculate consumed energy when start and end energy are available.

### Task 7: Implement Meter Value Aggregation

Implement configurable meter value aggregation.

Defaults:

1. `power_w`: average aggregation.
2. `energy_kwh`: last-value aggregation.

Meter values outside an active session should be configurable:

1. Save outside session.
2. Drop outside session.

Verification:

1. Unit tests aggregate power values using average over the configured interval.
2. Unit tests aggregate cumulative energy using last value over the configured interval.
3. Unit tests save or drop values outside a session according to config.
4. Unit tests link meter values to the active session when one exists.

### Task 8: Add SQLite Persistence

Replace JSONL/session-file persistence with SQLite-backed storage for v1 data.

Persist:

1. Chargers.
2. Active sessions.
3. Completed sessions.
4. Meter values.
5. Session lifecycle events.

Persistence code should be isolated behind interfaces so business logic does not depend directly on SQL statements.

Verification:

1. Unit or integration tests create a temporary SQLite database.
2. Tests persist and reload an active session.
3. Tests persist and query a completed session.
4. Tests persist and query meter values for a session.
5. Tests persist and query lifecycle events.

### Task 9: Implement Retention Rules

Implement configurable retention behavior for stored data.

Retention should cover at least:

1. Meter values.
2. Session lifecycle events.
3. Optional raw Home Assistant events if stored.

Verification:

1. Unit tests keep records newer than the retention threshold.
2. Unit tests delete records older than the retention threshold.
3. Unit tests preserve completed session summaries unless configured otherwise.

### Task 10: Add End-To-End Simulated Flow Test

Add an integration test that simulates a Home Assistant websocket and drives a full v1 session.

The test should simulate:

1. Websocket auth.
2. Event subscription.
3. Power above threshold.
4. Meter updates.
5. Power below threshold long enough to stop.
6. Completed session persistence.

Verification:

1. Integration test produces one completed session.
2. Integration test records expected meter values.
3. Integration test records `session_started` and `session_ended` events.
4. Integration test passes with `go test ./...`.

### Task 11: Add GitHub Actions CI

Add GitHub Actions workflow coverage for automated test verification.

The CI workflow should:

1. Verify formatting with `gofmt`.
2. Run Go tests with `go test ./...`.
3. Run `go vet ./...`.
4. Run `golangci-lint run ./...`.
5. Generate test coverage.
6. Be triggerable manually.
7. Run when a pull request is opened against `main`.
8. Publish or expose enough coverage data to support a README coverage badge.

README badges should show:

1. Test pass/fail status.
2. Test coverage status.

Badge updates should reflect the state after changes are merged into `main`.

Verification:

1. Workflow YAML exists under `.github/workflows/`.
2. Workflow supports `workflow_dispatch`.
3. Workflow supports pull requests targeting `main`.
4. CI verifies `gofmt` formatting.
5. CI runs `go test ./...`.
6. CI runs `go vet ./...`.
7. CI runs `golangci-lint run ./...`.
8. CI generates coverage output.
9. README contains test status and coverage badges pointing at the `main` branch state.

### Phase 1 Completion Criteria

Phase 1 is complete when:

1. The app runs as a Go Home Assistant websocket ingress bridge.
2. Configuration is YAML-based with environment-injected connection values.
3. Configured HA entities are routed into entity-specific channels.
4. Power-based charger events are detected from HA state changes.
5. Sessions start and end according to v1 rules.
6. Meter values are aggregated according to config.
7. Active and completed sessions are persisted in SQLite.
8. Lifecycle events are persisted in SQLite.
9. Unit tests cover core business logic.
10. Integration tests cover websocket-driven session flow.
11. GitHub Actions CI runs tests and coverage on manual trigger and PRs into `main`.
12. README includes test pass/fail and coverage badges for `main`.
13. `go test ./...` passes.
