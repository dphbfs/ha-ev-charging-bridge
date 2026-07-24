package router

import (
	"context"
	"errors"
	"testing"
	"time"

	"ha-ev-charging-bridge/internal/homeassistant"
)

func TestRouteSendsMatchingEntityToExpectedChannel(t *testing.T) {
	r, err := New([]string{"sensor.ev_charger_power"}, 1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	routed, err := r.Route(context.Background(), eventForEntity("sensor.ev_charger_power"))
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if !routed {
		t.Fatal("Route() routed = false, want true")
	}

	select {
	case got := <-r.Channels()["sensor.ev_charger_power"]:
		if got.Event.Data.EntityID != "sensor.ev_charger_power" {
			t.Fatalf("entity_id = %q, want sensor.ev_charger_power", got.Event.Data.EntityID)
		}
	default:
		t.Fatal("matching event was not delivered")
	}
}

func TestRouteIgnoresUnrelatedEntities(t *testing.T) {
	r, err := New([]string{"sensor.ev_charger_power"}, 1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	routed, err := r.Route(context.Background(), eventForEntity("sensor.unrelated"))
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if routed {
		t.Fatal("Route() routed = true, want false")
	}

	select {
	case got := <-r.Channels()["sensor.ev_charger_power"]:
		t.Fatalf("unexpected event delivered: %#v", got)
	default:
	}
}

func TestRouteReportsMissingEntityID(t *testing.T) {
	r, err := New([]string{"sensor.ev_charger_power"}, 1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = r.Route(context.Background(), homeassistant.EventMessage{})
	if !errors.Is(err, ErrMissingEntityID) {
		t.Fatalf("Route() error = %v, want ErrMissingEntityID", err)
	}
}

func TestMultipleEntityChannelsCanBeConsumedWithSelect(t *testing.T) {
	r, err := New([]string{"sensor.ev_charger_power", "sensor.ev_charger_energy"}, 1)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if _, err := r.Route(context.Background(), eventForEntity("sensor.ev_charger_power")); err != nil {
		t.Fatalf("Route(power) error = %v", err)
	}
	if _, err := r.Route(context.Background(), eventForEntity("sensor.ev_charger_energy")); err != nil {
		t.Fatalf("Route(energy) error = %v", err)
	}

	channels := r.Channels()
	seen := map[string]bool{}
	deadline := time.After(time.Second)
	for len(seen) < 2 {
		select {
		case msg := <-channels["sensor.ev_charger_power"]:
			seen[msg.Event.Data.EntityID] = true
		case msg := <-channels["sensor.ev_charger_energy"]:
			seen[msg.Event.Data.EntityID] = true
		case <-deadline:
			t.Fatalf("timed out consuming channels; seen=%v", seen)
		}
	}
}

func eventForEntity(entityID string) homeassistant.EventMessage {
	var msg homeassistant.EventMessage
	msg.Event.TimeFired = time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	msg.Event.Data.EntityID = entityID
	msg.Event.Data.NewState = &homeassistant.State{EntityID: entityID, State: "1"}
	return msg
}
