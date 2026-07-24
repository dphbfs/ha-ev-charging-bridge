package router

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"ha-ev-charging-bridge/internal/homeassistant"
)

var ErrMissingEntityID = errors.New("home assistant event missing entity_id")

type Router struct {
	channels map[string]chan homeassistant.EventMessage
}

func New(entityIDs []string, buffer int) (*Router, error) {
	if buffer < 0 {
		return nil, errors.New("buffer must be non-negative")
	}

	channels := make(map[string]chan homeassistant.EventMessage, len(entityIDs))
	for _, entityID := range entityIDs {
		entityID = strings.TrimSpace(entityID)
		if entityID == "" {
			continue
		}
		if _, exists := channels[entityID]; exists {
			return nil, fmt.Errorf("duplicate entity_id %q", entityID)
		}
		channels[entityID] = make(chan homeassistant.EventMessage, buffer)
	}

	return &Router{channels: channels}, nil
}

func (r *Router) Channels() map[string]<-chan homeassistant.EventMessage {
	channels := make(map[string]<-chan homeassistant.EventMessage, len(r.channels))
	for entityID, ch := range r.channels {
		channels[entityID] = ch
	}
	return channels
}

func (r *Router) Route(ctx context.Context, msg homeassistant.EventMessage) (bool, error) {
	entityID := strings.TrimSpace(msg.Event.Data.EntityID)
	if entityID == "" && msg.Event.Data.NewState != nil {
		entityID = strings.TrimSpace(msg.Event.Data.NewState.EntityID)
	}
	if entityID == "" {
		return false, ErrMissingEntityID
	}

	ch, ok := r.channels[entityID]
	if !ok {
		return false, nil
	}

	select {
	case ch <- msg:
		return true, nil
	case <-ctx.Done():
		return false, fmt.Errorf("route event for %q: %w", entityID, ctx.Err())
	}
}
