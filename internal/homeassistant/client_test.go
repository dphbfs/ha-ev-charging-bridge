package homeassistant

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebsocketURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "http base URL",
			raw:  "http://ha.test:8123",
			want: "ws://ha.test:8123/api/websocket",
		},
		{
			name: "https base URL with path",
			raw:  "https://ha.test:8123/homeassistant",
			want: "wss://ha.test:8123/homeassistant/api/websocket",
		},
		{
			name: "ws URL drops query",
			raw:  "ws://ha.test:8123?token=ignored",
			want: "ws://ha.test:8123/api/websocket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := WebsocketURL(tt.raw)
			if err != nil {
				t.Fatalf("WebsocketURL() error = %v", err)
			}
			if got.String() != tt.want {
				t.Fatalf("WebsocketURL() = %q, want %q", got.String(), tt.want)
			}
		})
	}
}

func TestAuthenticateRejectsAuthInvalid(t *testing.T) {
	server := newTestHAServer(t, func(t *testing.T, conn *websocket.Conn) {
		writeJSON(t, conn, map[string]any{"type": "auth_required"})
		var auth AuthMessage
		readJSON(t, conn, &auth)
		writeJSON(t, conn, map[string]any{"type": "auth_invalid", "message": "bad token"})
	})
	defer server.Close()

	conn := dialTestHA(t, server.URL)
	defer conn.Close()

	err := Authenticate(conn, "bad-token")
	if err == nil {
		t.Fatal("Authenticate() error = nil, want auth failure")
	}
	if !strings.Contains(err.Error(), "bad token") {
		t.Fatalf("Authenticate() error = %q, want bad token context", err)
	}
}

func TestWebsocketAuthSubscribeAndEventDelivery(t *testing.T) {
	server := newTestHAServer(t, func(t *testing.T, conn *websocket.Conn) {
		writeJSON(t, conn, map[string]any{"type": "auth_required"})

		var auth AuthMessage
		readJSON(t, conn, &auth)
		if auth.Type != "auth" || auth.AccessToken != "test-token" {
			t.Fatalf("auth message = %#v", auth)
		}
		writeJSON(t, conn, map[string]any{"type": "auth_ok"})

		var sub CommandMessage
		readJSON(t, conn, &sub)
		if sub.Type != "subscribe_events" || sub.EventType != "state_changed" {
			t.Fatalf("subscribe command = %#v", sub)
		}
		writeJSON(t, conn, ResultMessage{ID: sub.ID, Type: "result", Success: true})

		writeJSON(t, conn, map[string]any{
			"id": sub.ID,
			"event": map[string]any{
				"time_fired": time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
				"data": map[string]any{
					"entity_id": "sensor.ev_charger_power",
					"new_state": map[string]any{
						"entity_id":    "sensor.ev_charger_power",
						"state":        "250",
						"last_changed": time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
					},
				},
			},
		})
	})
	defer server.Close()

	conn := dialTestHA(t, server.URL)
	defer conn.Close()

	if err := Authenticate(conn, "test-token"); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if err := Subscribe(conn, 2, "state_changed"); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	messages := make(chan []byte, 1)
	readErrors := make(chan error, 1)
	go ReadMessages(conn, messages, readErrors)

	select {
	case payload := <-messages:
		var msg EventMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("parse event payload: %v", err)
		}
		if msg.Event.Data.EntityID != "sensor.ev_charger_power" {
			t.Fatalf("entity_id = %q, want sensor.ev_charger_power", msg.Event.Data.EntityID)
		}
	case err := <-readErrors:
		t.Fatalf("ReadMessages() error = %v", err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event payload")
	}
}

func TestSubscribeStateChangesSendsConfiguredEntities(t *testing.T) {
	server := newTestHAServer(t, func(t *testing.T, conn *websocket.Conn) {
		var sub CommandMessage
		readJSON(t, conn, &sub)
		if sub.Type != "subscribe_trigger" {
			t.Fatalf("Type = %q, want subscribe_trigger", sub.Type)
		}
		if sub.Trigger == nil {
			t.Fatal("Trigger = nil, want state trigger")
		}
		if sub.Trigger.Platform != "state" {
			t.Fatalf("Trigger.Platform = %q, want state", sub.Trigger.Platform)
		}
		want := []string{"sensor.ev_charger_power", "sensor.ev_charger_energy"}
		if len(sub.Trigger.EntityID) != len(want) {
			t.Fatalf("Trigger.EntityID = %#v, want %#v", sub.Trigger.EntityID, want)
		}
		for i := range want {
			if sub.Trigger.EntityID[i] != want[i] {
				t.Fatalf("Trigger.EntityID = %#v, want %#v", sub.Trigger.EntityID, want)
			}
		}
		writeJSON(t, conn, ResultMessage{ID: sub.ID, Type: "result", Success: true})
	})
	defer server.Close()

	conn := dialTestHA(t, server.URL)
	defer conn.Close()

	err := SubscribeStateChanges(conn, 2, []string{
		" sensor.ev_charger_power ",
		"sensor.ev_charger_energy",
		"sensor.ev_charger_power",
		"",
	})
	if err != nil {
		t.Fatalf("SubscribeStateChanges() error = %v", err)
	}
}

func TestParseEventMessageNormalizesStateTriggerPayload(t *testing.T) {
	at := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	payload, err := json.Marshal(map[string]any{
		"id": 2,
		"event": map[string]any{
			"variables": map[string]any{
				"trigger": map[string]any{
					"entity_id": "sensor.ev_charger_power",
					"to_state": map[string]any{
						"entity_id":    "sensor.ev_charger_power",
						"state":        "250",
						"last_changed": at.Format(time.RFC3339Nano),
					},
					"from_state": map[string]any{
						"entity_id":    "sensor.ev_charger_power",
						"state":        "0",
						"last_changed": at.Add(-time.Minute).Format(time.RFC3339Nano),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	msg, ok, err := ParseEventMessage(payload)
	if err != nil {
		t.Fatalf("ParseEventMessage() error = %v", err)
	}
	if !ok {
		t.Fatal("ParseEventMessage() ok = false, want true")
	}
	if msg.Event.Data.EntityID != "sensor.ev_charger_power" {
		t.Fatalf("EntityID = %q, want sensor.ev_charger_power", msg.Event.Data.EntityID)
	}
	if msg.Event.Data.NewState == nil || msg.Event.Data.NewState.State != "250" {
		t.Fatalf("NewState = %#v, want state 250", msg.Event.Data.NewState)
	}
	if msg.Event.Data.OldState == nil || msg.Event.Data.OldState.State != "0" {
		t.Fatalf("OldState = %#v, want state 0", msg.Event.Data.OldState)
	}
}

func newTestHAServer(t *testing.T, handle func(*testing.T, *websocket.Conn)) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/websocket" {
			t.Fatalf("path = %q, want /api/websocket", r.URL.Path)
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade websocket: %v", err)
		}
		defer conn.Close()
		handle(t, conn)
	}))
}

func dialTestHA(t *testing.T, rawURL string) *websocket.Conn {
	t.Helper()
	wsURL, err := WebsocketURL(rawURL)
	if err != nil {
		t.Fatalf("WebsocketURL() error = %v", err)
	}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	return conn
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
