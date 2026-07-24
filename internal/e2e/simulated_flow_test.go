package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	bridgeconfig "ha-ev-charging-bridge/internal/config"
	"ha-ev-charging-bridge/internal/detector"
	"ha-ev-charging-bridge/internal/events"
	"ha-ev-charging-bridge/internal/homeassistant"
	"ha-ev-charging-bridge/internal/persistence"
	"ha-ev-charging-bridge/internal/router"
	"ha-ev-charging-bridge/internal/session"
)

func TestSimulatedHomeAssistantFlowCompletesSession(t *testing.T) {
	ctx := context.Background()
	charger := testCharger()
	server := simulatedHAServer(t)
	defer server.Close()

	store, err := persistence.Open(ctx, filepath.Join(t.TempDir(), "bridge.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	det, err := detector.New(charger)
	if err != nil {
		t.Fatalf("detector.New() error = %v", err)
	}
	sessions := session.NewProcessor()
	r, err := router.New([]string{charger.Entities.PowerW, charger.Entities.EnergyKWh}, 1)
	if err != nil {
		t.Fatalf("router.New() error = %v", err)
	}
	channels := r.Channels()

	wsURL, err := homeassistant.WebsocketURL(server.URL)
	if err != nil {
		t.Fatalf("WebsocketURL() error = %v", err)
	}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	if err := homeassistant.Authenticate(conn, "test-token"); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if err := homeassistant.Subscribe(conn, 1, "state_changed"); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	messages := make(chan []byte, 8)
	readErrors := make(chan error, 1)
	go homeassistant.ReadMessages(conn, messages, readErrors)

	var completedSessionID string
	for completedSessionID == "" {
		select {
		case payload := <-messages:
			completedSessionID = processPayload(t, ctx, payload, r, channels, det, sessions, store)
		case err := <-readErrors:
			select {
			case payload := <-messages:
				completedSessionID = processPayload(t, ctx, payload, r, channels, det, sessions, store)
			default:
				t.Fatalf("ReadMessages() error = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for completed session")
		}
	}

	completed, ok, err := store.CompletedSession(ctx, completedSessionID)
	if err != nil {
		t.Fatalf("CompletedSession() error = %v", err)
	}
	if !ok {
		t.Fatal("CompletedSession() ok = false, want completed session")
	}
	if completed.State != events.SessionEndedState {
		t.Fatalf("completed.State = %q, want ended", completed.State)
	}

	meterValues, err := store.MeterValues(ctx, completedSessionID)
	if err != nil {
		t.Fatalf("MeterValues() error = %v", err)
	}
	if len(meterValues) == 0 {
		t.Fatal("MeterValues() is empty, want recorded meter values")
	}

	lifecycle, err := store.SessionEvents(ctx, completedSessionID)
	if err != nil {
		t.Fatalf("SessionEvents() error = %v", err)
	}
	if !hasSessionEvent(lifecycle, events.SessionStarted) || !hasSessionEvent(lifecycle, events.SessionEnded) {
		t.Fatalf("SessionEvents() = %#v, want started and ended", lifecycle)
	}
}

func processPayload(t *testing.T, ctx context.Context, payload []byte, r *router.Router, channels map[string]<-chan homeassistant.EventMessage, det *detector.Detector, sessions *session.Processor, store *persistence.Store) string {
	t.Helper()
	var msg homeassistant.EventMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("parse websocket event: %v", err)
	}
	if _, err := r.Route(ctx, msg); err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	return drainRoutedEvents(t, ctx, channels, det, sessions, store)
}

func drainRoutedEvents(t *testing.T, ctx context.Context, channels map[string]<-chan homeassistant.EventMessage, det *detector.Detector, sessions *session.Processor, store *persistence.Store) string {
	t.Helper()
	for {
		select {
		case msg := <-channels["sensor.ev_charger_power"]:
			if completed := handleEntityEvent(t, ctx, msg, det, sessions, store); completed != "" {
				return completed
			}
		case msg := <-channels["sensor.ev_charger_energy"]:
			if completed := handleEntityEvent(t, ctx, msg, det, sessions, store); completed != "" {
				return completed
			}
		default:
			return ""
		}
	}
}

func handleEntityEvent(t *testing.T, ctx context.Context, msg homeassistant.EventMessage, det *detector.Detector, sessions *session.Processor, store *persistence.Store) string {
	t.Helper()
	_, active := sessions.Active("charger-1")
	chargerEvents, err := det.Detect(msg, detector.State{ActiveSession: active})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	for _, chargerEvent := range chargerEvents {
		if chargerEvent.Type == events.MeterValueObserved {
			if activeSession, ok := sessions.Active(chargerEvent.ChargerID); ok {
				if err := store.SaveMeterValue(ctx, meterValueFromChargerEvent(chargerEvent, activeSession.ID)); err != nil {
					t.Fatalf("SaveMeterValue() error = %v", err)
				}
			}
		}

		sessionEvents, err := sessions.Apply(chargerEvent)
		if err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		for _, sessionEvent := range sessionEvents {
			if err := store.SaveSessionEvent(ctx, sessionEvent); err != nil {
				t.Fatalf("SaveSessionEvent() error = %v", err)
			}
			if sessionEvent.Type == events.SessionStarted {
				activeSession, ok := sessions.Active(chargerEvent.ChargerID)
				if !ok {
					t.Fatal("session_started emitted without active session")
				}
				if err := store.SaveActiveSession(ctx, *activeSession); err != nil {
					t.Fatalf("SaveActiveSession() error = %v", err)
				}
			}
			if sessionEvent.Type == events.SessionEnded {
				completed := events.Session{
					ID:          sessionEvent.SessionID,
					ChargerID:   sessionEvent.ChargerID,
					EVSEID:      chargerEvent.EVSEID,
					ConnectorID: chargerEvent.ConnectorID,
					State:       events.SessionEndedState,
					StartedAt:   sessionEvent.OccurredAt.Add(-time.Minute),
					EndedAt:     &sessionEvent.OccurredAt,
				}
				if err := store.SaveCompletedSession(ctx, completed); err != nil {
					t.Fatalf("SaveCompletedSession() error = %v", err)
				}
				return sessionEvent.SessionID
			}
		}
	}
	return ""
}

func simulatedHAServer(t *testing.T) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade websocket: %v", err)
		}
		defer conn.Close()

		writeJSON(t, conn, map[string]any{"type": "auth_required"})
		var auth homeassistant.AuthMessage
		readJSON(t, conn, &auth)
		writeJSON(t, conn, map[string]any{"type": "auth_ok"})
		var sub homeassistant.CommandMessage
		readJSON(t, conn, &sub)
		writeJSON(t, conn, homeassistant.ResultMessage{ID: sub.ID, Type: "result", Success: true})

		base := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
		writeJSON(t, conn, haEvent("sensor.ev_charger_power", "250", base))
		writeJSON(t, conn, haEvent("sensor.ev_charger_energy", "10.5", base.Add(30*time.Second)))
		writeJSON(t, conn, haEvent("sensor.ev_charger_power", "40", base.Add(time.Minute)))
	}))
}

func haEvent(entityID, state string, at time.Time) map[string]any {
	return map[string]any{
		"event": map[string]any{
			"time_fired": at.Format(time.RFC3339Nano),
			"data": map[string]any{
				"entity_id": entityID,
				"new_state": map[string]any{
					"entity_id":    entityID,
					"state":        state,
					"last_changed": at.Format(time.RFC3339Nano),
				},
			},
		},
	}
}

func meterValueFromChargerEvent(event events.ChargerEvent, sessionID string) events.MeterValue {
	value := 0.0
	unit := "W"
	if event.EnergyKWh != nil {
		value = *event.EnergyKWh
		unit = "kWh"
	}
	if event.PowerW != nil {
		value = *event.PowerW
	}
	return events.MeterValue{
		SessionID:  sessionID,
		ChargerID:  event.ChargerID,
		MeterID:    "meter-1",
		EntityID:   event.EntityID,
		Unit:       unit,
		Value:      value,
		ObservedAt: event.OccurredAt,
	}
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

func hasSessionEvent(values []events.SessionEvent, eventType events.SessionEventType) bool {
	for _, value := range values {
		if value.Type == eventType {
			return true
		}
	}
	return false
}

func readJSON(t *testing.T, conn *websocket.Conn, value any) {
	t.Helper()
	if err := conn.ReadJSON(value); err != nil {
		t.Fatalf("read json: %v", err)
	}
}

func writeJSON(t *testing.T, conn *websocket.Conn, value any) {
	t.Helper()
	if err := conn.WriteJSON(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}
