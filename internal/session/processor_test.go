package session

import (
	"errors"
	"testing"
	"time"

	"ha-ev-charging-bridge/internal/events"
)

func TestApplyStartsSessionFromChargingStarted(t *testing.T) {
	p := NewProcessor()
	startedAt := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	got, err := p.Apply(chargerEvent(events.ChargingStarted, startedAt, floatPtr(10)))
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(got) != 1 || got[0].Type != events.SessionStarted {
		t.Fatalf("Apply() events = %#v, want session_started", got)
	}
	active, ok := p.Active("charger-1")
	if !ok {
		t.Fatal("Active() ok = false, want active session")
	}
	if active.State != events.SessionCharging {
		t.Fatalf("active.State = %q, want charging", active.State)
	}
}

func TestApplyEndsSessionFromChargingStopped(t *testing.T) {
	p := startedProcessor(t, 10)

	got, err := p.Apply(chargerEvent(events.ChargingStopped, time.Date(2026, 7, 24, 13, 0, 0, 0, time.UTC), floatPtr(12.5)))
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(got) != 1 || got[0].Type != events.SessionEnded {
		t.Fatalf("Apply() events = %#v, want session_ended", got)
	}
	if _, ok := p.Active("charger-1"); ok {
		t.Fatal("Active() ok = true, want no active session")
	}
}

func TestApplyEndsSessionFromChargerUnavailable(t *testing.T) {
	p := startedProcessor(t, 10)

	got, err := p.Apply(chargerEvent(events.ChargerUnavailable, time.Date(2026, 7, 24, 13, 0, 0, 0, time.UTC), nil))
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(got) != 1 || got[0].Type != events.SessionEnded {
		t.Fatalf("Apply() events = %#v, want session_ended", got)
	}
	if got[0].Reason != string(events.ChargerUnavailable) {
		t.Fatalf("Reason = %q, want charger_unavailable", got[0].Reason)
	}
}

func TestApplyRejectsDuplicateActiveSession(t *testing.T) {
	p := startedProcessor(t, 10)

	_, err := p.Apply(chargerEvent(events.ChargingStarted, time.Date(2026, 7, 24, 12, 1, 0, 0, time.UTC), floatPtr(10.1)))
	if !errors.Is(err, ErrActiveSessionExists) {
		t.Fatalf("Apply() error = %v, want ErrActiveSessionExists", err)
	}
}

func TestApplyCalculatesConsumedEnergy(t *testing.T) {
	p := startedProcessor(t, 10)

	if _, err := p.Apply(chargerEvent(events.MeterValueObserved, time.Date(2026, 7, 24, 12, 30, 0, 0, time.UTC), floatPtr(11.25))); err != nil {
		t.Fatalf("Apply(meter) error = %v", err)
	}
	active, ok := p.Active("charger-1")
	if !ok {
		t.Fatal("Active() ok = false, want active session")
	}
	if active.EnergyConsumedKWh == nil || *active.EnergyConsumedKWh != 1.25 {
		t.Fatalf("EnergyConsumedKWh = %v, want 1.25", active.EnergyConsumedKWh)
	}
}

func startedProcessor(t *testing.T, startEnergyKWh float64) *Processor {
	t.Helper()
	p := NewProcessor()
	_, err := p.Apply(chargerEvent(events.ChargingStarted, time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC), &startEnergyKWh))
	if err != nil {
		t.Fatalf("Apply(start) error = %v", err)
	}
	return p
}

func chargerEvent(eventType events.ChargerEventType, at time.Time, energyKWh *float64) events.ChargerEvent {
	return events.ChargerEvent{
		Type:        eventType,
		ChargerID:   "charger-1",
		EVSEID:      "evse-1",
		ConnectorID: "connector-1",
		OccurredAt:  at,
		EnergyKWh:   energyKWh,
	}
}

func floatPtr(value float64) *float64 {
	return &value
}
