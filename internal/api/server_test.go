package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	bridgeconfig "ha-ev-charging-bridge/internal/config"
	"ha-ev-charging-bridge/internal/events"
)

func TestServerListSessions(t *testing.T) {
	started := time.Date(2024, 5, 19, 9, 15, 0, 0, time.UTC)
	store := fakeStore{completed: []events.Session{
		{ID: "session-1", ChargerID: "charger-1", EVSEID: "evse-1", ConnectorID: "conn-1", MeterID: "meter-1", State: events.SessionEndedState, StartedAt: started},
		{ID: "session-2", ChargerID: "charger-2", EVSEID: "evse-2", ConnectorID: "conn-2", MeterID: "meter-2", State: events.SessionEndedState, StartedAt: started.Add(-time.Hour)},
	}}
	server := New(store, []bridgeconfig.Charger{{ChargerID: "charger-1", ChargerName: "Garage Charger"}})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?search=garage&limit=10", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got listResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Total != 1 || len(got.Sessions) != 1 || got.Sessions[0].ID != "session-1" {
		t.Fatalf("response = %#v, want filtered session-1", got)
	}
}

func TestServerRejectsInvalidPagination(t *testing.T) {
	server := New(fakeStore{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?limit=0", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServerActiveSessions(t *testing.T) {
	store := fakeStore{active: []events.Session{{ID: "active-1", ChargerID: "charger-1", State: events.SessionCharging, StartedAt: time.Now().UTC()}}}
	server := New(store, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/active", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got struct {
		Sessions []sessionDTO `json:"sessions"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Sessions) != 1 || got.Sessions[0].Status != "charging" {
		t.Fatalf("sessions = %#v, want one charging session", got.Sessions)
	}
}

func TestServerSessionDetailNotFound(t *testing.T) {
	server := New(fakeStore{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/missing", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestServerSessionDetail(t *testing.T) {
	started := time.Date(2024, 5, 19, 14, 30, 0, 0, time.UTC)
	store := fakeStore{
		completed:   []events.Session{{ID: "session-1", ChargerID: "charger-1", State: events.SessionEndedState, StartedAt: started}},
		meterValues: map[string][]events.MeterValue{"session-1": {{SessionID: "session-1", ChargerID: "charger-1", MeterID: "meter-1", Unit: "kWh", Value: 12.47, ObservedAt: started}}},
		events:      map[string][]events.SessionEvent{"session-1": {{SessionID: "session-1", ChargerID: "charger-1", Type: events.SessionStarted, OccurredAt: started}}},
	}
	server := New(store, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

type fakeStore struct {
	active      []events.Session
	completed   []events.Session
	meterValues map[string][]events.MeterValue
	events      map[string][]events.SessionEvent
}

func (s fakeStore) ActiveSessions(context.Context) ([]events.Session, error) {
	return append([]events.Session(nil), s.active...), nil
}

func (s fakeStore) CompletedSessions(context.Context) ([]events.Session, error) {
	return append([]events.Session(nil), s.completed...), nil
}

func (s fakeStore) CompletedSession(_ context.Context, id string) (events.Session, bool, error) {
	for _, session := range s.completed {
		if session.ID == id {
			return session, true, nil
		}
	}
	return events.Session{}, false, nil
}

func (s fakeStore) MeterValues(_ context.Context, id string) ([]events.MeterValue, error) {
	return append([]events.MeterValue(nil), s.meterValues[id]...), nil
}

func (s fakeStore) SessionEvents(_ context.Context, id string) ([]events.SessionEvent, error) {
	return append([]events.SessionEvent(nil), s.events[id]...), nil
}
