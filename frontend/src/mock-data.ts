import type { ChargingSession, MeterSample, TimelineEvent } from "./types";

function samples(points: number[], energyTotal: number): MeterSample[] {
  return points.map((powerKw, index) => ({
    timestamp: new Date(Date.UTC(2024, 4, 19, 14, 30 + index * 10)).toISOString(),
    powerKw,
    energyKwh: Number(((index / Math.max(points.length - 1, 1)) * energyTotal).toFixed(2)),
  }));
}

export const mockSessions: ChargingSession[] = [
  {
    id: "sess_active_20240519_wallbox_1_connector_1_long_dev_id",
    shortId: "sess_active",
    status: "charging",
    chargerName: "Charger 1",
    chargerId: "wallbox-1",
    connectorName: "Connector 1",
    connectorId: "conn-1",
    evseId: "evse-001",
    meterId: "meter-001",
    startedAt: "May 19, 2024 9:15 AM",
    duration: "2h 09m",
    elapsed: "2h 09m",
    powerKw: 6.32,
    energyKwh: 12.47,
    startReadingKwh: 2441.1,
    samples: samples([0, 6.1, 6.3, 6.0, 6.7, 6.6, 5.8, 5.5, 6.0, 6.2, 6.4, 6.1, 6.7, 6.3, 6.0, 6.1, 5.9], 12.47),
    events: [{ timestamp: "9:15 AM", kind: "started", label: "Session started" }],
  },
  ...[
    ["sess_9f3a2e-7c4a8b", "completed", "May 19, 2024 2:30 PM", "May 19, 2024 5:28 PM", "2h 58m", "Charger 1", "wallbox-1", "Connector 1", "conn-1", 12.47],
    ["sess_8ab21e-9941aa", "completed", "May 18, 2024 6:42 PM", "May 18, 2024 8:00 PM", "1h 18m", "Charger 1", "wallbox-1", "Connector 1", "conn-1", 7.98],
    ["sess_stop_20240518", "stopped", "May 18, 2024 2:14 PM", "May 18, 2024 2:38 PM", "0h 24m", "Charger 2", "wallbox-2", "Connector 1", "conn-1", 1.87],
    ["sess_complete_20240517", "completed", "May 17, 2024 8:05 PM", "May 17, 2024 11:47 PM", "3h 42m", "Charger 1", "wallbox-1", "Connector 1", "conn-1", 14.97],
    ["sess_interrupted_20240517", "interrupted", "May 17, 2024 11:11 AM", "May 17, 2024 11:21 AM", "0h 10m", "Charger 2", "wallbox-2", "Connector 1", "conn-1", 0.62],
    ["sess_unknown_20240516", "unknown", "May 16, 2024 7:08 AM", "May 16, 2024 7:56 AM", "0h 48m", "Charger 2", "wallbox-2", "Connector 1", "conn-1", 3.21],
    ["sess_long_identifier_for_text_overflow_validation_20240517", "completed", "May 17, 2024 5:46 PM", "May 17, 2024 8:13 PM", "2h 27m", "Charger With A Long Display Name", "wallbox-long-identifier-001", "Long Connector Name", "connector-long-01", 9.34],
  ].map(([id, status, startedAt, endedAt, duration, chargerName, chargerId, connectorName, connectorId, energyKwh]) => ({
    id: String(id),
    shortId: String(id).slice(0, 18),
    status: status as ChargingSession["status"],
    chargerName: String(chargerName),
    chargerId: String(chargerId),
    connectorName: String(connectorName),
    connectorId: String(connectorId),
    evseId: "evse-001",
    meterId: "meter-001",
    startedAt: String(startedAt),
    endedAt: String(endedAt),
    duration: String(duration),
    energyKwh: Number(energyKwh),
    startReadingKwh: 2441.1,
    endReadingKwh: 2441.1 + Number(energyKwh),
    samples: samples([0, 7.8, 8.7, 8.3, 8.5, 8.1, 6.4, 6.8, 7.9, 8.0, 7.3, 7.5, 7.2, 6.8, 6.1, 5.9, 4.2, 2.8, 0], Number(energyKwh)),
    events: [
      { timestamp: "2:30 PM", kind: "started", label: "Session started" },
      { timestamp: "2:30 PM - 5:28 PM", kind: "meter_updates", label: "meter updates", sampleCount: 214 },
      { timestamp: "5:28 PM", kind: "stopped", label: "Charging stopped", reason: status === "interrupted" ? "charger_unavailable" : "charging_stopped" },
      { timestamp: "5:28 PM", kind: "completed", label: "Session completed" },
    ] satisfies TimelineEvent[],
  })),
];

export const activeSession = mockSessions[0];
export const historySessions = mockSessions.slice(1);
