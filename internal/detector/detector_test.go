package detector

import (
	"strings"
	"testing"
	"time"

	bridgeconfig "ha-ev-charging-bridge/internal/config"
	"ha-ev-charging-bridge/internal/events"
	"ha-ev-charging-bridge/internal/homeassistant"
)

func TestDetectEmitsChargingStartedAboveThresholdForDuration(t *testing.T) {
	d := newDetector(t, chargerConfig())
	start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	got, err := d.Detect(event("sensor.ev_charger_power", "250", start), State{})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if hasEvent(got, events.ChargingStarted) {
		t.Fatal("Detect() emitted charging_started before duration elapsed")
	}

	got, err = d.Detect(event("sensor.ev_charger_power", "260", start.Add(5*time.Second)), State{})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !hasEvent(got, events.ChargingStarted) {
		t.Fatalf("Detect() events = %#v, want charging_started", got)
	}
}

func TestDetectEmitsChargingStoppedBelowThresholdForDuration(t *testing.T) {
	d := newDetector(t, chargerConfig())
	start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	got, err := d.Detect(event("sensor.ev_charger_power", "40", start), State{ActiveSession: true})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if hasEvent(got, events.ChargingStopped) {
		t.Fatal("Detect() emitted charging_stopped before duration elapsed")
	}

	got, err = d.Detect(event("sensor.ev_charger_power", "35", start.Add(10*time.Second)), State{ActiveSession: true})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !hasEvent(got, events.ChargingStopped) {
		t.Fatalf("Detect() events = %#v, want charging_stopped", got)
	}
}

func TestDetectEmitsChargerUnavailableAfterDuration(t *testing.T) {
	d := newDetector(t, chargerConfig())
	start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	got, err := d.Detect(event("binary_sensor.ev_charger_online", "off", start), State{})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if hasEvent(got, events.ChargerUnavailable) {
		t.Fatal("Detect() emitted charger_unavailable before duration elapsed")
	}

	got, err = d.Detect(event("binary_sensor.ev_charger_online", "off", start.Add(2*time.Minute)), State{})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !hasEvent(got, events.ChargerUnavailable) {
		t.Fatalf("Detect() events = %#v, want charger_unavailable", got)
	}
}

func TestDetectDoesNotEmitStartWhenSessionIsActive(t *testing.T) {
	d := newDetector(t, chargerConfig())
	start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	if _, err := d.Detect(event("sensor.ev_charger_power", "250", start), State{ActiveSession: true}); err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	got, err := d.Detect(event("sensor.ev_charger_power", "260", start.Add(5*time.Second)), State{ActiveSession: true})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if hasEvent(got, events.ChargingStarted) {
		t.Fatalf("Detect() events = %#v, did not want charging_started", got)
	}
}

func TestNewDocumentsEnergyDeltaDetectionUnsupportedInV1(t *testing.T) {
	cfg := chargerConfig()
	cfg.Start.Type = "energy_delta"
	_, err := New(cfg)
	if err == nil {
		t.Fatal("New() error = nil, want unsupported detection type")
	}
	if !strings.Contains(err.Error(), "unsupported start detection type") {
		t.Fatalf("New() error = %q, want unsupported start detection type", err)
	}
}

func newDetector(t *testing.T, cfg bridgeconfig.Charger) *Detector {
	t.Helper()
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return d
}

func hasEvent(values []events.ChargerEvent, eventType events.ChargerEventType) bool {
	for _, value := range values {
		if value.Type == eventType {
			return true
		}
	}
	return false
}

func event(entityID, state string, at time.Time) homeassistant.EventMessage {
	var msg homeassistant.EventMessage
	msg.Event.TimeFired = at
	msg.Event.Data.EntityID = entityID
	msg.Event.Data.NewState = &homeassistant.State{EntityID: entityID, State: state, LastChanged: at}
	return msg
}

func chargerConfig() bridgeconfig.Charger {
	return bridgeconfig.Charger{
		ChargerID:   "charger-1",
		EVSEID:      "evse-1",
		ConnectorID: "connector-1",
		MeterID:     "meter-1",
		Entities: bridgeconfig.EntityMapping{
			PowerW:       "sensor.ev_charger_power",
			EnergyKWh:    "sensor.ev_charger_energy",
			Availability: "binary_sensor.ev_charger_online",
		},
		Availability: bridgeconfig.Availability{
			EntityID:         "binary_sensor.ev_charger_online",
			AvailableState:   "on",
			UnavailableState: "off",
			UnavailableAfter: "2m",
		},
		Start: bridgeconfig.PowerThreshold{
			Type:       "power_threshold",
			EntityID:   "sensor.ev_charger_power",
			ThresholdW: 200,
			Duration:   "5s",
		},
		Stop: bridgeconfig.PowerThreshold{
			Type:       "power_threshold",
			EntityID:   "sensor.ev_charger_power",
			ThresholdW: 50,
			Duration:   "10s",
		},
	}
}
