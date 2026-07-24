package main

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	bridgeconfig "ha-ev-charging-bridge/internal/config"
	"ha-ev-charging-bridge/internal/homeassistant"
)

func run(cfg bridgeconfig.Runtime) error {
	if err := ensureParentDir(cfg.IngressStore); err != nil {
		return fmt.Errorf("prepare ingress event store: %w", err)
	}
	if err := ensureParentDir(cfg.ActiveStore); err != nil {
		return fmt.Errorf("prepare active session store: %w", err)
	}
	if err := ensureParentDir(cfg.SessionStore); err != nil {
		return fmt.Errorf("prepare session store: %w", err)
	}

	if strings.TrimSpace(cfg.HAURL) == "" {
		return errors.New("missing Home Assistant URL: set HA_URL or pass -ha-url")
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return errors.New("missing Home Assistant token: set HA_TOKEN or pass -token")
	}
	devices, err := loadDevices(cfg.DeviceConfig)
	if err != nil {
		return err
	}
	smartPlug, smartPlugCount, err := firstSmartPlug(devices)
	if err != nil {
		return err
	}
	if smartPlugCount > 1 {
		slog.Info("multiple smart plug devices found; using first for this POC", "count", smartPlugCount, "device", smartPlug.Name)
	}
	slog.Info("loaded smart plug device", "device", smartPlug.Name, "power_entity_id", smartPlug.EntityID, "energy_entity_id", smartPlug.EnergyEntityID)

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

	sessionTracker := newSmartPlugHandler(cfg, smartPlug)
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
			if err := appendJSONLine(cfg.IngressStore, payload); err != nil {
				slog.Warn("ingress event write failed", "error", err)
			}
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
