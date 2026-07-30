package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"ha-ev-charging-bridge/internal/app"
	bridgeconfig "ha-ev-charging-bridge/internal/config"
	"ha-ev-charging-bridge/internal/homeassistant"
	"ha-ev-charging-bridge/internal/persistence"
)

func run(cfg bridgeconfig.Runtime) error {
	if err := ensureParentDir(cfg.DatabasePath); err != nil {
		return fmt.Errorf("prepare SQLite database: %w", err)
	}

	if strings.TrimSpace(cfg.HAURL) == "" {
		return errors.New("missing Home Assistant URL: set HA_URL or pass -ha-url")
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return errors.New("missing Home Assistant token: set HA_TOKEN or pass -token")
	}
	if len(cfg.Chargers) == 0 {
		return errors.New("missing charger configuration: define at least one charger in bridge.yaml")
	}
	ctx := context.Background()
	store, err := persistence.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer store.Close()
	pipelines := make([]*app.Pipeline, 0, len(cfg.Chargers))
	for _, charger := range cfg.Chargers {
		if strings.TrimSpace(charger.ChargerID) == "" {
			return errors.New("missing charger configuration: define charger_id in bridge.yaml")
		}
		slog.Info("loaded charger", "charger", charger.ChargerID, "power_entity_id", charger.Entities.PowerW, "energy_entity_id", charger.Entities.EnergyKWh)
		pipeline, err := app.NewPipeline(ctx, charger, store)
		if err != nil {
			return err
		}
		pipelines = append(pipelines, pipeline)
	}

	wsURL, err := homeassistant.WebsocketURL(cfg.HAURL)
	if err != nil {
		return err
	}

	slog.Info("connecting to Home Assistant websocket", "url", wsURL.Redacted())
	conn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		return fmt.Errorf("connect to Home Assistant websocket: %w", err)
	}
	defer conn.Close()

	if err := homeassistant.Authenticate(conn, cfg.Token); err != nil {
		return err
	}

	states, err := homeassistant.GetStates(conn, 1)
	if err != nil {
		return err
	}
	for _, pipeline := range pipelines {
		if err := pipeline.InitializeStates(ctx, states); err != nil {
			return err
		}
	}

	if err := homeassistant.Subscribe(conn, 2, cfg.EventType); err != nil {
		return err
	}

	messages := make(chan []byte)
	readErrors := make(chan error, 1)
	go homeassistant.ReadMessages(conn, messages, readErrors)

	for {
		var timer <-chan time.Time
		if deadline, ok := nextDeadline(pipelines); ok {
			delay := time.Until(deadline)
			if delay <= 0 {
				for _, pipeline := range pipelines {
					if _, err := pipeline.Advance(ctx, time.Now()); err != nil {
						slog.Warn("v1 event processing failed", "error", err)
					}
				}
				continue
			}
			timer = time.After(delay)
		}

		select {
		case payload := <-messages:
			homeassistant.LogEvent(payload)
			for _, pipeline := range pipelines {
				if _, err := pipeline.ProcessPayload(ctx, payload); err != nil {
					slog.Warn("v1 event processing failed", "error", err)
				}
			}
		case err := <-readErrors:
			return err
		case at := <-timer:
			for _, pipeline := range pipelines {
				if _, err := pipeline.Advance(ctx, at); err != nil {
					slog.Warn("v1 event processing failed", "error", err)
				}
			}
		}
	}
}

func nextDeadline(pipelines []*app.Pipeline) (time.Time, bool) {
	var next time.Time
	for _, pipeline := range pipelines {
		deadline, ok := pipeline.Deadline()
		if !ok {
			continue
		}
		if next.IsZero() || deadline.Before(next) {
			next = deadline
		}
	}
	return next, !next.IsZero()
}
