import { activeSession, historySessions } from "./mock-data";
import type { ChargingSession, MeterSample, SessionStatus, TimelineEvent } from "./types";

interface BackendSession {
  id: string;
  status: string;
  charger_id: string;
  charger_name?: string;
  evse_id: string;
  connector_id: string;
  meter_id: string;
  started_at: string;
  ended_at?: string;
  duration_seconds?: number;
  start_energy_kwh?: number;
  end_energy_kwh?: number;
  energy_consumed_kwh?: number;
}

interface BackendMeterValue {
  Unit?: string;
  Value?: number;
  ObservedAt?: string;
  unit?: string;
  value?: number;
  observed_at?: string;
}

interface BackendEvent {
  Type?: string;
  OccurredAt?: string;
  Reason?: string;
  type?: string;
  occurred_at?: string;
  reason?: string;
}

export interface SessionQuery {
  search?: string;
  status?: string;
  sort?: string;
  limit?: number;
  offset?: number;
}

export interface SessionListResult {
  sessions: ChargingSession[];
  total: number;
}

export class ApiClient {
  constructor(private readonly baseURL: string) {}

  async active(): Promise<ChargingSession | undefined> {
    const response = await this.get<{ sessions: BackendSession[] }>("/api/v1/sessions/active");
    return response.sessions[0] ? this.toSession(response.sessions[0]) : undefined;
  }

  async sessions(query: SessionQuery): Promise<SessionListResult> {
    const params = new URLSearchParams();
    if (query.search) params.set("search", query.search);
    if (query.status && query.status !== "all") params.set("status", query.status);
    if (query.sort) params.set("sort", query.sort);
    params.set("limit", String(query.limit ?? 5));
    params.set("offset", String(query.offset ?? 0));
    const response = await this.get<{ sessions: BackendSession[]; total: number }>(`/api/v1/sessions?${params}`);
    return { sessions: response.sessions.map((session) => this.toSession(session)), total: response.total };
  }

  async detail(id: string): Promise<ChargingSession> {
    const response = await this.get<{ session: BackendSession; meter_values: BackendMeterValue[]; events: BackendEvent[] }>(`/api/v1/sessions/${encodeURIComponent(id)}`);
    return this.toSession(response.session, response.meter_values, response.events);
  }

  private async get<T>(path: string): Promise<T> {
    const response = await fetch(`${this.baseURL}${path}`);
    if (!response.ok) {
      throw new Error(`API request failed: ${response.status}`);
    }
    return response.json() as Promise<T>;
  }

  private toSession(session: BackendSession, meterValues: BackendMeterValue[] = [], events: BackendEvent[] = []): ChargingSession {
    return {
      id: session.id,
      shortId: session.id.slice(0, 18),
      status: toStatus(session.status),
      chargerName: session.charger_name || session.charger_id,
      chargerId: session.charger_id,
      connectorName: session.connector_id,
      connectorId: session.connector_id,
      evseId: session.evse_id,
      meterId: session.meter_id,
      startedAt: formatDate(session.started_at),
      endedAt: session.ended_at ? formatDate(session.ended_at) : undefined,
      duration: formatDuration(session.duration_seconds),
      energyKwh: session.energy_consumed_kwh ?? 0,
      startReadingKwh: session.start_energy_kwh ?? 0,
      endReadingKwh: session.end_energy_kwh,
      samples: toSamples(meterValues),
      events: toEvents(events),
    };
  }
}

export function apiBaseURL(): string | undefined {
  const params = new URLSearchParams(location.search);
  const fromQuery = params.get("apiBaseUrl");
  if (fromQuery) return fromQuery.replace(/\/$/, "");
  const runtime = (window as Window & { EV_CHARGING_API_BASE_URL?: string }).EV_CHARGING_API_BASE_URL;
  if (runtime) return runtime.replace(/\/$/, "");
  const env = import.meta.env.VITE_API_BASE_URL as string | undefined;
  if (env) return env.replace(/\/$/, "");
  return "";
}

export function fallbackActive(): ChargingSession {
  return activeSession;
}

export function fallbackHistory(): ChargingSession[] {
  return historySessions;
}

function toStatus(value: string): SessionStatus {
  return ["charging", "completed", "stopped", "interrupted", "unknown"].includes(value) ? value as SessionStatus : "unknown";
}

function formatDate(value: string): string {
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString([], { month: "short", day: "numeric", year: "numeric", hour: "numeric", minute: "2-digit" });
}

function formatDuration(seconds = 0): string {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  return `${hours}h ${minutes.toString().padStart(2, "0")}m`;
}

function toSamples(values: BackendMeterValue[]): MeterSample[] {
  const byTime = new Map<string, MeterSample>();
  for (const value of values) {
    const unit = value.unit ?? value.Unit ?? "";
    const observedAt = value.observed_at ?? value.ObservedAt ?? "";
    const numeric = value.value ?? value.Value ?? 0;
    const sample = byTime.get(observedAt) ?? { timestamp: observedAt, powerKw: 0, energyKwh: 0 };
    if (unit.toLowerCase() === "w") sample.powerKw = numeric / 1000;
    if (unit.toLowerCase() === "kw") sample.powerKw = numeric;
    if (unit.toLowerCase() === "kwh") sample.energyKwh = numeric;
    byTime.set(observedAt, sample);
  }
  return [...byTime.values()];
}

function toEvents(values: BackendEvent[]): TimelineEvent[] {
  return values.map((value) => {
    const kind = value.type ?? value.Type ?? "session_updated";
    return {
      timestamp: formatDate(value.occurred_at ?? value.OccurredAt ?? ""),
      kind: kind === "session_started" ? "started" : kind === "session_ended" ? "stopped" : "meter_updates",
      label: kind.replaceAll("_", " "),
      reason: value.reason ?? value.Reason,
    };
  });
}
