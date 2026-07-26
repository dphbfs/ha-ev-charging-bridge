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

func TestDetectEmitsChargingStoppedFromConfiguredStopEvent(t *testing.T) {
	cfg := chargerConfig()
	cfg.Stop.Events = []bridgeconfig.Event{
		{EntityID: "switch.charger_plug", State: "off", Reason: "smart_plug_off"},
	}
	d := newDetector(t, cfg)
	at := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	got, err := d.Detect(event("switch.charger_plug", "off", at), State{ActiveSession: true})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	stop, ok := findEvent(got, events.ChargingStopped)
	if !ok {
		t.Fatalf("Detect() events = %#v, want charging_stopped", got)
	}
	if stop.Reason != "smart_plug_off" {
		t.Fatalf("Reason = %q, want smart_plug_off", stop.Reason)
	}
}

func TestDetectEmitsChargingStoppedFromConfiguredStopRuleList(t *testing.T) {
	cfg := chargerConfig()
	cfg.Stop.Rules = []bridgeconfig.PowerThreshold{
		cfg.Stop,
		{Type: "device_offline", EntityID: "switch.charger_plug", State: "off", Reason: "smart_plug_off"},
	}
	d := newDetector(t, cfg)
	at := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	got, err := d.Detect(event("switch.charger_plug", "off", at), State{ActiveSession: true})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	stop, ok := findEvent(got, events.ChargingStopped)
	if !ok {
		t.Fatalf("Detect() events = %#v, want charging_stopped", got)
	}
	if stop.Reason != "smart_plug_off" {
		t.Fatalf("Reason = %q, want smart_plug_off", stop.Reason)
	}
}

func TestDetectDebouncesConfiguredStopRuleListEvent(t *testing.T) {
	cfg := chargerConfig()
	cfg.Stop.Rules = []bridgeconfig.PowerThreshold{
		cfg.Stop,
		{Type: "device_offline", EntityID: "switch.charger_plug", State: "off", Duration: "10s", Reason: "smart_plug_off"},
	}
	d := newDetector(t, cfg)
	start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	got, err := d.Detect(event("switch.charger_plug", "off", start), State{ActiveSession: true})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if hasEvent(got, events.ChargingStopped) {
		t.Fatalf("Detect() events = %#v, did not want charging_stopped before duration", got)
	}

	got, err = d.Detect(event("switch.charger_plug", "off", start.Add(10*time.Second)), State{ActiveSession: true})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !hasEvent(got, events.ChargingStopped) {
		t.Fatalf("Detect() events = %#v, want charging_stopped", got)
	}
}

func TestDetectIgnoresConfiguredStopEventWithoutActiveSession(t *testing.T) {
	cfg := chargerConfig()
	cfg.Stop.Events = []bridgeconfig.Event{
		{EntityID: "switch.charger_plug", State: "off", Reason: "smart_plug_off"},
	}
	d := newDetector(t, cfg)

	got, err := d.Detect(event("switch.charger_plug", "off", time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)), State{})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if hasEvent(got, events.ChargingStopped) {
		t.Fatalf("Detect() events = %#v, did not want charging_stopped", got)
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
	_, ok := findEvent(values, eventType)
	return ok
}

func findEvent(values []events.ChargerEvent, eventType events.ChargerEventType) (events.ChargerEvent, bool) {
	for _, value := range values {
		if value.Type == eventType {
			return value, true
		}
	}
	return events.ChargerEvent{}, false
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
