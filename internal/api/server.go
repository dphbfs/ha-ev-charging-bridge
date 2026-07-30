package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	bridgeconfig "ha-ev-charging-bridge/internal/config"
	"ha-ev-charging-bridge/internal/events"
)

type Store interface {
	ActiveSessions(context.Context) ([]events.Session, error)
	CompletedSessions(context.Context) ([]events.Session, error)
	CompletedSession(context.Context, string) (events.Session, bool, error)
	MeterValues(context.Context, string) ([]events.MeterValue, error)
	SessionEvents(context.Context, string) ([]events.SessionEvent, error)
}

type Server struct {
	store    Store
	chargers map[string]bridgeconfig.Charger
}

func New(store Store, chargers []bridgeconfig.Charger) *Server {
	byID := make(map[string]bridgeconfig.Charger, len(chargers))
	for _, charger := range chargers {
		byID[charger.ChargerID] = charger
	}
	return &Server{store: store, chargers: byID}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/sessions", s.listSessions)
	mux.HandleFunc("GET /api/v1/sessions/active", s.activeSessions)
	mux.HandleFunc("GET /api/v1/sessions/{session_id}", s.sessionDetail)
	mux.HandleFunc("GET /api/v1/sessions/{session_id}/meter-values", s.meterValues)
	mux.HandleFunc("GET /api/v1/sessions/{session_id}/events", s.sessionEvents)
	return mux
}

type sessionDTO struct {
	ID                string     `json:"id"`
	Status            string     `json:"status"`
	State             string     `json:"state"`
	ChargerID         string     `json:"charger_id"`
	ChargerName       string     `json:"charger_name,omitempty"`
	EVSEID            string     `json:"evse_id"`
	ConnectorID       string     `json:"connector_id"`
	MeterID           string     `json:"meter_id"`
	StartedAt         time.Time  `json:"started_at"`
	EndedAt           *time.Time `json:"ended_at,omitempty"`
	DurationSeconds   *int64     `json:"duration_seconds,omitempty"`
	StartEnergyKWh    *float64   `json:"start_energy_kwh,omitempty"`
	EndEnergyKWh      *float64   `json:"end_energy_kwh,omitempty"`
	EnergyConsumedKWh *float64   `json:"energy_consumed_kwh,omitempty"`
}

type listResponse struct {
	Sessions []sessionDTO `json:"sessions"`
	Total    int          `json:"total"`
	Limit    int          `json:"limit"`
	Offset   int          `json:"offset"`
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := pagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sessions, err := s.store.CompletedSessions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load sessions")
		return
	}
	filtered, err := s.filterSessions(r, sessions)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sortSessions(r.URL.Query().Get("sort"), filtered)
	total := len(filtered)
	filtered = page(filtered, limit, offset)
	writeJSON(w, http.StatusOK, listResponse{Sessions: s.sessionDTOs(filtered), Total: total, Limit: limit, Offset: offset})
}

func (s *Server) activeSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.store.ActiveSessions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load active sessions")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": s.sessionDTOs(sessions)})
}

func (s *Server) sessionDetail(w http.ResponseWriter, r *http.Request) {
	session, ok, err := s.store.CompletedSession(r.Context(), r.PathValue("session_id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load session")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	meterValues, err := s.store.MeterValues(r.Context(), session.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load meter values")
		return
	}
	events, err := s.store.SessionEvents(r.Context(), session.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load session events")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": s.toDTO(session), "meter_values": meterValues, "events": events})
}

func (s *Server) meterValues(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.MeterValues(r.Context(), r.PathValue("session_id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load meter values")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"meter_values": values})
}

func (s *Server) sessionEvents(w http.ResponseWriter, r *http.Request) {
	events, err := s.store.SessionEvents(r.Context(), r.PathValue("session_id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load session events")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) filterSessions(r *http.Request, sessions []events.Session) ([]events.Session, error) {
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
	charger := strings.TrimSpace(r.URL.Query().Get("charger_id"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	startedAfter, err := timeParam(r, "started_after")
	if err != nil {
		return nil, err
	}
	startedBefore, err := timeParam(r, "started_before")
	if err != nil {
		return nil, err
	}
	filtered := sessions[:0]
	for _, session := range sessions {
		if startedAfter != nil && session.StartedAt.Before(*startedAfter) {
			continue
		}
		if startedBefore != nil && session.StartedAt.After(*startedBefore) {
			continue
		}
		if charger != "" && session.ChargerID != charger {
			continue
		}
		if status != "" && status != "all" && s.status(session) != status {
			continue
		}
		if query != "" && !s.matchesSearch(session, query) {
			continue
		}
		filtered = append(filtered, session)
	}
	return filtered, nil
}

func (s *Server) matchesSearch(session events.Session, query string) bool {
	charger := s.chargers[session.ChargerID]
	for _, value := range []string{session.ID, session.ChargerID, charger.ChargerName, session.ConnectorID, session.EVSEID} {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func (s *Server) sessionDTOs(sessions []events.Session) []sessionDTO {
	items := make([]sessionDTO, len(sessions))
	for i, session := range sessions {
		items[i] = s.toDTO(session)
	}
	return items
}

func (s *Server) toDTO(session events.Session) sessionDTO {
	charger := s.chargers[session.ChargerID]
	var duration *int64
	if session.EndedAt != nil {
		seconds := int64(session.EndedAt.Sub(session.StartedAt).Seconds())
		duration = &seconds
	}
	return sessionDTO{ID: session.ID, Status: s.status(session), State: string(session.State), ChargerID: session.ChargerID, ChargerName: charger.ChargerName, EVSEID: session.EVSEID, ConnectorID: session.ConnectorID, MeterID: session.MeterID, StartedAt: session.StartedAt, EndedAt: session.EndedAt, DurationSeconds: duration, StartEnergyKWh: session.StartEnergyKWh, EndEnergyKWh: session.EndEnergyKWh, EnergyConsumedKWh: session.EnergyConsumedKWh}
}

func (s *Server) status(session events.Session) string {
	if session.State == events.SessionCharging {
		return "charging"
	}
	return "completed"
}

func sortSessions(sortValue string, sessions []events.Session) {
	sort.SliceStable(sessions, func(i, j int) bool {
		if sortValue == "oldest" {
			return sessions[i].StartedAt.Before(sessions[j].StartedAt)
		}
		return sessions[i].StartedAt.After(sessions[j].StartedAt)
	})
}

func pagination(r *http.Request) (int, int, error) {
	limit, err := intParam(r, "limit", 50)
	if err != nil {
		return 0, 0, err
	}
	offset, err := intParam(r, "offset", 0)
	if err != nil {
		return 0, 0, err
	}
	if limit < 1 || limit > 200 {
		return 0, 0, errors.New("limit must be between 1 and 200")
	}
	if offset < 0 {
		return 0, 0, errors.New("offset must be greater than or equal to 0")
	}
	return limit, offset, nil
}

func intParam(r *http.Request, name string, fallback int) (int, error) {
	value := strings.TrimSpace(r.URL.Query().Get(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return parsed, nil
}

func timeParam(r *http.Request, name string) (*time.Time, error) {
	value := strings.TrimSpace(r.URL.Query().Get(name))
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("%s must be an RFC3339 timestamp", name)
	}
	return &parsed, nil
}

func page(sessions []events.Session, limit, offset int) []events.Session {
	if offset >= len(sessions) {
		return nil
	}
	end := offset + limit
	if end > len(sessions) {
		end = len(sessions)
	}
	return sessions[offset:end]
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
