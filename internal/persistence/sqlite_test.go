package persistence

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	bridgeconfig "ha-ev-charging-bridge/internal/config"
	"ha-ev-charging-bridge/internal/events"
)

func TestStorePersistsCharger(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, ctx)

	charger := bridgeconfig.Charger{ChargerID: "charger-1", EVSEID: "evse-1", ConnectorID: "connector-1", MeterID: "meter-1"}
	if err := store.SaveCharger(ctx, charger); err != nil {
		t.Fatalf("SaveCharger() error = %v", err)
	}

	got, ok, err := store.Charger(ctx, "charger-1")
	if err != nil {
		t.Fatalf("Charger() error = %v", err)
	}
	if !ok {
		t.Fatal("Charger() ok = false, want charger")
	}
	if got.ChargerID != charger.ChargerID || got.EVSEID != charger.EVSEID || got.ConnectorID != charger.ConnectorID || got.MeterID != charger.MeterID {
		t.Fatalf("Charger() = %#v, want %#v", got, charger)
	}
}

func TestStorePersistsAndReloadsActiveSession(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, ctx)
	session := testSession(events.SessionCharging)

	if err := store.SaveActiveSession(ctx, session); err != nil {
		t.Fatalf("SaveActiveSession() error = %v", err)
	}

	got, ok, err := store.ActiveSession(ctx, "charger-1")
	if err != nil {
		t.Fatalf("ActiveSession() error = %v", err)
	}
	if !ok {
		t.Fatal("ActiveSession() ok = false, want active session")
	}
	if got.ID != session.ID || got.State != events.SessionCharging {
		t.Fatalf("ActiveSession() = %#v, want %#v", got, session)
	}
}

func TestStorePersistsAndQueriesCompletedSession(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, ctx)
	session := testSession(events.SessionEndedState)
	endedAt := session.StartedAt.Add(time.Hour)
	session.EndedAt = &endedAt

	if err := store.SaveCompletedSession(ctx, session); err != nil {
		t.Fatalf("SaveCompletedSession() error = %v", err)
	}

	got, ok, err := store.CompletedSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("CompletedSession() error = %v", err)
	}
	if !ok {
		t.Fatal("CompletedSession() ok = false, want completed session")
	}
	if got.ID != session.ID || got.State != events.SessionEndedState || got.EndedAt == nil || !got.EndedAt.Equal(endedAt) {
		t.Fatalf("CompletedSession() = %#v, want completed session with ended_at", got)
	}
}

func TestStorePersistsAndQueriesMeterValues(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, ctx)

	value := events.MeterValue{
		SessionID:  "session-1",
		ChargerID:  "charger-1",
		MeterID:    "meter-1",
		EntityID:   "sensor.ev_charger_energy",
		Unit:       "kWh",
		Value:      12.5,
		ObservedAt: time.Date(2026, 7, 24, 12, 30, 0, 0, time.UTC),
	}
	if err := store.SaveMeterValue(ctx, value); err != nil {
		t.Fatalf("SaveMeterValue() error = %v", err)
	}

	got, err := store.MeterValues(ctx, "session-1")
	if err != nil {
		t.Fatalf("MeterValues() error = %v", err)
	}
	if len(got) != 1 || got[0].Value != value.Value || got[0].SessionID != value.SessionID {
		t.Fatalf("MeterValues() = %#v, want one matching value", got)
	}
}

func TestStorePersistsAndQueriesLifecycleEvents(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, ctx)

	event := events.SessionEvent{
		Type:       events.SessionStarted,
		SessionID:  "session-1",
		ChargerID:  "charger-1",
		OccurredAt: time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC),
	}
	if err := store.SaveSessionEvent(ctx, event); err != nil {
		t.Fatalf("SaveSessionEvent() error = %v", err)
	}

	got, err := store.SessionEvents(ctx, "session-1")
	if err != nil {
		t.Fatalf("SessionEvents() error = %v", err)
	}
	if len(got) != 1 || got[0].Type != events.SessionStarted || got[0].SessionID != event.SessionID {
		t.Fatalf("SessionEvents() = %#v, want one session_started event", got)
	}
}

func TestStoreDeletesMeterValuesOlderThanRetentionCutoff(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, ctx)
	cutoff := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	oldValue := testMeterValue("session-1", cutoff.Add(-time.Minute), 10)
	newValue := testMeterValue("session-1", cutoff.Add(time.Minute), 11)
	if err := store.SaveMeterValue(ctx, oldValue); err != nil {
		t.Fatalf("SaveMeterValue(old) error = %v", err)
	}
	if err := store.SaveMeterValue(ctx, newValue); err != nil {
		t.Fatalf("SaveMeterValue(new) error = %v", err)
	}

	deleted, err := store.DeleteMeterValuesBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteMeterValuesBefore() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	got, err := store.MeterValues(ctx, "session-1")
	if err != nil {
		t.Fatalf("MeterValues() error = %v", err)
	}
	if len(got) != 1 || got[0].Value != newValue.Value {
		t.Fatalf("MeterValues() = %#v, want only newer value", got)
	}
}

func TestStoreDeletesSessionEventsOlderThanRetentionCutoff(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, ctx)
	cutoff := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)

	oldEvent := testSessionEvent("session-1", events.SessionStarted, cutoff.Add(-time.Minute))
	newEvent := testSessionEvent("session-1", events.SessionEnded, cutoff.Add(time.Minute))
	if err := store.SaveSessionEvent(ctx, oldEvent); err != nil {
		t.Fatalf("SaveSessionEvent(old) error = %v", err)
	}
	if err := store.SaveSessionEvent(ctx, newEvent); err != nil {
		t.Fatalf("SaveSessionEvent(new) error = %v", err)
	}

	deleted, err := store.DeleteSessionEventsBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("DeleteSessionEventsBefore() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	got, err := store.SessionEvents(ctx, "session-1")
	if err != nil {
		t.Fatalf("SessionEvents() error = %v", err)
	}
	if len(got) != 1 || got[0].Type != newEvent.Type {
		t.Fatalf("SessionEvents() = %#v, want only newer event", got)
	}
}

func TestStoreRetentionPreservesCompletedSessionSummaries(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, ctx)
	session := testSession(events.SessionEndedState)
	endedAt := session.StartedAt.Add(time.Hour)
	session.EndedAt = &endedAt
	if err := store.SaveCompletedSession(ctx, session); err != nil {
		t.Fatalf("SaveCompletedSession() error = %v", err)
	}

	cutoff := session.StartedAt.Add(24 * time.Hour)
	if _, err := store.DeleteMeterValuesBefore(ctx, cutoff); err != nil {
		t.Fatalf("DeleteMeterValuesBefore() error = %v", err)
	}
	if _, err := store.DeleteSessionEventsBefore(ctx, cutoff); err != nil {
		t.Fatalf("DeleteSessionEventsBefore() error = %v", err)
	}

	_, ok, err := store.CompletedSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("CompletedSession() error = %v", err)
	}
	if !ok {
		t.Fatal("CompletedSession() ok = false, want completed session preserved")
	}
}

func openTestStore(t *testing.T, ctx context.Context) *Store {
	t.Helper()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "bridge.db"))
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

func testSession(state events.SessionState) events.Session {
	startEnergy := 10.0
	endEnergy := 12.5
	consumed := 2.5
	return events.Session{
		ID:                "session-1",
		ChargerID:         "charger-1",
		EVSEID:            "evse-1",
		ConnectorID:       "connector-1",
		MeterID:           "meter-1",
		State:             state,
		StartedAt:         time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC),
		StartEnergyKWh:    &startEnergy,
		EndEnergyKWh:      &endEnergy,
		EnergyConsumedKWh: &consumed,
	}
}

func testMeterValue(sessionID string, at time.Time, value float64) events.MeterValue {
	return events.MeterValue{
		SessionID:  sessionID,
		ChargerID:  "charger-1",
		MeterID:    "meter-1",
		EntityID:   "sensor.ev_charger_energy",
		Unit:       "kWh",
		Value:      value,
		ObservedAt: at,
	}
}

func testSessionEvent(sessionID string, eventType events.SessionEventType, at time.Time) events.SessionEvent {
	return events.SessionEvent{
		Type:       eventType,
		SessionID:  sessionID,
		ChargerID:  "charger-1",
		OccurredAt: at,
	}
}
