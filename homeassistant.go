package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type authRequiredMessage struct {
	Type string `json:"type"`
}

type authMessage struct {
	Type        string `json:"type"`
	AccessToken string `json:"access_token"`
}

type commandMessage struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	EventType string `json:"event_type,omitempty"`
}

type resultMessage struct {
	ID      int             `json:"id"`
	Type    string          `json:"type"`
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

type haState struct {
	EntityID    string          `json:"entity_id"`
	State       string          `json:"state"`
	LastChanged time.Time       `json:"last_changed"`
	Raw         json.RawMessage `json:"-"`
}

type haEventMessage struct {
	ID    int `json:"id"`
	Event struct {
		TimeFired time.Time `json:"time_fired"`
		Data      struct {
			EntityID string   `json:"entity_id"`
			NewState *haState `json:"new_state"`
			OldState *haState `json:"old_state"`
		} `json:"data"`
	} `json:"event"`
}

func websocketURL(raw string) (*url.URL, error) {
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

func authenticate(conn *websocket.Conn, token string) error {
	var authRequired authRequiredMessage
	if err := conn.ReadJSON(&authRequired); err != nil {
		return fmt.Errorf("read auth_required message: %w", err)
	}
	if authRequired.Type != "auth_required" {
		return fmt.Errorf("expected auth_required message, got %q", authRequired.Type)
	}

	if err := conn.WriteJSON(authMessage{Type: "auth", AccessToken: token}); err != nil {
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

func getStates(conn *websocket.Conn, id int) ([]haState, error) {
	if err := conn.WriteJSON(commandMessage{ID: id, Type: "get_states"}); err != nil {
		return nil, fmt.Errorf("send get_states command: %w", err)
	}

	var result resultMessage
	if err := conn.ReadJSON(&result); err != nil {
		return nil, fmt.Errorf("read get_states response: %w", err)
	}
	if result.Type != "result" || result.ID != id || !result.Success {
		if len(result.Error) > 0 {
			return nil, fmt.Errorf("get_states failed: %s", result.Error)
		}
		return nil, fmt.Errorf("get_states failed: type=%q id=%d success=%t", result.Type, result.ID, result.Success)
	}

	var states []haState
	if err := json.Unmarshal(result.Result, &states); err != nil {
		return nil, fmt.Errorf("parse get_states response: %w", err)
	}

	slog.Info("loaded Home Assistant states", "count", len(states))
	return states, nil
}

func subscribe(conn *websocket.Conn, id int, eventType string) error {
	cmd := commandMessage{ID: id, Type: "subscribe_events", EventType: strings.TrimSpace(eventType)}
	if err := conn.WriteJSON(cmd); err != nil {
		return fmt.Errorf("send subscribe_events command: %w", err)
	}

	var result resultMessage
	if err := conn.ReadJSON(&result); err != nil {
		return fmt.Errorf("read subscribe_events response: %w", err)
	}
	if result.Type != "result" || result.ID != cmd.ID || !result.Success {
		if len(result.Error) > 0 {
			return fmt.Errorf("subscribe_events failed: %s", result.Error)
		}
		return fmt.Errorf("subscribe_events failed: type=%q id=%d success=%t", result.Type, result.ID, result.Success)
	}

	if cmd.EventType == "" {
		slog.Info("subscribed to all Home Assistant events")
	} else {
		slog.Info("subscribed to Home Assistant event type", "event_type", cmd.EventType)
	}

	conn.SetReadLimit(10 * 1024 * 1024)
	return nil
}

func readMessages(conn *websocket.Conn, messages chan<- []byte, errors chan<- error) {
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			errors <- fmt.Errorf("read Home Assistant websocket message: %w", err)
			return
		}
		messages <- payload
	}
}

func logEvent(payload []byte) {
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
