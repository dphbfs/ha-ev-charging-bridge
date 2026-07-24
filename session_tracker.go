package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	bridgeconfig "ha-ev-charging-bridge/internal/config"
	"ha-ev-charging-bridge/internal/events"
	"ha-ev-charging-bridge/internal/homeassistant"
	"ha-ev-charging-bridge/internal/persistence"
)

type session struct {
	ID                string     `json:"id"`
	DeviceName        string     `json:"device_name"`
	DeviceType        string     `json:"device_type"`
	PowerEntityID     string     `json:"power_entity_id"`
	EnergyEntityID    string     `json:"energy_entity_id"`
	Status            string     `json:"status"`
	StartedAt         time.Time  `json:"started_at"`
	EndedAt           *time.Time `json:"ended_at,omitempty"`
	StartEnergyKWh    float64    `json:"start_energy_kwh"`
	EndEnergyKWh      float64    `json:"end_energy_kwh"`
	EnergyConsumedKWh float64    `json:"energy_consumed_kwh"`
	StartPowerW       float64    `json:"start_power_w"`
	CurrentPowerW     float64    `json:"current_power_w"`
	EndPowerW         float64    `json:"end_power_w"`
}

type smartPlugHandler struct {
	cfg              bridgeconfig.Runtime
	charger          bridgeconfig.Charger
	store            *persistence.Store
	active           *session
	latestEnergyKWh  *float64
	latestPowerW     *float64
	belowThresholdAt *time.Time
	endTimer         *time.Timer
}

func newSmartPlugHandler(cfg bridgeconfig.Runtime, charger bridgeconfig.Charger, store *persistence.Store) *smartPlugHandler {
	return &smartPlugHandler{cfg: cfg, charger: charger, store: store}
}

func (h *smartPlugHandler) initialize(states []homeassistant.State) error {
	for _, state := range states {
		switch state.EntityID {
		case h.charger.Entities.EnergyKWh:
			if value, err := parseStateFloat(state.State); err == nil {
				h.latestEnergyKWh = &value
				slog.Info("initial energy loaded", "entity_id", h.charger.Entities.EnergyKWh, "energy_kwh", value)
			}
		case h.charger.Entities.PowerW:
			if value, err := parseStateFloat(state.State); err == nil {
				h.latestPowerW = &value
				slog.Info("initial power loaded", "entity_id", h.charger.Entities.PowerW, "power_w", value)
			}
		}
	}
	slog.Info("smart plug handler initialized", "charger", h.charger.ChargerID, "power_entity_id", h.charger.Entities.PowerW, "energy_entity_id", h.charger.Entities.EnergyKWh, "database", h.cfg.DatabasePath)

	if err := h.loadActive(); err != nil {
		return err
	}
	if h.active != nil && h.latestPowerW != nil {
		return h.handlePower(*h.latestPowerW, time.Now())
	}
	return nil
}

func (h *smartPlugHandler) loadActive() error {
	if h.store == nil {
		return nil
	}

	active, ok, err := h.store.ActiveSession(context.Background(), h.charger.ChargerID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	legacy := legacySessionFromEventSession(active, h.charger)
	h.active = &legacy
	slog.Info("resumed active session", "session_id", active.ID)
	return nil
}

func (h *smartPlugHandler) handleEvent(payload []byte) error {
	var msg homeassistant.EventMessage
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&msg); err != nil {
		if err := json.Unmarshal(payload, &msg); err != nil {
			return nil
		}
	}

	entityID := msg.Event.Data.EntityID
	if msg.Event.Data.NewState == nil {
		return nil
	}

	value, err := parseStateFloat(msg.Event.Data.NewState.State)
	if err != nil {
		return nil
	}

	switch entityID {
	case h.charger.Entities.EnergyKWh:
		h.latestEnergyKWh = &value
		if h.active != nil {
			if err := h.writeMeterValue(entityID, "kWh", value, eventTime(msg.Event.TimeFired)); err != nil {
				return err
			}
		}
		return h.writeActive()
	case h.charger.Entities.PowerW:
		h.latestPowerW = &value
		return h.handlePower(value, eventTime(msg.Event.TimeFired))
	}

	return nil
}

func (h *smartPlugHandler) handlePower(powerW float64, at time.Time) error {
	if h.active == nil && powerW > h.cfg.StartThresholdW {
		if h.latestEnergyKWh == nil {
			slog.Warn("power is above start threshold but no energy reading is available yet", "power_w", powerW)
			return nil
		}

		h.active = &session{
			ID:             fmt.Sprintf("session-%s", at.UTC().Format("20060102T150405Z")),
			DeviceName:     h.charger.ChargerID,
			DeviceType:     "charger",
			PowerEntityID:  h.charger.Entities.PowerW,
			EnergyEntityID: h.charger.Entities.EnergyKWh,
			Status:         "in_progress",
			StartedAt:      at,
			StartEnergyKWh: *h.latestEnergyKWh,
			StartPowerW:    powerW,
			CurrentPowerW:  powerW,
		}
		if err := h.writeActive(); err != nil {
			return err
		}
		if h.store != nil {
			if err := h.store.SaveSessionEvent(context.Background(), events.SessionEvent{
				Type:       events.SessionStarted,
				SessionID:  h.active.ID,
				ChargerID:  h.active.DeviceName,
				OccurredAt: at,
			}); err != nil {
				return err
			}
		}
		slog.Info("session started", "session_id", h.active.ID, "charger", h.charger.ChargerID, "power_w", powerW, "start_energy_kwh", h.active.StartEnergyKWh)
	}

	if h.active == nil {
		return nil
	}
	h.active.CurrentPowerW = powerW
	if err := h.writeMeterValue(h.charger.Entities.PowerW, "W", powerW, at); err != nil {
		return err
	}
	if h.latestEnergyKWh != nil {
		h.active.EndEnergyKWh = *h.latestEnergyKWh
		h.active.EnergyConsumedKWh = h.active.EndEnergyKWh - h.active.StartEnergyKWh
	}
	if err := h.writeActive(); err != nil {
		return err
	}

	if powerW < h.cfg.EndThresholdW {
		if h.belowThresholdAt == nil {
			h.belowThresholdAt = &at
			h.endTimer = time.NewTimer(h.cfg.EndDebounce)
			slog.Info("power below end threshold", "power_w", powerW, "threshold_w", h.cfg.EndThresholdW, "debounce", h.cfg.EndDebounce)
		}
		return nil
	}

	if h.belowThresholdAt != nil {
		slog.Info("power rose above end threshold; keeping session active", "power_w", powerW, "threshold_w", h.cfg.EndThresholdW)
		h.belowThresholdAt = nil
		if h.endTimer != nil {
			h.endTimer.Stop()
			h.endTimer = nil
		}
	}

	return nil
}

func (h *smartPlugHandler) finishAfterDebounce() error {
	h.endTimer = nil
	if h.active == nil || h.belowThresholdAt == nil || h.latestPowerW == nil || *h.latestPowerW >= h.cfg.EndThresholdW {
		h.belowThresholdAt = nil
		return nil
	}
	if h.latestEnergyKWh == nil {
		return errors.New("cannot finish session: no energy reading is available")
	}

	endedAt := *h.belowThresholdAt
	h.active.EndedAt = &endedAt
	h.active.Status = "completed"
	h.active.EndPowerW = *h.latestPowerW
	h.active.CurrentPowerW = *h.latestPowerW
	h.active.EndEnergyKWh = *h.latestEnergyKWh
	h.active.EnergyConsumedKWh = h.active.EndEnergyKWh - h.active.StartEnergyKWh

	completed := *h.active
	if h.store == nil {
		return errors.New("cannot finish session: SQLite store is not configured")
	}
	if err := h.store.SaveCompletedSession(context.Background(), eventSessionFromLegacy(completed, h.charger, events.SessionEndedState)); err != nil {
		return err
	}
	if err := h.store.SaveSessionEvent(context.Background(), events.SessionEvent{
		Type:       events.SessionEnded,
		SessionID:  completed.ID,
		ChargerID:  completed.DeviceName,
		OccurredAt: endedAt,
		Reason:     string(events.ChargingStopped),
	}); err != nil {
		return err
	}

	slog.Info("session ended", "session_id", completed.ID, "duration", completed.EndedAt.Sub(completed.StartedAt), "energy_kwh", completed.EnergyConsumedKWh)
	h.active = nil
	h.belowThresholdAt = nil
	return nil
}

func (h *smartPlugHandler) writeActive() error {
	if h.active == nil {
		return nil
	}
	if h.latestEnergyKWh != nil {
		h.active.EndEnergyKWh = *h.latestEnergyKWh
		h.active.EnergyConsumedKWh = h.active.EndEnergyKWh - h.active.StartEnergyKWh
	}
	if h.store == nil {
		return nil
	}
	if err := h.store.SaveActiveSession(context.Background(), eventSessionFromLegacy(*h.active, h.charger, events.SessionCharging)); err != nil {
		return err
	}
	return h.store.SaveSessionEvent(context.Background(), events.SessionEvent{
		Type:       events.SessionUpdated,
		SessionID:  h.active.ID,
		ChargerID:  h.active.DeviceName,
		OccurredAt: time.Now(),
	})
}

func (h *smartPlugHandler) writeMeterValue(entityID, unit string, value float64, at time.Time) error {
	if h.store == nil || h.active == nil {
		return nil
	}
	return h.store.SaveMeterValue(context.Background(), events.MeterValue{
		SessionID:  h.active.ID,
		ChargerID:  h.active.DeviceName,
		MeterID:    h.charger.MeterID,
		EntityID:   entityID,
		Unit:       unit,
		Value:      value,
		ObservedAt: at,
	})
}

func eventSessionFromLegacy(value session, charger bridgeconfig.Charger, state events.SessionState) events.Session {
	startEnergy := value.StartEnergyKWh
	endEnergy := value.EndEnergyKWh
	consumed := value.EnergyConsumedKWh
	return events.Session{
		ID:                value.ID,
		ChargerID:         value.DeviceName,
		EVSEID:            charger.EVSEID,
		ConnectorID:       charger.ConnectorID,
		MeterID:           charger.MeterID,
		State:             state,
		StartedAt:         value.StartedAt,
		EndedAt:           value.EndedAt,
		StartEnergyKWh:    &startEnergy,
		EndEnergyKWh:      &endEnergy,
		EnergyConsumedKWh: &consumed,
	}
}

func legacySessionFromEventSession(value events.Session, charger bridgeconfig.Charger) session {
	legacy := session{
		ID:             value.ID,
		DeviceName:     charger.ChargerID,
		DeviceType:     "charger",
		PowerEntityID:  charger.Entities.PowerW,
		EnergyEntityID: charger.Entities.EnergyKWh,
		Status:         "in_progress",
		StartedAt:      value.StartedAt,
		EndedAt:        value.EndedAt,
	}
	if value.StartEnergyKWh != nil {
		legacy.StartEnergyKWh = *value.StartEnergyKWh
	}
	if value.EndEnergyKWh != nil {
		legacy.EndEnergyKWh = *value.EndEnergyKWh
	}
	if value.EnergyConsumedKWh != nil {
		legacy.EnergyConsumedKWh = *value.EnergyConsumedKWh
	}
	return legacy
}

func parseStateFloat(raw string) (float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" || value == "unknown" || value == "unavailable" {
		return 0, fmt.Errorf("state is not numeric: %q", raw)
	}
	return strconv.ParseFloat(value, 64)
}

func eventTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now()
	}
	return value
}
