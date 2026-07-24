# Implementation Requirements V1

This document captures implementation requirements for v1 of the Home Assistant EV charging bridge.

## Language

The application must be written in Go.

## Runtime Entry Point

The main runtime entry point is a Home Assistant websocket listener.

The `main` package should stay small. It should load configuration, initialize logging, wire dependencies, handle shutdown, and call the application runner. Business logic should live outside `main`, preferably under `internal/` packages.

The websocket listener should:

1. Connect to the configured Home Assistant websocket endpoint.
2. Authenticate using configured credentials.
3. Subscribe to relevant Home Assistant events.
4. Receive Home Assistant event payloads.
5. Convert raw Home Assistant events into typed Go objects.
6. Route typed events into internal channels for downstream processing.

## Event Routing

Home Assistant events should be converted into internal event objects before business logic processes them.

Each configured Home Assistant entity should have a separate event channel.

The application should have another listener that consumes entity channels using `select`, allowing concurrent switching between entity events.

The event flow should be:

```text
Home Assistant websocket
  -> raw websocket message
  -> typed Home Assistant event object
  -> entity-specific channel
  -> charger/session event detection
  -> session and meter processing
```

## Packages

Each logical part of the application should live in a separate Go package only when that package boundary is useful and testable. The repository should remain a single Go module for v1 unless a concrete multi-module need appears.

Candidate logical areas:

1. Home Assistant websocket client.
2. Home Assistant event types.
3. Configuration loading.
4. Charger configuration model.
5. Event routing/channel dispatch.
6. Charger event detection.
7. Session lifecycle processing.
8. Meter value aggregation.
9. Persistence.
10. Application orchestration.

The package boundaries should stay practical and small. Avoid over-abstracting before the behavior is proven.

## Public Types And Interfaces

Types should remain package-private by default. Export types only when they are shared across package boundaries or represent stable domain concepts.

Examples of concepts that may justify exported types:

1. Charger configuration types.
2. Home Assistant event types.
3. Charger event types.
4. Session event types.
5. Session model.
6. Meter value model.
7. Persistence contracts.
8. Event publisher/router contracts.

Package-private types may still be used for implementation details inside a module.

Interfaces should be defined at the consuming package boundary when they improve testability or decouple business logic from external systems. Avoid defining interfaces only because a concrete type exists.

## Libraries

The application should use established Go libraries where useful instead of custom implementations.

Likely library areas:

1. Websocket client.
2. YAML parsing.
3. SQLite access.
4. Environment variable loading.
5. Testing utilities.
6. Test containers or integration test infrastructure.

Library choices should favor stable, commonly used packages with clear maintenance status. Small focused standard-library implementations are preferred over large dependencies when the behavior is simple.

## Configuration

Application configuration should live in YAML files.

Connection configuration should also be YAML-based, with values injectable from environment variables.

Environment variables may come from:

1. A local `.env` file.
2. The process environment.

Secrets such as Home Assistant tokens must not be hard-coded in YAML files committed to the repository.

Configuration loading should support environment interpolation or equivalent explicit environment lookup for sensitive values.

Example concept:

```yaml
home_assistant:
  url: ${HA_URL}
  token: ${HA_TOKEN}
```

Device and charger behavior configuration should also live in YAML.

Configuration loading should produce validated typed structs. Invalid required fields, invalid durations, unsupported detection types, and duplicate charger or entity identifiers should fail fast at startup.

Example areas for YAML configuration:

1. Charger IDs.
2. EVSE IDs.
3. Connector IDs.
4. Home Assistant entity mappings.
5. Availability detection.
6. Start detection.
7. Stop detection.
8. Meter value aggregation.
9. Retention settings.

## Testing

The application should contain unit tests and integration tests.

All Go code should be formatted with `gofmt`. V1 should include `go test ./...` as the baseline verification command, with `go vet` and `golangci-lint` added as project quality gates.

### Unit Tests

Unit tests should cover business logic that does not require a real Home Assistant connection. Unit tests should focus on observable behavior and pure decision logic rather than implementation details.

Duration-based detection logic should be testable without real sleeps. Time-dependent code should use injectable clocks/timers or isolated functions that can be unit tested deterministically.

Priority unit test areas:

1. Config parsing and validation.
2. Home Assistant event parsing.
3. Entity-to-event mapping.
4. Charging start detection.
5. Charging stop detection.
6. Session lifecycle transitions.
7. Meter value aggregation.
8. Retention logic.

### Integration Tests

Integration tests should cover websocket behavior and realistic event flow.

Integration tests should use in-process fakes before testcontainers unless a real service boundary is being exercised. Tests that require external services should use build tags.

Integration test areas:

1. Websocket connection handling.
2. Authentication flow simulation.
3. Event subscription flow.
4. Delivery of websocket events into typed objects.
5. Routing of entity events into channels.
6. End-to-end session start and stop from simulated Home Assistant events.

If testcontainers is too heavy for the first pass, a local in-process websocket test server may be acceptable for early v1 tests, with testcontainers added when there is a real external service boundary to exercise.

## Concurrency

The application should use Go channels to route typed entity events.

The channel-based design should support concurrent event sources while keeping session lifecycle decisions deterministic.

Channel ownership should be explicit. Internal event channels should be bounded or have clearly documented blocking behavior so websocket ingestion cannot create unbounded memory growth.

The session processor should be the owner of session state for a charger to avoid scattered mutable state. Session state for a charger should be owned by a single processor goroutine or otherwise protected from concurrent mutation.

Where multiple goroutines are used, shutdown and error propagation should be explicit. Long-running goroutines should accept `context.Context`, stop on cancellation, and report terminal errors through a clear owner.

## Error Handling

Errors should be returned with enough context to identify the failing subsystem.

Errors should be wrapped where appropriate so callers can preserve the underlying cause.

Errors should be logged at the boundary where they are handled. Avoid logging and returning the same error.

Websocket failures should be handled explicitly.

Configuration errors should fail fast at startup.

Invalid Home Assistant events should not crash the process if they are unrelated or safely ignorable.

## Persistence

SQLite is the preferred persistence layer for v1.

Persistence code should be isolated behind interfaces so session logic is not coupled directly to SQL statements.

The bridge should persist enough information to expose:

1. Active sessions.
2. Completed sessions.
3. Meter values.
4. Session lifecycle events.

Retention should be configurable.

## Non-Goals For V1

V1 does not need to implement full OCPI.

V1 does not need to implement full OCPP.

V1 does not need a Home Assistant embedded UI yet.

V1 does not need multi-connector charger support.

V1 does not need full `charging_suspended` behavior.
