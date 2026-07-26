package session

import (
	"errors"
	"fmt"
	"time"

	"ha-ev-charging-bridge/internal/events"
)

var ErrActiveSessionExists = errors.New("active session already exists")

type Processor struct {
	active    map[string]*events.Session
	completed map[string]events.Session
}

func NewProcessor() *Processor {
	return &Processor{
		active:    map[string]*events.Session{},
		completed: map[string]events.Session{},
	}
}

func (p *Processor) Apply(event events.ChargerEvent) ([]events.SessionEvent, error) {
	switch event.Type {
	case events.ChargingStarted:
		return p.start(event)
	case events.ChargingStopped, events.ChargerUnavailable:
		return p.end(event)
	case events.MeterValueObserved:
		if p.updateMeter(event) {
			return []events.SessionEvent{p.sessionEvent(events.SessionUpdated, event, "")}, nil
		}
		return nil, nil
	default:
		return nil, nil
	}
}

func (p *Processor) Active(chargerID string) (*events.Session, bool) {
	active, ok := p.active[chargerID]
	if !ok {
		return nil, false
	}
	copy := *active
	return &copy, true
}

func (p *Processor) RestoreActive(session events.Session) {
	copy := session
	p.active[session.ChargerID] = &copy
}

func (p *Processor) Completed(sessionID string) (events.Session, bool) {
	completed, ok := p.completed[sessionID]
	if !ok {
		return events.Session{}, false
	}
	return completed, true
}

func (p *Processor) start(event events.ChargerEvent) ([]events.SessionEvent, error) {
	if _, exists := p.active[event.ChargerID]; exists {
		return nil, ErrActiveSessionExists
	}

	s := &events.Session{
		ID:             sessionID(event),
		ChargerID:      event.ChargerID,
		EVSEID:         event.EVSEID,
		ConnectorID:    event.ConnectorID,
		MeterID:        event.MeterID,
		State:          events.SessionCharging,
		StartedAt:      event.OccurredAt,
		StartEnergyKWh: cloneFloat(event.EnergyKWh),
		EndEnergyKWh:   cloneFloat(event.EnergyKWh),
	}
	p.active[event.ChargerID] = s
	return []events.SessionEvent{p.sessionEvent(events.SessionStarted, event, "")}, nil
}

func (p *Processor) end(event events.ChargerEvent) ([]events.SessionEvent, error) {
	active, ok := p.active[event.ChargerID]
	if !ok {
		return nil, nil
	}

	endedAt := event.OccurredAt
	active.State = events.SessionEndedState
	active.EndedAt = &endedAt
	if event.EnergyKWh != nil {
		active.EndEnergyKWh = cloneFloat(event.EnergyKWh)
	}
	if active.StartEnergyKWh != nil && active.EndEnergyKWh != nil {
		consumed := *active.EndEnergyKWh - *active.StartEnergyKWh
		active.EnergyConsumedKWh = &consumed
	}

	reason := event.Reason
	if reason == "" {
		reason = string(event.Type)
	}
	sessionEvent := events.SessionEvent{
		Type:       events.SessionEnded,
		SessionID:  active.ID,
		ChargerID:  event.ChargerID,
		OccurredAt: event.OccurredAt,
		Reason:     reason,
	}
	p.completed[active.ID] = *active
	delete(p.active, event.ChargerID)
	return []events.SessionEvent{sessionEvent}, nil
}

func (p *Processor) updateMeter(event events.ChargerEvent) bool {
	active, ok := p.active[event.ChargerID]
	if !ok || event.EnergyKWh == nil {
		return false
	}
	active.EndEnergyKWh = cloneFloat(event.EnergyKWh)
	if active.StartEnergyKWh != nil {
		consumed := *active.EndEnergyKWh - *active.StartEnergyKWh
		active.EnergyConsumedKWh = &consumed
	}
	return true
}

func (p *Processor) sessionEvent(eventType events.SessionEventType, event events.ChargerEvent, reason string) events.SessionEvent {
	sessionID := ""
	if active, ok := p.active[event.ChargerID]; ok {
		sessionID = active.ID
	}
	return events.SessionEvent{
		Type:       eventType,
		SessionID:  sessionID,
		ChargerID:  event.ChargerID,
		OccurredAt: event.OccurredAt,
		Reason:     reason,
	}
}

func sessionID(event events.ChargerEvent) string {
	at := event.OccurredAt
	if at.IsZero() {
		at = time.Now()
	}
	return fmt.Sprintf("session-%s-%s", event.ChargerID, at.UTC().Format("20060102T150405Z"))
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
