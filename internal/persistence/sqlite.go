package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	bridgeconfig "ha-ev-charging-bridge/internal/config"
	"ha-ev-charging-bridge/internal/events"
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &Store{db: db}
	if err := store.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) SaveCharger(ctx context.Context, charger bridgeconfig.Charger) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO chargers (charger_id, evse_id, connector_id, meter_id)
VALUES (?, ?, ?, ?)
ON CONFLICT(charger_id) DO UPDATE SET
  evse_id = excluded.evse_id,
  connector_id = excluded.connector_id,
  meter_id = excluded.meter_id`, charger.ChargerID, charger.EVSEID, charger.ConnectorID, charger.MeterID)
	if err != nil {
		return fmt.Errorf("save charger %q: %w", charger.ChargerID, err)
	}
	return nil
}

func (s *Store) Charger(ctx context.Context, chargerID string) (bridgeconfig.Charger, bool, error) {
	var charger bridgeconfig.Charger
	err := s.db.QueryRowContext(ctx, `
SELECT charger_id, evse_id, connector_id, meter_id
FROM chargers
WHERE charger_id = ?`, chargerID).Scan(&charger.ChargerID, &charger.EVSEID, &charger.ConnectorID, &charger.MeterID)
	if errors.Is(err, sql.ErrNoRows) {
		return bridgeconfig.Charger{}, false, nil
	}
	if err != nil {
		return bridgeconfig.Charger{}, false, fmt.Errorf("load charger %q: %w", chargerID, err)
	}
	return charger, true, nil
}

func (s *Store) SaveActiveSession(ctx context.Context, session events.Session) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO active_sessions (session_id, charger_id, evse_id, connector_id, meter_id, state, started_at, start_energy_kwh, end_energy_kwh, energy_consumed_kwh)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(charger_id) DO UPDATE SET
  session_id = excluded.session_id,
  evse_id = excluded.evse_id,
  connector_id = excluded.connector_id,
  meter_id = excluded.meter_id,
  state = excluded.state,
  started_at = excluded.started_at,
  start_energy_kwh = excluded.start_energy_kwh,
  end_energy_kwh = excluded.end_energy_kwh,
  energy_consumed_kwh = excluded.energy_consumed_kwh`,
		session.ID, session.ChargerID, session.EVSEID, session.ConnectorID, session.MeterID, session.State, formatTime(session.StartedAt), nullableFloat(session.StartEnergyKWh), nullableFloat(session.EndEnergyKWh), nullableFloat(session.EnergyConsumedKWh))
	if err != nil {
		return fmt.Errorf("save active session %q: %w", session.ID, err)
	}
	return nil
}

func (s *Store) ActiveSession(ctx context.Context, chargerID string) (events.Session, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT session_id, charger_id, evse_id, connector_id, meter_id, state, started_at, start_energy_kwh, end_energy_kwh, energy_consumed_kwh
FROM active_sessions
WHERE charger_id = ?`, chargerID)
	session, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return events.Session{}, false, nil
	}
	if err != nil {
		return events.Session{}, false, fmt.Errorf("load active session for charger %q: %w", chargerID, err)
	}
	return session, true, nil
}

func (s *Store) ActiveSessions(ctx context.Context) ([]events.Session, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT session_id, charger_id, evse_id, connector_id, meter_id, state, started_at, start_energy_kwh, end_energy_kwh, energy_consumed_kwh
FROM active_sessions
ORDER BY started_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query active sessions: %w", err)
	}
	defer rows.Close()

	var sessions []events.Session
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scan active session: %w", err)
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active sessions: %w", err)
	}
	return sessions, nil
}

func (s *Store) SaveCompletedSession(ctx context.Context, session events.Session) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO completed_sessions (session_id, charger_id, evse_id, connector_id, meter_id, state, started_at, ended_at, start_energy_kwh, end_energy_kwh, energy_consumed_kwh)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(session_id) DO UPDATE SET
  state = excluded.state,
  ended_at = excluded.ended_at,
  end_energy_kwh = excluded.end_energy_kwh,
  energy_consumed_kwh = excluded.energy_consumed_kwh`,
		session.ID, session.ChargerID, session.EVSEID, session.ConnectorID, session.MeterID, session.State, formatTime(session.StartedAt), nullableTime(session.EndedAt), nullableFloat(session.StartEnergyKWh), nullableFloat(session.EndEnergyKWh), nullableFloat(session.EnergyConsumedKWh))
	if err != nil {
		return fmt.Errorf("save completed session %q: %w", session.ID, err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM active_sessions WHERE charger_id = ?`, session.ChargerID); err != nil {
		return fmt.Errorf("delete active session for charger %q: %w", session.ChargerID, err)
	}
	return nil
}

func (s *Store) CompletedSession(ctx context.Context, sessionID string) (events.Session, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT session_id, charger_id, evse_id, connector_id, meter_id, state, started_at, ended_at, start_energy_kwh, end_energy_kwh, energy_consumed_kwh
FROM completed_sessions
WHERE session_id = ?`, sessionID)
	session, err := scanCompletedSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return events.Session{}, false, nil
	}
	if err != nil {
		return events.Session{}, false, fmt.Errorf("load completed session %q: %w", sessionID, err)
	}
	return session, true, nil
}

func (s *Store) CompletedSessions(ctx context.Context) ([]events.Session, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT session_id, charger_id, evse_id, connector_id, meter_id, state, started_at, ended_at, start_energy_kwh, end_energy_kwh, energy_consumed_kwh
FROM completed_sessions
ORDER BY started_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query completed sessions: %w", err)
	}
	defer rows.Close()

	var sessions []events.Session
	for rows.Next() {
		session, err := scanCompletedSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scan completed session: %w", err)
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate completed sessions: %w", err)
	}
	return sessions, nil
}

func scanCompletedSession(row scanner) (events.Session, error) {
	var session events.Session
	var startedAt string
	var endedAt sql.NullString
	var startEnergy, endEnergy, consumed sql.NullFloat64
	if err := row.Scan(&session.ID, &session.ChargerID, &session.EVSEID, &session.ConnectorID, &session.MeterID, &session.State, &startedAt, &endedAt, &startEnergy, &endEnergy, &consumed); err != nil {
		return events.Session{}, err
	}
	parsedStartedAt, err := parseTime(startedAt)
	if err != nil {
		return events.Session{}, err
	}
	session.StartedAt = parsedStartedAt
	if endedAt.Valid {
		parsedEndedAt, err := parseTime(endedAt.String)
		if err != nil {
			return events.Session{}, err
		}
		session.EndedAt = &parsedEndedAt
	}
	session.StartEnergyKWh = floatFromNull(startEnergy)
	session.EndEnergyKWh = floatFromNull(endEnergy)
	session.EnergyConsumedKWh = floatFromNull(consumed)
	return session, nil
}

func (s *Store) SaveMeterValue(ctx context.Context, value events.MeterValue) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO meter_values (session_id, charger_id, meter_id, entity_id, unit, value, observed_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`, value.SessionID, value.ChargerID, value.MeterID, value.EntityID, value.Unit, value.Value, formatTime(value.ObservedAt))
	if err != nil {
		return fmt.Errorf("save meter value for charger %q: %w", value.ChargerID, err)
	}
	return nil
}

func (s *Store) MeterValues(ctx context.Context, sessionID string) ([]events.MeterValue, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT session_id, charger_id, meter_id, entity_id, unit, value, observed_at
FROM meter_values
WHERE session_id = ?
ORDER BY observed_at`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query meter values for session %q: %w", sessionID, err)
	}
	defer rows.Close()

	var values []events.MeterValue
	for rows.Next() {
		var value events.MeterValue
		var observedAt string
		if err := rows.Scan(&value.SessionID, &value.ChargerID, &value.MeterID, &value.EntityID, &value.Unit, &value.Value, &observedAt); err != nil {
			return nil, fmt.Errorf("scan meter value: %w", err)
		}
		parsed, err := parseTime(observedAt)
		if err != nil {
			return nil, err
		}
		value.ObservedAt = parsed
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate meter values: %w", err)
	}
	return values, nil
}

func (s *Store) SaveSessionEvent(ctx context.Context, event events.SessionEvent) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO session_events (session_id, charger_id, type, occurred_at, reason)
VALUES (?, ?, ?, ?, ?)`, event.SessionID, event.ChargerID, event.Type, formatTime(event.OccurredAt), event.Reason)
	if err != nil {
		return fmt.Errorf("save session event %q: %w", event.Type, err)
	}
	return nil
}

func (s *Store) SessionEvents(ctx context.Context, sessionID string) ([]events.SessionEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT session_id, charger_id, type, occurred_at, reason
FROM session_events
WHERE session_id = ?
ORDER BY occurred_at`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query session events for session %q: %w", sessionID, err)
	}
	defer rows.Close()

	var values []events.SessionEvent
	for rows.Next() {
		var value events.SessionEvent
		var occurredAt string
		if err := rows.Scan(&value.SessionID, &value.ChargerID, &value.Type, &occurredAt, &value.Reason); err != nil {
			return nil, fmt.Errorf("scan session event: %w", err)
		}
		parsed, err := parseTime(occurredAt)
		if err != nil {
			return nil, err
		}
		value.OccurredAt = parsed
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session events: %w", err)
	}
	return values, nil
}

func (s *Store) DeleteMeterValuesBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM meter_values WHERE observed_at < ?`, formatTime(cutoff))
	if err != nil {
		return 0, fmt.Errorf("delete old meter values: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted meter values: %w", err)
	}
	return deleted, nil
}

func (s *Store) DeleteSessionEventsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM session_events WHERE occurred_at < ?`, formatTime(cutoff))
	if err != nil {
		return 0, fmt.Errorf("delete old session events: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted session events: %w", err)
	}
	return deleted, nil
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("migrate sqlite database: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSession(row scanner) (events.Session, error) {
	var session events.Session
	var startedAt string
	var startEnergy, endEnergy, consumed sql.NullFloat64
	if err := row.Scan(&session.ID, &session.ChargerID, &session.EVSEID, &session.ConnectorID, &session.MeterID, &session.State, &startedAt, &startEnergy, &endEnergy, &consumed); err != nil {
		return events.Session{}, err
	}
	parsed, err := parseTime(startedAt)
	if err != nil {
		return events.Session{}, err
	}
	session.StartedAt = parsed
	session.StartEnergyKWh = floatFromNull(startEnergy)
	session.EndEnergyKWh = floatFromNull(endEnergy)
	session.EnergyConsumedKWh = floatFromNull(consumed)
	return session, nil
}

func nullableFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

func floatFromNull(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	copy := value.Float64
	return &copy
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse persisted time %q: %w", value, err)
	}
	return parsed, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS chargers (
  charger_id TEXT PRIMARY KEY,
  evse_id TEXT NOT NULL,
  connector_id TEXT NOT NULL,
  meter_id TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS active_sessions (
  charger_id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  evse_id TEXT NOT NULL,
  connector_id TEXT NOT NULL,
  meter_id TEXT NOT NULL,
  state TEXT NOT NULL,
  started_at TEXT NOT NULL,
  start_energy_kwh REAL,
  end_energy_kwh REAL,
  energy_consumed_kwh REAL
);

CREATE TABLE IF NOT EXISTS completed_sessions (
  session_id TEXT PRIMARY KEY,
  charger_id TEXT NOT NULL,
  evse_id TEXT NOT NULL,
  connector_id TEXT NOT NULL,
  meter_id TEXT NOT NULL,
  state TEXT NOT NULL,
  started_at TEXT NOT NULL,
  ended_at TEXT,
  start_energy_kwh REAL,
  end_energy_kwh REAL,
  energy_consumed_kwh REAL
);

CREATE TABLE IF NOT EXISTS meter_values (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id TEXT,
  charger_id TEXT NOT NULL,
  meter_id TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  unit TEXT NOT NULL,
  value REAL NOT NULL,
  observed_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS session_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id TEXT NOT NULL,
  charger_id TEXT NOT NULL,
  type TEXT NOT NULL,
  occurred_at TEXT NOT NULL,
  reason TEXT
);
`
