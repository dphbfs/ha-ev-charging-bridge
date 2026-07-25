package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	bridgeconfig "ha-ev-charging-bridge/internal/config"
	"ha-ev-charging-bridge/internal/detector"
	"ha-ev-charging-bridge/internal/events"
	"ha-ev-charging-bridge/internal/homeassistant"
	"ha-ev-charging-bridge/internal/meter"
	"ha-ev-charging-bridge/internal/persistence"
	"ha-ev-charging-bridge/internal/router"
	"ha-ev-charging-bridge/internal/session"
)

type Pipeline struct {
	charger      bridgeconfig.Charger
	router       *router.Router
	channels     map[string]<-chan homeassistant.EventMessage
	detector     *detector.Detector
	sessions     *session.Processor
	store        *persistence.Store
	aggregators  map[string]*meter.Aggregator
	latestPower  *float64
	latestEnergy *float64
}

func NewPipeline(ctx context.Context, charger bridgeconfig.Charger, store *persistence.Store) (*Pipeline, error) {
	det, err := detector.New(charger)
	if err != nil {
		return nil, fmt.Errorf("initialize charger detector: %w", err)
	}
	r, err := router.New(entityIDs(charger), 8)
	if err != nil {
		return nil, fmt.Errorf("initialize event router: %w", err)
	}
	aggregators, err := meterAggregators(charger)
	if err != nil {
		return nil, err
	}
	if err := store.SaveCharger(ctx, charger); err != nil {
		return nil, err
	}

	p := &Pipeline{
		charger:     charger,
		router:      r,
		channels:    r.Channels(),
		detector:    det,
		sessions:    session.NewProcessor(),
		store:       store,
		aggregators: aggregators,
	}
	if active, ok, err := store.ActiveSession(ctx, charger.ChargerID); err != nil {
		return nil, err
	} else if ok {
		p.sessions.RestoreActive(active)
	}
	return p, nil
}

func (p *Pipeline) InitializeStates(ctx context.Context, states []homeassistant.State) error {
	var powerState *homeassistant.State
	for _, state := range states {
		value, err := parseStateFloat(state.State)
		if err != nil {
			continue
		}
		switch state.EntityID {
		case p.charger.Entities.PowerW, p.charger.Start.EntityID, p.charger.Stop.EntityID:
			p.latestPower = &value
			copy := state
			powerState = &copy
		case p.charger.Entities.EnergyKWh:
			p.latestEnergy = &value
		}
	}
	if powerState != nil {
		msg := homeassistant.EventMessage{}
		msg.Event.TimeFired = powerState.LastChanged
		msg.Event.Data.EntityID = powerState.EntityID
		msg.Event.Data.NewState = powerState
		_, err := p.processEvent(ctx, msg)
		return err
	}
	return nil
}

func (p *Pipeline) ProcessPayload(ctx context.Context, payload []byte) (string, error) {
	var msg homeassistant.EventMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return "", nil
	}
	if msg.Event.Data.EntityID == "" && msg.Event.Data.NewState == nil {
		return "", nil
	}

	routed, err := p.router.Route(ctx, msg)
	if err != nil {
		return "", err
	}
	if !routed {
		return "", nil
	}
	return p.drainRouted(ctx)
}

func (p *Pipeline) Deadline() (time.Time, bool) {
	_, active := p.sessions.Active(p.charger.ChargerID)
	return p.detector.Deadline(detector.State{ActiveSession: active})
}

func (p *Pipeline) Advance(ctx context.Context, at time.Time) (string, error) {
	_, active := p.sessions.Active(p.charger.ChargerID)
	chargerEvents := p.detector.Advance(at, detector.State{ActiveSession: active})
	return p.processChargerEvents(ctx, chargerEvents)
}

func (p *Pipeline) drainRouted(ctx context.Context) (string, error) {
	var completedSessionID string
	for {
		processed := false
		for entityID, ch := range p.channels {
			select {
			case msg := <-ch:
				processed = true
				completed, err := p.processEvent(ctx, msg)
				if err != nil {
					return "", err
				}
				if completed != "" {
					completedSessionID = completed
				}
			default:
				_ = entityID
			}
		}
		if !processed {
			return completedSessionID, nil
		}
	}
}

func (p *Pipeline) processEvent(ctx context.Context, msg homeassistant.EventMessage) (string, error) {
	activeBefore, active := p.sessions.Active(p.charger.ChargerID)
	chargerEvents, err := p.detector.Detect(msg, detector.State{ActiveSession: active})
	if err != nil {
		return "", err
	}
	return p.processChargerEvents(ctx, chargerEvents, activeBefore)
}

func (p *Pipeline) processChargerEvents(ctx context.Context, chargerEvents []events.ChargerEvent, activeBeforeValues ...*events.Session) (string, error) {
	var activeBefore *events.Session
	if len(activeBeforeValues) > 0 {
		activeBefore = activeBeforeValues[0]
	} else if active, ok := p.sessions.Active(p.charger.ChargerID); ok {
		activeBefore = active
	}

	var completedSessionID string
	for _, chargerEvent := range chargerEvents {
		p.applyLatest(chargerEvent)
		if chargerEvent.Type == events.ChargingStarted && chargerEvent.EnergyKWh == nil && p.latestEnergy != nil {
			chargerEvent.EnergyKWh = cloneFloat(p.latestEnergy)
		}
		if (chargerEvent.Type == events.ChargingStopped || chargerEvent.Type == events.ChargerUnavailable) && chargerEvent.EnergyKWh == nil && p.latestEnergy != nil {
			chargerEvent.EnergyKWh = cloneFloat(p.latestEnergy)
		}

		if chargerEvent.Type == events.MeterValueObserved {
			activeSessionID := ""
			if activeBefore != nil {
				activeSessionID = activeBefore.ID
			}
			if err := p.persistMeterValue(ctx, chargerEvent, activeSessionID); err != nil {
				return "", err
			}
		}

		sessionEvents, err := p.sessions.Apply(chargerEvent)
		if err != nil {
			return "", err
		}
		for _, sessionEvent := range sessionEvents {
			if err := p.persistSessionEvent(ctx, sessionEvent); err != nil {
				return "", err
			}
			if sessionEvent.Type == events.SessionEnded {
				completedSessionID = sessionEvent.SessionID
			}
		}
		if current, ok := p.sessions.Active(p.charger.ChargerID); ok {
			if err := p.store.SaveActiveSession(ctx, *current); err != nil {
				return "", err
			}
		}
	}

	return completedSessionID, nil
}

func (p *Pipeline) persistMeterValue(ctx context.Context, event events.ChargerEvent, activeSessionID string) error {
	agg, ok := p.aggregators[event.EntityID]
	if !ok {
		return nil
	}
	value, ok := meterValueFromChargerEvent(event)
	if !ok {
		return nil
	}
	agg.Observe(value, activeSessionID)
	aggregated, ok, err := agg.Flush(event.OccurredAt)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return p.store.SaveMeterValue(ctx, aggregated)
}

func (p *Pipeline) persistSessionEvent(ctx context.Context, event events.SessionEvent) error {
	if event.Type == events.SessionEnded {
		completed, ok := p.sessions.Completed(event.SessionID)
		if !ok {
			return fmt.Errorf("completed session %q was not available after session_ended", event.SessionID)
		}
		if err := p.store.SaveCompletedSession(ctx, completed); err != nil {
			return err
		}
	}
	return p.store.SaveSessionEvent(ctx, event)
}

func (p *Pipeline) applyLatest(event events.ChargerEvent) {
	if event.PowerW != nil {
		p.latestPower = cloneFloat(event.PowerW)
	}
	if event.EnergyKWh != nil {
		p.latestEnergy = cloneFloat(event.EnergyKWh)
	}
}

func entityIDs(charger bridgeconfig.Charger) []string {
	values := []string{
		charger.Start.EntityID,
		charger.Stop.EntityID,
		charger.Entities.PowerW,
		charger.Entities.EnergyKWh,
		charger.Availability.EntityID,
		charger.Entities.Availability,
		charger.Entities.Fault,
		charger.Entities.Plug,
	}
	seen := map[string]struct{}{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func meterAggregators(charger bridgeconfig.Charger) (map[string]*meter.Aggregator, error) {
	configs := charger.Meters
	if len(configs) == 0 {
		configs = []bridgeconfig.Meter{
			{
				MeterID:               charger.MeterID,
				EntityID:              charger.Entities.PowerW,
				Unit:                  "W",
				Aggregation:           "average",
				OutsideSessionStorage: "drop",
			},
			{
				MeterID:               charger.MeterID,
				EntityID:              charger.Entities.EnergyKWh,
				Unit:                  "kWh",
				Aggregation:           "last",
				OutsideSessionStorage: "save",
			},
		}
	}

	aggregators := make(map[string]*meter.Aggregator, len(configs))
	for _, cfg := range configs {
		agg, err := meter.NewAggregator(cfg)
		if err != nil {
			return nil, fmt.Errorf("initialize meter aggregator for %q: %w", cfg.EntityID, err)
		}
		aggregators[cfg.EntityID] = agg
	}
	return aggregators, nil
}

func meterValueFromChargerEvent(event events.ChargerEvent) (events.MeterValue, bool) {
	value := events.MeterValue{
		ChargerID:  event.ChargerID,
		MeterID:    event.MeterID,
		EntityID:   event.EntityID,
		ObservedAt: event.OccurredAt,
	}
	if event.PowerW != nil {
		value.Unit = "W"
		value.Value = *event.PowerW
		return value, true
	}
	if event.EnergyKWh != nil {
		value.Unit = "kWh"
		value.Value = *event.EnergyKWh
		return value, true
	}
	return events.MeterValue{}, false
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func parseStateFloat(raw string) (float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" || value == "unknown" || value == "unavailable" {
		return 0, fmt.Errorf("state is not numeric: %q", raw)
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse state %q: %w", raw, err)
	}
	return parsed, nil
}

func EventMessage(entityID, state string, at time.Time) homeassistant.EventMessage {
	msg := homeassistant.EventMessage{}
	msg.Event.TimeFired = at
	msg.Event.Data.EntityID = entityID
	msg.Event.Data.NewState = &homeassistant.State{EntityID: entityID, State: state, LastChanged: at}
	return msg
}
