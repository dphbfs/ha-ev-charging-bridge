# Agent Context

## Project

`ha-ev-charging-bridge` is a proof-of-concept Go application that bridges Home Assistant energy events into EV charging session events.

The application should subscribe to Home Assistant websocket events, filter events related to entities defined in local configuration, and translate those events into charger/session events. The target event format will be OCPI or OCPP, but that decision is intentionally deferred.

This repository is only the bridge component. A later external system will receive and store the generated charging session events.

## Current State

- This repository is intentionally new and mostly empty.
- Prefer small, reversible changes while the POC direction is being validated.
- The intended implementation language is Go.
- Do not assume a production architecture, deployment target, storage backend, OCPI/OCPP choice, or Home Assistant deployment details until specified.

## Working Guidelines

- Inspect the repo before making changes; this file may become stale as the POC evolves.
- Keep the first implementation simple and observable before adding abstractions.
- Favor local configuration examples over hard-coded personal infrastructure details.
- Avoid introducing secrets, tokens, Home Assistant long-lived access tokens, charger credentials, or real hostnames into committed files.
- Model the bridge around configurable Home Assistant entity IDs rather than hard-coded entity names.
- Keep generated charger/session events format-neutral until OCPI versus OCPP is decided.
- If event semantics, session boundaries, or integration behavior are ambiguous, ask one focused question rather than designing around guesses.

## Likely POC Concerns

- Home Assistant websocket subscription and reconnect behavior.
- Config-driven filtering for relevant Home Assistant entities.
- Mapping Home Assistant energy/state events into charger/session lifecycle events.
- Session boundary detection from energy or charging-state changes.
- A dry-run/logging sink before forwarding events to a storage system.
- Safe failure behavior when Home Assistant or the network is unavailable.

## Verification Expectations

- Add a lightweight test or simulation path for decision logic once code exists.
- Prefer tests around event mapping and session boundary logic.
- Prefer commands documented in the repo, such as `README.md`, `Makefile`, or package scripts.
- If no verification command exists yet, state exactly what was manually checked.
