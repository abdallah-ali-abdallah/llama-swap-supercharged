import { writable } from "svelte/store";
import type { Model, Metrics, VersionInfo, LogData, APIEventEnvelope, ReqRespCapture, InFlightStats, ModelConfiguration } from "../lib/types";
import { connectionState } from "./theme";

const LOG_LENGTH_LIMIT = 1024 * 100; /* 100KB of log data */

// Stores
export const models = writable<Model[]>([]);
export const proxyLogs = writable<string>("");
export const upstreamLogs = writable<string>("");
export const metrics = writable<Metrics[]>([]);
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
              return (a.name + a.id).localeCompare(b.name + b.id, undefined, { numeric : true} );
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
            const newMetrics = JSON.parse(message.data) as Metrics[];
            metrics.update((prevMetrics) => [...newMetrics, ...prevMetrics]);
            break;
          }
          case "inflight": {
            const stats = JSON.parse(message.data) as InFlightStats;
            inFlightRequests.set(stats.total ?? 0);
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
      const delay = Math.min(initialDelay * Math.pow(2, retryCount - 1), 5000);
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

export interface MetricsRangeOptions {
  range: string;
  from?: string;
  to?: string;
  scope?: "usage" | "activity";
}

export interface MetricsRangeResult {
  metrics: Metrics[];
  truncated: boolean;
}

export async function listMetrics(options: MetricsRangeOptions): Promise<MetricsRangeResult> {
  try {
    const params = new URLSearchParams();
    params.set("range", options.range);
    if (options.from) params.set("from", options.from);
    if (options.to) params.set("to", options.to);
    if (options.scope) params.set("scope", options.scope);

    const response = await fetch(`/api/metrics?${params.toString()}`);
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }
    const data = (await response.json()) as Metrics[] | null;
    return {
      metrics: data || [],
      truncated: response.headers.get("X-Metrics-Truncated") === "true",
    };
  } catch (error) {
    console.error("Failed to fetch metrics:", error);
    return { metrics: [], truncated: false };
  }
}

export interface ActivityFieldsSettings {
  model: boolean;
  tokens: boolean;
  speeds: boolean;
  duration: boolean;
}

export interface PersistenceSettings {
  sqlite_available: boolean;
  yaml_available: boolean;
  yaml_path: string;
  yaml_conflicts?: PersistenceConflict[];
  db_path: string;
  retention_days: number;
  logging_enabled: boolean;
  usage_metrics_persistence: boolean;
  activity_persistence: boolean;
  activity_capture_persistence: boolean;
  capture_redact_headers: boolean;
  activity_fields: ActivityFieldsSettings;
  stats?: PersistenceStats;
}

export interface PersistenceConflict {
  field: string;
  yaml_value: string;
  sqlite_value: string;
}

export interface PersistenceStats {
  db_size_bytes: number;
  wal_size_bytes: number;
  shm_size_bytes: number;
  total_size_bytes: number;
  usage_metrics_rows: number;
  activity_rows: number;
  activity_captures: number;
  capture_bytes: number;
  settings_rows: number;
  oldest_metric_ms?: number;
  newest_metric_ms?: number;
  oldest_activity_ms?: number;
  newest_activity_ms?: number;
}

export async function getPersistenceSettings(): Promise<PersistenceSettings | null> {
  try {
    const response = await fetch("/api/settings/persistence");
    if (!response.ok) {
      throw new Error(`Failed to fetch persistence settings: ${response.status}`);
    }
    return await response.json();
  } catch (error) {
    console.error("Failed to fetch persistence settings:", error);
    return null;
  }
}

export async function updatePersistenceSettings(settings: PersistenceSettings): Promise<PersistenceSettings> {
  const response = await fetch("/api/settings/persistence", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(settings),
  });
  if (!response.ok) {
    throw new Error(`Failed to update persistence settings: ${response.status}`);
  }
  return await response.json();
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

export async function getModelConfiguration(model: string): Promise<ModelConfiguration | null> {
  try {
    const response = await fetch(`/api/models/config/${encodeURIComponent(model)}`);
    if (response.status === 404) {
      return null;
    }
    if (!response.ok) {
      throw new Error(`Failed to fetch model configuration: ${response.status}`);
    }
    return await response.json();
  } catch (error) {
    console.error("Failed to fetch model configuration:", error);
    return null;
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
