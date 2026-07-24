package meter

import (
	"testing"
	"time"

	bridgeconfig "ha-ev-charging-bridge/internal/config"
	"ha-ev-charging-bridge/internal/events"
)

func TestAggregatorAveragesPowerValues(t *testing.T) {
	agg := newAggregator(t, bridgeconfig.Meter{
		MeterID:               "power-meter",
		EntityID:              "sensor.ev_charger_power",
		Unit:                  "W",
		Aggregation:           "average",
		OutsideSessionStorage: "save",
	})

	agg.Observe(meterValue(100), "session-1")
	agg.Observe(meterValue(200), "session-1")
	agg.Observe(meterValue(300), "session-1")

	got, ok, err := agg.Flush(time.Date(2026, 7, 24, 12, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if !ok {
		t.Fatal("Flush() ok = false, want aggregate")
	}
	if got.Value != 200 {
		t.Fatalf("Value = %.1f, want 200", got.Value)
	}
}

func TestAggregatorUsesLastEnergyValue(t *testing.T) {
	agg := newAggregator(t, bridgeconfig.Meter{
		MeterID:               "energy-meter",
		EntityID:              "sensor.ev_charger_energy",
		Unit:                  "kWh",
		Aggregation:           "last",
		OutsideSessionStorage: "save",
	})

	agg.Observe(meterValue(10.1), "session-1")
	agg.Observe(meterValue(10.4), "session-1")

	got, ok, err := agg.Flush(time.Date(2026, 7, 24, 12, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if !ok {
		t.Fatal("Flush() ok = false, want aggregate")
	}
	if got.Value != 10.4 {
		t.Fatalf("Value = %.1f, want 10.4", got.Value)
	}
}

func TestAggregatorDropsValuesOutsideSessionWhenConfigured(t *testing.T) {
	agg := newAggregator(t, bridgeconfig.Meter{
		MeterID:               "power-meter",
		EntityID:              "sensor.ev_charger_power",
		Unit:                  "W",
		Aggregation:           "average",
		OutsideSessionStorage: "drop",
	})

	agg.Observe(meterValue(100), "")

	_, ok, err := agg.Flush(time.Date(2026, 7, 24, 12, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if ok {
		t.Fatal("Flush() ok = true, want dropped outside-session value")
	}
}

func TestAggregatorSavesValuesOutsideSessionWhenConfigured(t *testing.T) {
	agg := newAggregator(t, bridgeconfig.Meter{
		MeterID:               "energy-meter",
		EntityID:              "sensor.ev_charger_energy",
		Unit:                  "kWh",
		Aggregation:           "last",
		OutsideSessionStorage: "save",
	})

	agg.Observe(meterValue(10.1), "")

	got, ok, err := agg.Flush(time.Date(2026, 7, 24, 12, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if !ok {
		t.Fatal("Flush() ok = false, want saved outside-session value")
	}
	if got.SessionID != "" {
		t.Fatalf("SessionID = %q, want empty outside session", got.SessionID)
	}
}

func TestAggregatorLinksValuesToActiveSession(t *testing.T) {
	agg := newAggregator(t, bridgeconfig.Meter{
		MeterID:               "power-meter",
		EntityID:              "sensor.ev_charger_power",
		Unit:                  "W",
		Aggregation:           "average",
		OutsideSessionStorage: "drop",
	})

	agg.Observe(meterValue(120), "session-1")

	got, ok, err := agg.Flush(time.Date(2026, 7, 24, 12, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if !ok {
		t.Fatal("Flush() ok = false, want aggregate")
	}
	if got.SessionID != "session-1" {
		t.Fatalf("SessionID = %q, want session-1", got.SessionID)
	}
}

func newAggregator(t *testing.T, cfg bridgeconfig.Meter) *Aggregator {
	t.Helper()
	agg, err := NewAggregator(cfg)
	if err != nil {
		t.Fatalf("NewAggregator() error = %v", err)
	}
	return agg
}

func meterValue(value float64) events.MeterValue {
	return events.MeterValue{
		ChargerID:  "charger-1",
		Value:      value,
		ObservedAt: time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC),
	}
}
