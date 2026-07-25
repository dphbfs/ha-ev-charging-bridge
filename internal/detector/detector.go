package detector

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	bridgeconfig "ha-ev-charging-bridge/internal/config"
	"ha-ev-charging-bridge/internal/events"
	"ha-ev-charging-bridge/internal/homeassistant"
)

const powerThresholdDetection = "power_threshold"

type Detector struct {
	charger bridgeconfig.Charger

	startDuration       time.Duration
	stopDuration        time.Duration
	unavailableDuration time.Duration
	aboveStartSince     *time.Time
	belowStopSince      *time.Time
	unavailableSince    *time.Time
	lastPowerEntityID   string
	lastAvailabilityID  string
}

type State struct {
	ActiveSession bool
}

func New(charger bridgeconfig.Charger) (*Detector, error) {
	if charger.Start.Type != powerThresholdDetection {
		return nil, fmt.Errorf("unsupported start detection type %q", charger.Start.Type)
	}
	if charger.Stop.Type != powerThresholdDetection {
		return nil, fmt.Errorf("unsupported stop detection type %q", charger.Stop.Type)
	}

	startDuration, err := parseDuration(charger.Start.Duration)
	if err != nil {
		return nil, fmt.Errorf("parse start duration: %w", err)
	}
	stopDuration, err := parseDuration(charger.Stop.Duration)
	if err != nil {
		return nil, fmt.Errorf("parse stop duration: %w", err)
	}
	unavailableDuration, err := parseDuration(charger.Availability.UnavailableAfter)
	if err != nil {
		return nil, fmt.Errorf("parse unavailable duration: %w", err)
	}

	return &Detector{
		charger:             charger,
		startDuration:       startDuration,
		stopDuration:        stopDuration,
		unavailableDuration: unavailableDuration,
	}, nil
}

func (d *Detector) Detect(msg homeassistant.EventMessage, state State) ([]events.ChargerEvent, error) {
	entityID := strings.TrimSpace(msg.Event.Data.EntityID)
	if msg.Event.Data.NewState == nil {
		return nil, nil
	}
	at := eventTime(msg.Event.TimeFired)

	switch entityID {
	case d.charger.Start.EntityID, d.charger.Stop.EntityID:
		powerW, err := parseStateFloat(msg.Event.Data.NewState.State)
		if err != nil {
			return nil, nil
		}
		d.lastPowerEntityID = entityID
		return d.detectPower(entityID, powerW, at, state), nil
	case d.charger.Entities.EnergyKWh:
		energyKWh, err := parseStateFloat(msg.Event.Data.NewState.State)
		if err != nil {
			return nil, nil
		}
		return []events.ChargerEvent{d.event(events.MeterValueObserved, entityID, at, nil, &energyKWh, "")}, nil
	case d.charger.Availability.EntityID, d.charger.Entities.Availability:
		d.lastAvailabilityID = entityID
		return d.detectAvailability(entityID, msg.Event.Data.NewState.State, at), nil
	}

	return nil, nil
}

func (d *Detector) Deadline(state State) (time.Time, bool) {
	var deadline time.Time
	if !state.ActiveSession && d.aboveStartSince != nil {
		deadline = d.aboveStartSince.Add(d.startDuration)
	}
	if state.ActiveSession && d.belowStopSince != nil {
		deadline = earliest(deadline, d.belowStopSince.Add(d.stopDuration))
	}
	if d.unavailableSince != nil {
		deadline = earliest(deadline, d.unavailableSince.Add(d.unavailableDuration))
	}
	return deadline, !deadline.IsZero()
}

func (d *Detector) Advance(at time.Time, state State) []events.ChargerEvent {
	var result []events.ChargerEvent
	if !state.ActiveSession && d.aboveStartSince != nil && !at.Before(d.aboveStartSince.Add(d.startDuration)) {
		result = append(result, d.event(events.ChargingStarted, d.lastPowerEntityID, at, nil, nil, ""))
		d.aboveStartSince = nil
	}
	if state.ActiveSession && d.belowStopSince != nil && !at.Before(d.belowStopSince.Add(d.stopDuration)) {
		result = append(result, d.event(events.ChargingStopped, d.lastPowerEntityID, at, nil, nil, ""))
		d.belowStopSince = nil
	}
	if d.unavailableSince != nil && !at.Before(d.unavailableSince.Add(d.unavailableDuration)) {
		result = append(result, d.event(events.ChargerUnavailable, d.lastAvailabilityID, at, nil, nil, "charger_unavailable"))
		d.unavailableSince = nil
	}
	return result
}

func (d *Detector) detectPower(entityID string, powerW float64, at time.Time, state State) []events.ChargerEvent {
	result := []events.ChargerEvent{d.event(events.MeterValueObserved, entityID, at, &powerW, nil, "")}

	if !state.ActiveSession && powerW > d.charger.Start.ThresholdW {
		if d.aboveStartSince == nil {
			d.aboveStartSince = &at
		}
		if at.Sub(*d.aboveStartSince) >= d.startDuration {
			result = append(result, d.event(events.ChargingStarted, entityID, at, &powerW, nil, ""))
			d.aboveStartSince = nil
		}
	} else {
		d.aboveStartSince = nil
	}

	if state.ActiveSession && powerW < d.charger.Stop.ThresholdW {
		if d.belowStopSince == nil {
			d.belowStopSince = &at
		}
		if at.Sub(*d.belowStopSince) >= d.stopDuration {
			result = append(result, d.event(events.ChargingStopped, entityID, at, &powerW, nil, ""))
			d.belowStopSince = nil
		}
	} else {
		d.belowStopSince = nil
	}

	return result
}

func (d *Detector) detectAvailability(entityID, rawState string, at time.Time) []events.ChargerEvent {
	state := strings.TrimSpace(rawState)
	if state == d.charger.Availability.AvailableState {
		d.unavailableSince = nil
		return []events.ChargerEvent{d.event(events.ChargerAvailable, entityID, at, nil, nil, "")}
	}
	if state != d.charger.Availability.UnavailableState {
		return nil
	}

	if d.unavailableSince == nil {
		d.unavailableSince = &at
	}
	if at.Sub(*d.unavailableSince) < d.unavailableDuration {
		return nil
	}

	d.unavailableSince = nil
	return []events.ChargerEvent{d.event(events.ChargerUnavailable, entityID, at, nil, nil, "charger_unavailable")}
}

func (d *Detector) event(eventType events.ChargerEventType, entityID string, at time.Time, powerW, energyKWh *float64, reason string) events.ChargerEvent {
	return events.ChargerEvent{
		Type:        eventType,
		ChargerID:   d.charger.ChargerID,
		EVSEID:      d.charger.EVSEID,
		ConnectorID: d.charger.ConnectorID,
		MeterID:     d.charger.MeterID,
		EntityID:    entityID,
		OccurredAt:  at,
		PowerW:      powerW,
		EnergyKWh:   energyKWh,
		Reason:      reason,
	}
}

func parseStateFloat(raw string) (float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" || value == "unknown" || value == "unavailable" {
		return 0, fmt.Errorf("state is not numeric: %q", raw)
	}
	return strconv.ParseFloat(value, 64)
}

func parseDuration(raw string) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	return time.ParseDuration(raw)
}

func eventTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now()
	}
	return value
}

func earliest(current, candidate time.Time) time.Time {
	if current.IsZero() || candidate.Before(current) {
		return candidate
	}
	return current
}
