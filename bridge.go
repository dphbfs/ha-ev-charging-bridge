package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gorilla/websocket"
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
	if strings.TrimSpace(cfg.Charger.ChargerID) == "" {
		return errors.New("missing charger configuration: define at least one charger in config.yaml")
	}
	slog.Info("loaded charger", "charger", cfg.Charger.ChargerID, "power_entity_id", cfg.Charger.Entities.PowerW, "energy_entity_id", cfg.Charger.Entities.EnergyKWh)
	ctx := context.Background()
	store, err := persistence.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.SaveCharger(ctx, cfg.Charger); err != nil {
		return err
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

	sessionTracker := newSmartPlugHandler(cfg, cfg.Charger, store)
	if err := sessionTracker.initialize(states); err != nil {
		return err
	}

	if err := homeassistant.Subscribe(conn, 2, cfg.EventType); err != nil {
		return err
	}

	messages := make(chan []byte)
	readErrors := make(chan error, 1)
	go homeassistant.ReadMessages(conn, messages, readErrors)

	for {
		var timer <-chan time.Time
		if sessionTracker.endTimer != nil {
			timer = sessionTracker.endTimer.C
		}

		select {
		case payload := <-messages:
			homeassistant.LogEvent(payload)
			if err := sessionTracker.handleEvent(payload); err != nil {
				slog.Warn("session tracking failed", "error", err)
			}
		case err := <-readErrors:
			return err
		case <-timer:
			if err := sessionTracker.finishAfterDebounce(); err != nil {
				slog.Warn("session tracking failed", "error", err)
			}
		}
	}
}
