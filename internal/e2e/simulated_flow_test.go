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
	"ha-ev-charging-bridge/internal/app"
	bridgeconfig "ha-ev-charging-bridge/internal/config"
	"ha-ev-charging-bridge/internal/events"
	"ha-ev-charging-bridge/internal/homeassistant"
	"ha-ev-charging-bridge/internal/persistence"
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

	pipeline, err := app.NewPipeline(ctx, charger, store)
	if err != nil {
		t.Fatalf("NewPipeline() error = %v", err)
	}

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
	states, err := homeassistant.GetStates(conn, 1)
	if err != nil {
		t.Fatalf("GetStates() error = %v", err)
	}
	if err := pipeline.InitializeStates(ctx, states); err != nil {
		t.Fatalf("InitializeStates() error = %v", err)
	}
	if err := homeassistant.SubscribeStateChanges(conn, 2, app.EntityIDs(charger)); err != nil {
		t.Fatalf("SubscribeStateChanges() error = %v", err)
	}

	messages := make(chan []byte, 8)
	readErrors := make(chan error, 1)
	go homeassistant.ReadMessages(conn, messages, readErrors)

	var completedSessionID string
	for completedSessionID == "" {
		select {
		case payload := <-messages:
			completedSessionID = processPayload(t, ctx, payload, pipeline)
		case err := <-readErrors:
			select {
			case payload := <-messages:
				completedSessionID = processPayload(t, ctx, payload, pipeline)
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
	if completed.ChargerID != charger.ChargerID || completed.EVSEID != charger.EVSEID || completed.ConnectorID != charger.ConnectorID || completed.MeterID != charger.MeterID {
		t.Fatalf("completed identity = %#v, want configured charger identity", completed)
	}
	if completed.StartEnergyKWh == nil || completed.EndEnergyKWh == nil || completed.EnergyConsumedKWh == nil {
		t.Fatalf("completed energy fields = start %v end %v consumed %v, want all populated", completed.StartEnergyKWh, completed.EndEnergyKWh, completed.EnergyConsumedKWh)
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

func processPayload(t *testing.T, ctx context.Context, payload []byte, pipeline *app.Pipeline) string {
	t.Helper()
	completed, err := pipeline.ProcessPayload(ctx, payload)
	if err != nil {
		t.Fatalf("ProcessPayload() error = %v", err)
	}
	return completed
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
		var getStates homeassistant.CommandMessage
		readJSON(t, conn, &getStates)
		states, err := json.Marshal([]homeassistant.State{
			{EntityID: "sensor.ev_charger_power", State: "0", LastChanged: time.Date(2026, 7, 24, 11, 59, 0, 0, time.UTC)},
			{EntityID: "sensor.ev_charger_energy", State: "10.0", LastChanged: time.Date(2026, 7, 24, 11, 59, 0, 0, time.UTC)},
		})
		if err != nil {
			t.Fatalf("marshal states: %v", err)
		}
		writeJSON(t, conn, homeassistant.ResultMessage{ID: getStates.ID, Type: "result", Success: true, Result: states})

		var sub homeassistant.CommandMessage
		readJSON(t, conn, &sub)
		if sub.Type != "subscribe_trigger" || sub.Trigger == nil {
			t.Fatalf("subscribe command = %#v, want state trigger", sub)
		}
		writeJSON(t, conn, homeassistant.ResultMessage{ID: sub.ID, Type: "result", Success: true})

		base := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
		writeJSON(t, conn, haTriggerEvent("sensor.ev_charger_power", "250", base))
		writeJSON(t, conn, haTriggerEvent("sensor.ev_charger_energy", "10.5", base.Add(30*time.Second)))
		writeJSON(t, conn, haTriggerEvent("sensor.ev_charger_power", "40", base.Add(time.Minute)))
	}))
}

func haTriggerEvent(entityID, state string, at time.Time) map[string]any {
	return map[string]any{
		"event": map[string]any{
			"variables": map[string]any{
				"trigger": map[string]any{
					"entity_id": entityID,
					"to_state": map[string]any{
						"entity_id":    entityID,
						"state":        state,
						"last_changed": at.Format(time.RFC3339Nano),
					},
					"from_state": map[string]any{
						"entity_id":    entityID,
						"state":        "0",
						"last_changed": at.Add(-time.Second).Format(time.RFC3339Nano),
					},
				},
			},
		},
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
