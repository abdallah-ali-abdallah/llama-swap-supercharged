import { writable } from "svelte/store";
import type {
  Model,
  Metrics,
  ActivityLogEntry,
  LiveActivityRow,
  TokenStreamChunk,
  VersionInfo,
  LogData,
  APIEventEnvelope,
  ReqRespCapture,
  InFlightStats,
  PerformanceResponse,
} from "../lib/types";
import { connectionState } from "./theme";

const LOG_LENGTH_LIMIT = 1024 * 100; /* 100KB of log data */

// Stores
export const models = writable<Model[]>([]);
export const proxyLogs = writable<string>("");
export const upstreamLogs = writable<string>("");
export const metrics = writable<Metrics[]>([]);
export const activityLog = writable<ActivityLogEntry[]>([]);
export const inFlightRequests = writable<number>(0);
export const versionInfo = writable<VersionInfo>({
  build_date: "unknown",
  commit: "unknown",
  version: "unknown",
});

let apiEventSource: EventSource | null = null;

function appendLog(newData: string, store: typeof proxyLogs | typeof upstreamLogs): void {
  store.update((prev) => {
    const updatedLog = prev + newData;
    return updatedLog.length > LOG_LENGTH_LIMIT ? updatedLog.slice(-LOG_LENGTH_LIMIT) : updatedLog;
  });
}

export function enableAPIEvents(enabled: boolean): void {
  if (!enabled) {
    apiEventSource?.close();
    apiEventSource = null;
    metrics.set([]);
    inFlightRequests.set(0);
    return;
  }

  let retryCount = 0;
  const initialDelay = 1000; // 1 second

  const connect = () => {
    apiEventSource?.close();
    apiEventSource = new EventSource("/api/events");

    connectionState.set("connecting");

    apiEventSource.onopen = () => {
      // Clear everything on connect to keep things in sync
      proxyLogs.set("");
      upstreamLogs.set("");
      metrics.set([]);
      activityLive.set([]);
      inFlightRequests.set(0);
      models.set([]);
      retryCount = 0;
      connectionState.set("connected");
    };

    apiEventSource.onmessage = (e: MessageEvent) => {
      try {
        const message = JSON.parse(e.data) as APIEventEnvelope;
        switch (message.type) {
          case "modelStatus": {
            const newModels = JSON.parse(message.data) as Model[];
            // Sort models by name and id
            newModels.sort((a, b) => {
              return (a.name + a.id).localeCompare(b.name + b.id, undefined, { numeric: true });
            });
            models.set(newModels);
            break;
          }

          case "logData": {
            const logData = JSON.parse(message.data) as LogData;
            switch (logData.source) {
              case "proxy":
                appendLog(logData.data, proxyLogs);
                break;
              case "upstream":
                appendLog(logData.data, upstreamLogs);
                break;
            }
            break;
          }

          case "metrics": {
            const entries = JSON.parse(message.data) as ActivityLogEntry[];
            activityLog.update((prev) => [...entries, ...prev].slice(0, REALTIME_METRICS_LIMIT));
            const newMetrics = entries.map(activityLogEntryToMetrics);
            metrics.update((prevMetrics) =>
              mergeRealtimeMetrics(newMetrics, prevMetrics),
            );
            break;
          }
          case "inflight": {
            const stats = JSON.parse(message.data) as InFlightStats;
            inFlightRequests.set(stats.total ?? 0);
            break;
          }
          case "activityLive": {
            const liveRows = JSON.parse(message.data) as LiveActivityRow[];
            activityLive.set(liveRows);
            break;
          }
        }
      } catch (err) {
        console.error(e.data, err);
      }
    };

    apiEventSource.onerror = () => {
      apiEventSource?.close();
      retryCount++;
      const delay = Math.min(initialDelay * 2 ** (retryCount - 1), 5000);
      connectionState.set("disconnected");
      setTimeout(connect, delay);
    };
  };

  connect();
}

// Fetch version info when connected
connectionState.subscribe(async (status) => {
  if (status === "connected") {
    try {
      const response = await fetch("/api/version");
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data: VersionInfo = await response.json();
      versionInfo.set(data);
    } catch (error) {
      console.error(error);
    }
  }
});

export async function listModels(): Promise<Model[]> {
  try {
    const response = await fetch("/api/models/");
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    const data = await response.json();
    return data || [];
  } catch (error) {
    console.error("Failed to fetch models:", error);
    return [];
  }
}

export async function unloadAllModels(): Promise<void> {
  try {
    const response = await fetch(`/api/models/unload`, {
      method: "POST",
    });
    if (!response.ok) {
      throw new Error(`Failed to unload models: ${response.status}`);
    }
  } catch (error) {
    console.error("Failed to unload models:", error);
    throw error;
  }
}

export async function unloadSingleModel(model: string): Promise<void> {
  try {
    const response = await fetch(`/api/models/unload/${model}`, {
      method: "POST",
    });
    if (!response.ok) {
      throw new Error(`Failed to unload model: ${response.status}`);
    }
  } catch (error) {
    console.error("Failed to unload model", model, error);
    throw error;
  }
}

export async function loadModel(model: string): Promise<void> {
  try {
    const response = await fetch(`/upstream/${model}/`, {
      method: "GET",
    });
    if (!response.ok) {
      throw new Error(`Failed to load model: ${response.status}`);
    }
  } catch (error) {
    console.error("Failed to load model:", error);
    throw error;
  }
}

export async function getCapture(id: number): Promise<ReqRespCapture | null> {
  try {
    const response = await fetch(`/api/captures/${id}`);
    if (response.status === 404) {
      return null;
    }
    if (!response.ok) {
      throw new Error(`Failed to fetch capture: ${response.status}`);
    }
    return await response.json();
  } catch (error) {
    console.error("Failed to fetch capture:", error);
    return null;
  }
}

export async function fetchPerformance(after?: string): Promise<PerformanceResponse | null> {
  try {
    const url = after ? `/api/performance?after=${encodeURIComponent(after)}` : "/api/performance";
    const response = await fetch(url);
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    return await response.json();
  } catch (error) {
    console.error("Failed to fetch performance data:", error);
    return null;
  }
}

// --- Local features (re-integrated after upstream merge) ---

export const REALTIME_METRICS_MAX_AGE_MS = 10 * 60 * 1000;
export const REALTIME_METRICS_LIMIT = 5000;
export const activityLive = writable<LiveActivityRow[]>([]);

function activityLogEntryToMetrics(e: ActivityLogEntry): Metrics {
  return {
    id: e.id,
    timestamp: e.timestamp,
    model: e.model,
    cache_tokens: e.tokens?.cache_tokens ?? 0,
    new_input_tokens: e.tokens?.input_tokens ?? 0,
    output_tokens: e.tokens?.output_tokens ?? 0,
    prompt_per_second: e.tokens?.prompt_per_second ?? 0,
    tokens_per_second: e.tokens?.tokens_per_second ?? 0,
    duration_ms: e.duration_ms,
    prompt_ms: 0,
    predicted_ms: 0,
    has_capture: e.has_capture,
    multimodal: false,
    draft_acceptance_rate: 0,
    accepted_drafts: 0,
    generated_drafts: 0,
  };
}

export function mergeRealtimeMetrics(
  newMetrics: Metrics[],
  prevMetrics: Metrics[],
  now = Date.now(),
): Metrics[] {
  const cutoff = now - REALTIME_METRICS_MAX_AGE_MS;
  const merged = new Map<number, Metrics>();
  for (const metric of [...newMetrics, ...prevMetrics]) {
    const timestamp = Date.parse(metric.timestamp);
    if (!Number.isNaN(timestamp) && timestamp < cutoff) {
      continue;
    }
    if (!merged.has(metric.id)) {
      merged.set(metric.id, metric);
    }
  }

  return [...merged.values()]
    .sort((a, b) => {
      const timeDiff = Date.parse(b.timestamp) - Date.parse(a.timestamp);
      return Number.isFinite(timeDiff) && timeDiff !== 0
        ? timeDiff
        : b.id - a.id;
    })
    .slice(0, REALTIME_METRICS_LIMIT);
}

export async function listMetrics(
  start?: string,
  end?: string,
  model?: string,
  limit?: number,
): Promise<MetricsRangeResult> {
  const params = new URLSearchParams();
  if (start) params.append("start", start);
  if (end) params.append("end", end);
  if (model) params.append("model", model);
  if (limit) params.append("limit", String(limit));
  const qs = params.toString();
  const response = await fetch(`/api/metrics${qs ? "?" + qs : ""}`);
  if (!response.ok) throw new Error(`Failed to fetch metrics: ${response.status}`);
  return await response.json();
}

export interface MetricsRangeOptions {
  start?: string;
  end?: string;
  model?: string;
  limit?: number;
}

export interface MetricsRangeResult {
  metrics: Metrics[];
  truncated: boolean;
  total: number;
}

export interface ActivityFieldsSettings {
  model: boolean;
  tokens: boolean;
  speeds: boolean;
  duration: boolean;
}

export interface PersistenceSettings {
  db_path: string;
  logging_enabled: boolean;
  usage_metrics_persistence: boolean;
  activity_persistence: boolean;
  activity_capture_persistence: boolean;
  capture_redact_headers: boolean;
  activity_fields: ActivityFieldsSettings;
  conflicts: PersistenceConflict[];
}

export interface PersistenceConflict {
  key: string;
  yaml_value: boolean | string;
  db_value: boolean | string;
}

export interface PersistenceStats {
  usage_metrics_count: number;
  activity_count: number;
  capture_count: number;
}

export async function getPersistenceSettings(): Promise<PersistenceSettings | null> {
  try {
    const response = await fetch("/api/settings/persistence");
    if (!response.ok) return null;
    return await response.json();
  } catch {
    return null;
  }
}

export async function updatePersistenceSettings(
  settings: Partial<PersistenceSettings>,
): Promise<PersistenceSettings | null> {
  try {
    const response = await fetch("/api/settings/persistence", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(settings),
    });
    if (!response.ok) throw new Error(`Failed to update settings: ${response.status}`);
    return await response.json();
  } catch (error) {
    console.error("Failed to update persistence settings:", error);
    return null;
  }
}

export async function getPersistenceStats(): Promise<PersistenceStats | null> {
  try {
    const response = await fetch("/api/settings/persistence/stats");
    if (!response.ok) return null;
    return await response.json();
  } catch {
    return null;
  }
}

export async function getModelConfiguration(modelID: string): Promise<Record<string, unknown> | null> {
  try {
    const response = await fetch(`/api/models/config/${modelID}`);
    if (!response.ok) return null;
    return await response.json();
  } catch {
    return null;
  }
}

export async function cancelActivity(id: string): Promise<boolean> {
  try {
    const response = await fetch(`/api/activity/live/${id}/cancel`, { method: "POST" });
    return response.ok;
  } catch {
    return false;
  }
}

export interface LiveTokenStreamCallbacks {
  onChunk: (chunk: TokenStreamChunk) => void;
  onDone: () => void;
  onError?: (error: Error) => void;
}

export function streamLiveTokens(
  id: string,
  callbacks: LiveTokenStreamCallbacks,
): () => void {
  const es = new EventSource(`/api/activity/live/${id}/stream`);

  es.onmessage = (e: MessageEvent) => {
    try {
      if (e.data === '{"done":true}') {
        callbacks.onDone();
        es.close();
        return;
      }
      const chunk = JSON.parse(e.data) as TokenStreamChunk;
      callbacks.onChunk(chunk);
    } catch (err) {
      callbacks.onError?.(err instanceof Error ? err : new Error(String(err)));
    }
  };

  es.onerror = () => {
    callbacks.onError?.(new Error("SSE connection error"));
    es.close();
  };

  return () => es.close();
}
