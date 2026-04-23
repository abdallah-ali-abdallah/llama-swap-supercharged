import { describe, expect, test } from "vitest";
import type { Metrics } from "./types";
import { buildHistogram, generationSpeed, metricsWithinWindow, percentile, summarizeDashboard } from "./metricsStats";

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
    has_capture: false,
    draft_acceptance_rate: 0,
    accepted_drafts: 0,
    generated_drafts: 0,
    ...overrides,
  };
}

describe("metricsStats", () => {
  test("calculates interpolated percentiles", () => {
    expect(percentile([10, 20, 30, 40], 50)).toBe(25);
    expect(percentile([10, 20, 30, 40, 50], 90)).toBe(46);
    expect(percentile([], 95)).toBe(0);
  });

  test("bins identical values without dropping samples", () => {
    const bins = buildHistogram([7, 7, 7], 10);

    expect(bins.reduce((sum, bin) => sum + bin.count, 0)).toBe(3);
    expect(bins.some((bin) => bin.count === 3)).toBe(true);
  });

  test("prefers reported generation speed over computed fallback", () => {
    expect(generationSpeed(metric({ tokens_per_second: 42, output_tokens: 10, duration_ms: 1000 }))).toBe(42);
    expect(generationSpeed(metric({ tokens_per_second: -1, output_tokens: 10, duration_ms: 2000 }))).toBe(5);
  });

  test("excludes unknown values from summaries", () => {
    const stats = summarizeDashboard([
      metric({ id: 1, tokens_per_second: -1, output_tokens: 0, duration_ms: 0, prompt_per_second: -1 }),
      metric({ id: 2, tokens_per_second: 20, output_tokens: 30, duration_ms: 500, prompt_per_second: 100 }),
    ]);

    expect(stats.generationSpeed.count).toBe(1);
    expect(stats.promptSpeed.count).toBe(1);
    expect(stats.duration.count).toBe(1);
    expect(stats.generationSpeed.p50).toBe(20);
  });

  test("converts duration chart series to seconds", () => {
    const stats = summarizeDashboard([metric({ duration_ms: 1234 })]);

    expect(stats.series.duration[0].points[0].y).toBe(1.234);
  });

  test("keeps zero draft acceptance samples in summaries and charts", () => {
    const stats = summarizeDashboard([
      metric({ id: 1, generated_drafts: 4, accepted_drafts: 0, draft_acceptance_rate: 0 }),
      metric({ id: 2, generated_drafts: 6, accepted_drafts: 3, draft_acceptance_rate: 0.5 }),
    ]);

    expect(stats.tokens.draftGenerated).toBe(10);
    expect(stats.tokens.draftAccepted).toBe(3);
    expect(stats.draftEfficiency).toBeCloseTo(0.3);
    expect(stats.draftAcceptance.count).toBe(2);
    expect(stats.series.draftAcceptance[0].points.map((point) => point.y)).toEqual([0, 50]);
  });

  test("filters metrics to a moving time window", () => {
    const now = Date.parse("2026-04-20T12:03:00.000Z");
    const windowMs = 3 * 60 * 1000;

    const filtered = metricsWithinWindow(
      [
        metric({ id: 1, timestamp: "2026-04-20T11:59:59.999Z" }),
        metric({ id: 2, timestamp: "2026-04-20T12:00:00.000Z" }),
        metric({ id: 3, timestamp: "2026-04-20T12:02:59.999Z" }),
        metric({ id: 4, timestamp: "2026-04-20T12:03:00.001Z" }),
      ],
      now,
      windowMs,
    );

    expect(filtered.map((item) => item.id)).toEqual([2, 3]);
  });

  test("computes cache hit rate with unknown cache values", () => {
    const stats = summarizeDashboard([
      metric({ new_input_tokens: 80, cache_tokens: 20, output_tokens: 10 }),
      metric({ new_input_tokens: 50, cache_tokens: -1, output_tokens: 5 }),
    ]);

    expect(stats.tokens.cached).toBe(20);
    expect(stats.tokens.totalInput).toBe(150);
    expect(stats.tokens.total).toBe(165);
    expect(stats.tokens.cacheHitRate).toBeCloseTo(20 / 150);
  });

  test("groups and orders per-model breakdowns by recent activity then token volume", () => {
    const stats = summarizeDashboard([
      metric({ id: 1, model: "model-a", timestamp: "2026-04-20T12:00:00.000Z", new_input_tokens: 100, output_tokens: 50 }),
      metric({ id: 2, model: "model-b", timestamp: "2026-04-20T12:01:00.000Z", new_input_tokens: 10, output_tokens: 5 }),
      metric({ id: 3, model: "model-c", timestamp: "2026-04-20T12:01:00.000Z", new_input_tokens: 30, output_tokens: 10 }),
    ]);

    expect(stats.models.map((modelStats) => modelStats.model)).toEqual(["model-c", "model-b", "model-a"]);
    expect(stats.models.find((modelStats) => modelStats.model === "model-a")?.tokens.output).toBe(50);
    expect(stats.models.find((modelStats) => modelStats.model === "model-a")?.tokens.total).toBe(150);
    expect(stats.models).toHaveLength(3);
  });

  test("calculates per-model token shares from global totals", () => {
    const stats = summarizeDashboard([
      metric({ id: 1, model: "model-a", new_input_tokens: 60, output_tokens: 40 }),
      metric({ id: 2, model: "model-b", new_input_tokens: 20, output_tokens: 80 }),
    ]);

    const modelA = stats.models.find((modelStats) => modelStats.model === "model-a");

    expect(modelA?.share.totalTokens).toBeCloseTo(100 / 200);
    expect(modelA?.share.generatedTokens).toBeCloseTo(40 / 120);
  });
});
