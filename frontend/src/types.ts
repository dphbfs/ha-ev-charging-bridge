export type SessionStatus = "charging" | "completed" | "stopped" | "interrupted" | "unknown";

export interface MeterSample {
  timestamp: string;
  powerKw: number;
  energyKwh: number;
}

export interface TimelineEvent {
  timestamp: string;
  kind: "started" | "meter_updates" | "stopped" | "completed";
  label: string;
  reason?: string;
  sampleCount?: number;
}

export interface ChargingSession {
  id: string;
  shortId: string;
  status: SessionStatus;
  chargerName: string;
  chargerId: string;
  connectorName: string;
  connectorId: string;
  evseId: string;
  meterId: string;
  startedAt: string;
  endedAt?: string;
  duration: string;
  elapsed?: string;
  powerKw?: number;
  energyKwh: number;
  startReadingKwh: number;
  endReadingKwh?: number;
  samples: MeterSample[];
  events: TimelineEvent[];
}
