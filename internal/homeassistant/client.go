package homeassistant

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type AuthRequiredMessage struct {
	Type string `json:"type"`
}

type AuthMessage struct {
	Type        string `json:"type"`
	AccessToken string `json:"access_token"`
}

type CommandMessage struct {
	ID        int           `json:"id"`
	Type      string        `json:"type"`
	EventType string        `json:"event_type,omitempty"`
	Trigger   *StateTrigger `json:"trigger,omitempty"`
}

type StateTrigger struct {
	Platform string   `json:"platform"`
	EntityID []string `json:"entity_id"`
}

type ResultMessage struct {
	ID      int             `json:"id"`
	Type    string          `json:"type"`
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

type State struct {
	EntityID    string          `json:"entity_id"`
	State       string          `json:"state"`
	LastChanged time.Time       `json:"last_changed"`
	Raw         json.RawMessage `json:"-"`
}

type EventMessage struct {
	ID    int `json:"id"`
	Event struct {
		TimeFired time.Time `json:"time_fired"`
		Data      struct {
			EntityID string `json:"entity_id"`
			NewState *State `json:"new_state"`
			OldState *State `json:"old_state"`
		} `json:"data"`
	} `json:"event"`
}

type TriggerMessage struct {
	ID    int `json:"id"`
	Event struct {
		Variables struct {
			Trigger struct {
				EntityID  string `json:"entity_id"`
				FromState *State `json:"from_state"`
				ToState   *State `json:"to_state"`
			} `json:"trigger"`
		} `json:"variables"`
	} `json:"event"`
}

func WebsocketURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse Home Assistant URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("Home Assistant URL must include scheme and host: %q", raw)
	}

	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return nil, fmt.Errorf("unsupported Home Assistant URL scheme %q", parsed.Scheme)
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api/websocket"
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return parsed, nil
}

func Authenticate(conn *websocket.Conn, token string) error {
	var authRequired AuthRequiredMessage
	if err := conn.ReadJSON(&authRequired); err != nil {
		return fmt.Errorf("read auth_required message: %w", err)
	}
	if authRequired.Type != "auth_required" {
		return fmt.Errorf("expected auth_required message, got %q", authRequired.Type)
	}

	if err := conn.WriteJSON(AuthMessage{Type: "auth", AccessToken: token}); err != nil {
		return fmt.Errorf("send auth message: %w", err)
	}

	var response struct {
		Type    string          `json:"type"`
		Message string          `json:"message,omitempty"`
		Error   json.RawMessage `json:"error,omitempty"`
	}
	if err := conn.ReadJSON(&response); err != nil {
		return fmt.Errorf("read auth response: %w", err)
	}
	if response.Type != "auth_ok" {
		if len(response.Error) > 0 {
			return fmt.Errorf("Home Assistant authentication failed: %s", response.Error)
		}
		if response.Message != "" {
			return fmt.Errorf("Home Assistant authentication failed: %s", response.Message)
		}
		return fmt.Errorf("Home Assistant authentication failed: %s", response.Type)
	}

	slog.Info("authenticated with Home Assistant")
	return nil
}

func GetStates(conn *websocket.Conn, id int) ([]State, error) {
	if err := conn.WriteJSON(CommandMessage{ID: id, Type: "get_states"}); err != nil {
		return nil, fmt.Errorf("send get_states command: %w", err)
	}

	var result ResultMessage
	if err := conn.ReadJSON(&result); err != nil {
		return nil, fmt.Errorf("read get_states response: %w", err)
	}
	if result.Type != "result" || result.ID != id || !result.Success {
		if len(result.Error) > 0 {
			return nil, fmt.Errorf("get_states failed: %s", result.Error)
		}
		return nil, fmt.Errorf("get_states failed: type=%q id=%d success=%t", result.Type, result.ID, result.Success)
	}

	var states []State
	if err := json.Unmarshal(result.Result, &states); err != nil {
		return nil, fmt.Errorf("parse get_states response: %w", err)
	}

	slog.Info("loaded Home Assistant states", "count", len(states))
	return states, nil
}

func Subscribe(conn *websocket.Conn, id int, eventType string) error {
	cmd := CommandMessage{ID: id, Type: "subscribe_events", EventType: strings.TrimSpace(eventType)}
	return subscribe(conn, cmd)
}

func SubscribeStateChanges(conn *websocket.Conn, id int, entityIDs []string) error {
	trimmed := uniqueEntityIDs(entityIDs)
	cmd := CommandMessage{
		ID:   id,
		Type: "subscribe_trigger",
		Trigger: &StateTrigger{
			Platform: "state",
			EntityID: trimmed,
		},
	}
	return subscribe(conn, cmd)
}

func subscribe(conn *websocket.Conn, cmd CommandMessage) error {
	if err := conn.WriteJSON(cmd); err != nil {
		return fmt.Errorf("send %s command: %w", cmd.Type, err)
	}

	var result ResultMessage
	if err := conn.ReadJSON(&result); err != nil {
		return fmt.Errorf("read %s response: %w", cmd.Type, err)
	}
	if result.Type != "result" || result.ID != cmd.ID || !result.Success {
		if len(result.Error) > 0 {
			return fmt.Errorf("%s failed: %s", cmd.Type, result.Error)
		}
		return fmt.Errorf("%s failed: type=%q id=%d success=%t", cmd.Type, result.Type, result.ID, result.Success)
	}

	if cmd.Type == "subscribe_trigger" {
		slog.Info("subscribed to configured Home Assistant entities", "entity_count", len(cmd.Trigger.EntityID))
	} else if cmd.EventType == "" {
		slog.Info("subscribed to all Home Assistant events")
	} else {
		slog.Info("subscribed to Home Assistant event type", "event_type", cmd.EventType)
	}

	conn.SetReadLimit(10 * 1024 * 1024)
	return nil
}

func ParseEventMessage(payload []byte) (EventMessage, bool, error) {
	var msg EventMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return EventMessage{}, false, err
	}
	if msg.Event.Data.EntityID != "" || msg.Event.Data.NewState != nil {
		return msg, true, nil
	}

	var trigger TriggerMessage
	if err := json.Unmarshal(payload, &trigger); err != nil {
		return EventMessage{}, false, err
	}
	stateTrigger := trigger.Event.Variables.Trigger
	if stateTrigger.EntityID == "" && stateTrigger.ToState == nil {
		return EventMessage{}, false, nil
	}

	msg.ID = trigger.ID
	if stateTrigger.ToState != nil {
		msg.Event.TimeFired = stateTrigger.ToState.LastChanged
	}
	msg.Event.Data.EntityID = stateTrigger.EntityID
	msg.Event.Data.NewState = stateTrigger.ToState
	msg.Event.Data.OldState = stateTrigger.FromState
	return msg, true, nil
}

func uniqueEntityIDs(entityIDs []string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	for _, entityID := range entityIDs {
		entityID = strings.TrimSpace(entityID)
		if entityID == "" {
			continue
		}
		if _, ok := seen[entityID]; ok {
			continue
		}
		seen[entityID] = struct{}{}
		result = append(result, entityID)
	}
	return result
}

func ReadMessages(conn *websocket.Conn, messages chan<- []byte, errors chan<- error) {
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			errors <- fmt.Errorf("read Home Assistant websocket message: %w", err)
			return
		}
		messages <- payload
	}
}

func LogEvent(payload []byte) {
	var pretty map[string]any
	if err := json.Unmarshal(payload, &pretty); err != nil {
		slog.Info("event raw", "payload", string(payload))
		return
	}

	formatted, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		slog.Info("event raw", "payload", string(payload))
		return
	}

	slog.Info("event received", "payload", string(formatted))
}
