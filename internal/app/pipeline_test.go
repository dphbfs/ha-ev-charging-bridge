package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	bridgeconfig "ha-ev-charging-bridge/internal/config"
	"ha-ev-charging-bridge/internal/homeassistant"
	"ha-ev-charging-bridge/internal/persistence"
)

func TestInitializeStatesStartsActiveSessionWhenAlreadyCharging(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, ctx)
	charger := testCharger()
	pipeline, err := NewPipeline(ctx, charger, store)
	if err != nil {
		t.Fatalf("NewPipeline() error = %v", err)
	}

	states := []homeassistant.State{
		{
			EntityID:    charger.Entities.EnergyKWh,
			State:       "42.5",
			LastChanged: time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC),
		},
		{
			EntityID:    charger.Entities.PowerW,
			State:       "1386.9",
			LastChanged: time.Date(2026, 7, 24, 12, 0, 5, 0, time.UTC),
		},
	}
	if err := pipeline.InitializeStates(ctx, states); err != nil {
		t.Fatalf("InitializeStates() error = %v", err)
	}

	active, ok, err := store.ActiveSession(ctx, charger.ChargerID)
	if err != nil {
		t.Fatalf("ActiveSession() error = %v", err)
	}
	if !ok {
		t.Fatal("ActiveSession() ok = false, want startup active session")
	}
	if active.StartEnergyKWh == nil || *active.StartEnergyKWh != 42.5 {
		t.Fatalf("StartEnergyKWh = %v, want 42.5", active.StartEnergyKWh)
	}
}

func openTestStore(t *testing.T, ctx context.Context) *persistence.Store {
	t.Helper()
	store, err := persistence.Open(ctx, filepath.Join(t.TempDir(), "bridge.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	return store
}

func testCharger() bridgeconfig.Charger {
	return bridgeconfig.Charger{
		ChargerID:   "charger-1",
		EVSEID:      "evse-1",
		ConnectorID: "connector-1",
		MeterID:     "meter-1",
		Entities: bridgeconfig.EntityMapping{
			PowerW:    "sensor.ev_charger_power",
			EnergyKWh: "sensor.ev_charger_energy",
		},
		Start: bridgeconfig.PowerThreshold{
			Type:       "power_threshold",
			EntityID:   "sensor.ev_charger_power",
			ThresholdW: 200,
			Duration:   "0s",
		},
		Stop: bridgeconfig.PowerThreshold{
			Type:       "power_threshold",
			EntityID:   "sensor.ev_charger_power",
			ThresholdW: 50,
			Duration:   "0s",
		},
	}
}
