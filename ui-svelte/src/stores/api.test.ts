import { afterEach, describe, expect, it, test, vi } from "vitest";
import type { Metrics } from "../lib/types";
import { getModelConfiguration, mergeRealtimeMetrics, REALTIME_METRICS_LIMIT, REALTIME_METRICS_MAX_AGE_MS } from "./api";

function metric(overrides: Partial<Metrics>): Metrics {
  return {
    id: 0,
    timestamp: "2026-04-20T12:00:00.000Z",
    model: "model-a",
    cache_tokens: 0,
    new_input_tokens: 0,
    output_tokens: 0,
    prompt_per_second: -1,
    tokens_per_second: -1,
    duration_ms: 0,
    prompt_ms: 0,
    predicted_ms: 0,
    has_capture: false,
    draft_acceptance_rate: 0,
    accepted_drafts: 0,
    generated_drafts: 0,
    ...overrides,
  };
}

describe("api realtime metrics", () => {
  test("drops metrics outside the realtime retention window", () => {
    const now = Date.parse("2026-04-20T12:10:00.000Z");
    const recentTimestamp = new Date(now - REALTIME_METRICS_MAX_AGE_MS + 1).toISOString();
    const oldTimestamp = new Date(now - REALTIME_METRICS_MAX_AGE_MS - 1).toISOString();

    const merged = mergeRealtimeMetrics(
      [metric({ id: 1, timestamp: recentTimestamp }), metric({ id: 2, timestamp: oldTimestamp })],
      [metric({ id: 3, timestamp: recentTimestamp }), metric({ id: 4, timestamp: oldTimestamp })],
      now,
    );

    expect(merged.map((item) => item.id)).toEqual([3, 1]);
  });

  test("keeps only the newest realtime metrics when over the limit", () => {
    const now = Date.parse("2026-04-20T12:10:00.000Z");
    const total = REALTIME_METRICS_LIMIT + 5;
    const metrics = Array.from({ length: total }, (_, index) =>
      metric({
        id: total - index,
        timestamp: new Date(now - index).toISOString(),
      }),
    );

    const merged = mergeRealtimeMetrics([], metrics, now);

    expect(merged).toHaveLength(REALTIME_METRICS_LIMIT);
    expect(merged[0].id).toBe(total);
    expect(merged[REALTIME_METRICS_LIMIT - 1].id).toBe(6);
  });

  test("deduplicates updated metrics by id", () => {
    const now = Date.parse("2026-04-20T12:10:00.000Z");

    const merged = mergeRealtimeMetrics(
      [metric({ id: 7, timestamp: new Date(now).toISOString(), generated_drafts: 10, accepted_drafts: 8, draft_acceptance_rate: 0.8 })],
      [metric({ id: 7, timestamp: new Date(now).toISOString(), generated_drafts: 0, accepted_drafts: 0, draft_acceptance_rate: 0 })],
      now,
    );

    expect(merged).toHaveLength(1);
    expect(merged[0].generated_drafts).toBe(10);
    expect(merged[0].accepted_drafts).toBe(8);
    expect(merged[0].draft_acceptance_rate).toBe(0.8);
  });
});

describe("api", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("fetches model configuration with URL-encoded model IDs", async () => {
    const configuration = {
      modelID: "team/qwen model",
      cmd: "llama-server -c 4096",
      proxy: "http://127.0.0.1:5800",
      checkEndpoint: "/health",
      ttl: 30,
      yaml: "cmd: llama-server -c 4096",
    };
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue(configuration),
    });
    vi.stubGlobal("fetch", fetchMock);

    await expect(getModelConfiguration("team/qwen model")).resolves.toEqual(configuration);
    expect(fetchMock).toHaveBeenCalledWith("/api/models/config/team%2Fqwen%20model");
  });

  it("returns null when model configuration is not found", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 404,
      }),
    );

    await expect(getModelConfiguration("missing")).resolves.toBeNull();
  });

  it("returns null when fetching model configuration fails", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("network unavailable")));

    await expect(getModelConfiguration("model")).resolves.toBeNull();
  });
});
