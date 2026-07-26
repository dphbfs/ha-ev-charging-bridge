package events

import "time"

type ChargerEventType string

const (
	ChargerAvailable   ChargerEventType = "charger_available"
	ChargerUnavailable ChargerEventType = "charger_unavailable"
	ChargingStarted    ChargerEventType = "charging_started"
	ChargingStopped    ChargerEventType = "charging_stopped"
	MeterValueObserved ChargerEventType = "meter_value"
)

type SessionEventType string

const (
	SessionStarted SessionEventType = "session_started"
	SessionUpdated SessionEventType = "session_updated"
	SessionEnded   SessionEventType = "session_ended"
)

type ChargerEvent struct {
	Type        ChargerEventType
	ChargerID   string
	EVSEID      string
	ConnectorID string
	MeterID     string
	EntityID    string
	OccurredAt  time.Time
	PowerW      *float64
	EnergyKWh   *float64
	Reason      string
}

type SessionEvent struct {
	Type       SessionEventType
	SessionID  string
	ChargerID  string
	OccurredAt time.Time
	Reason     string
}

type SessionState string

const (
	SessionCharging   SessionState = "charging"
	SessionEndedState SessionState = "ended"
)

type Session struct {
	ID                string
	ChargerID         string
	EVSEID            string
	ConnectorID       string
	MeterID           string
	State             SessionState
	StartedAt         time.Time
	EndedAt           *time.Time
	StartEnergyKWh    *float64
	EndEnergyKWh      *float64
	EnergyConsumedKWh *float64
}

type MeterValue struct {
	SessionID  string
	ChargerID  string
	MeterID    string
	EntityID   string
	Unit       string
	Value      float64
	ObservedAt time.Time
}
