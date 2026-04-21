import type { Metrics } from "./types";

export interface PercentileSummary {
  min: number;
  max: number;
  avg: number;
  p50: number;
  p90: number;
  p95: number;
  p99: number;
  count: number;
}

export interface HistogramBin {
  start: number;
  end: number;
  count: number;
}

export interface ChartPoint {
  x: number;
  y: number | null;
}

export interface ChartSeries {
  label: string;
  color: string;
  points: ChartPoint[];
}

export interface TokenTotals {
  newInput: number;
  cached: number;
  output: number;
  totalInput: number;
  total: number;
  cacheHitRate: number;
}

export interface MetricSummary {
  requests: number;
  inFlight: number;
  tokens: TokenTotals;
  promptSpeed: PercentileSummary;
  generationSpeed: PercentileSummary;
  duration: PercentileSummary;
  generatedTokens: PercentileSummary;
  histogram: HistogramBin[];
  latestTimestamp: number | null;
  trend: {
    generationSpeed: number | null;
    duration: number | null;
    outputTokens: number | null;
  };
}

export interface ModelMetricSummary extends MetricSummary {
  model: string;
  share: {
    totalTokens: number;
    generatedTokens: number;
  };
}

export interface DashboardStats extends MetricSummary {
  metrics: Metrics[];
  models: ModelMetricSummary[];
  series: {
    tokenVolume: ChartSeries[];
    generationSpeed: ChartSeries[];
    promptSpeed: ChartSeries[];
    duration: ChartSeries[];
  };
}

const EMPTY_PERCENTILES: PercentileSummary = {
  min: 0,
  max: 0,
  avg: 0,
  p50: 0,
  p90: 0,
  p95: 0,
  p99: 0,
  count: 0,
};

const SERIES_COLORS = {
  newInput: "#7dc36f",
  cached: "#5794f2",
  output: "#f2cc0c",
  generationSpeed: "#73bf69",
  promptSpeed: "#b877d9",
  duration: "#ff9830",
};

function validNumber(value: number): boolean {
  return Number.isFinite(value) && value >= 0;
}

export function metricTimestamp(metric: Metrics): number {
  const parsed = Date.parse(metric.timestamp);
  return Number.isFinite(parsed) ? parsed : metric.id;
}

export function metricsWithinWindow(metrics: Metrics[], now: number, windowMs: number): Metrics[] {
  const start = now - windowMs;
  return metrics.filter((metric) => {
    const timestamp = metricTimestamp(metric);
    return timestamp >= start && timestamp <= now;
  });
}

function sortMetrics(metrics: Metrics[]): Metrics[] {
  return [...metrics].sort((a, b) => {
    const timeDiff = metricTimestamp(a) - metricTimestamp(b);
    return timeDiff !== 0 ? timeDiff : a.id - b.id;
  });
}

export function generationSpeed(metric: Metrics): number | null {
  if (validNumber(metric.tokens_per_second) && metric.tokens_per_second > 0) {
    return metric.tokens_per_second;
  }

  if (metric.output_tokens > 0 && metric.duration_ms > 0) {
    return metric.output_tokens / (metric.duration_ms / 1000);
  }

  return null;
}

export function promptSpeed(metric: Metrics): number | null {
  return validNumber(metric.prompt_per_second) && metric.prompt_per_second > 0 ? metric.prompt_per_second : null;
}

export function percentile(values: number[], p: number): number {
  const sorted = values.filter(Number.isFinite).sort((a, b) => a - b);
  if (sorted.length === 0) return 0;
  if (sorted.length === 1) return sorted[0];

  const bounded = Math.min(100, Math.max(0, p));
  const rank = (bounded / 100) * (sorted.length - 1);
  const lower = Math.floor(rank);
  const upper = Math.ceil(rank);
  const weight = rank - lower;

  return sorted[lower] + (sorted[upper] - sorted[lower]) * weight;
}

export function summarizeValues(values: number[]): PercentileSummary {
  const validValues = values.filter(Number.isFinite);
  if (validValues.length === 0) return { ...EMPTY_PERCENTILES };

  const total = validValues.reduce((sum, value) => sum + value, 0);

  return {
    min: Math.min(...validValues),
    max: Math.max(...validValues),
    avg: total / validValues.length,
    p50: percentile(validValues, 50),
    p90: percentile(validValues, 90),
    p95: percentile(validValues, 95),
    p99: percentile(validValues, 99),
    count: validValues.length,
  };
}

export function buildHistogram(values: number[], requestedBins = 24): HistogramBin[] {
  const validValues = values.filter(Number.isFinite);
  if (validValues.length === 0) return [];

  const min = Math.min(...validValues);
  const max = Math.max(...validValues);
  const binCount = Math.max(1, Math.min(requestedBins, Math.max(6, Math.ceil(Math.sqrt(validValues.length) * 2))));
  const range = max === min ? 1 : max - min;
  const start = max === min ? min - 0.5 : min;
  const binSize = range / binCount;

  const bins = Array.from({ length: binCount }, (_, index) => ({
    start: start + index * binSize,
    end: start + (index + 1) * binSize,
    count: 0,
  }));

  for (const value of validValues) {
    const rawIndex = Math.floor((value - start) / binSize);
    const index = Math.max(0, Math.min(bins.length - 1, rawIndex));
    bins[index].count++;
  }

  return bins;
}

function totalsFor(metrics: Metrics[]): TokenTotals {
  const newInput = metrics.reduce((sum, metric) => sum + Math.max(0, metric.new_input_tokens || 0), 0);
  const cached = metrics.reduce((sum, metric) => sum + Math.max(0, metric.cache_tokens || 0), 0);
  const output = metrics.reduce((sum, metric) => sum + Math.max(0, metric.output_tokens || 0), 0);
  const totalInput = newInput + cached;

  return {
    newInput,
    cached,
    output,
    totalInput,
    total: totalInput + output,
    cacheHitRate: totalInput > 0 ? cached / totalInput : 0,
  };
}

function trend(values: number[]): number | null {
  if (values.length < 2) return null;

  const midpoint = Math.floor(values.length / 2);
  const previous = values.slice(0, midpoint);
  const recent = values.slice(midpoint);
  if (previous.length === 0 || recent.length === 0) return null;

  const previousAvg = previous.reduce((sum, value) => sum + value, 0) / previous.length;
  const recentAvg = recent.reduce((sum, value) => sum + value, 0) / recent.length;
  if (previousAvg === 0) return recentAvg === 0 ? 0 : 1;

  return (recentAvg - previousAvg) / previousAvg;
}

function lineSeries(metrics: Metrics[], label: string, color: string, value: (metric: Metrics) => number | null): ChartSeries {
  return {
    label,
    color,
    points: metrics.map((metric) => ({
      x: metricTimestamp(metric),
      y: value(metric),
    })),
  };
}

function baseSummary(metrics: Metrics[], inFlight: number): MetricSummary {
  const ordered = sortMetrics(metrics);
  const generationSpeeds = ordered.map(generationSpeed).filter((value): value is number => value !== null);
  const promptSpeeds = ordered.map(promptSpeed).filter((value): value is number => value !== null);
  const durations = ordered.map((metric) => metric.duration_ms).filter((value) => validNumber(value) && value > 0);
  const generatedTokens = ordered.map((metric) => metric.output_tokens).filter((value) => validNumber(value) && value > 0);

  return {
    requests: ordered.length,
    inFlight,
    tokens: totalsFor(ordered),
    promptSpeed: summarizeValues(promptSpeeds),
    generationSpeed: summarizeValues(generationSpeeds),
    duration: summarizeValues(durations),
    generatedTokens: summarizeValues(generatedTokens),
    histogram: buildHistogram(generationSpeeds),
    latestTimestamp: ordered.length > 0 ? metricTimestamp(ordered[ordered.length - 1]) : null,
    trend: {
      generationSpeed: trend(generationSpeeds),
      duration: trend(durations),
      outputTokens: trend(generatedTokens),
    },
  };
}

function modelSummaries(metrics: Metrics[]): ModelMetricSummary[] {
  const groups = new Map<string, Metrics[]>();
  const globalTotals = totalsFor(metrics);

  for (const metric of metrics) {
    const model = metric.model || "unknown";
    groups.set(model, [...(groups.get(model) || []), metric]);
  }

  return [...groups.entries()]
    .map(([model, groupedMetrics]) => {
      const summary = baseSummary(groupedMetrics, 0);
      return {
        model,
        share: {
          totalTokens: globalTotals.total > 0 ? summary.tokens.total / globalTotals.total : 0,
          generatedTokens: globalTotals.output > 0 ? summary.tokens.output / globalTotals.output : 0,
        },
        ...summary,
      };
    })
    .sort((a, b) => {
      const recentDiff = (b.latestTimestamp || 0) - (a.latestTimestamp || 0);
      if (recentDiff !== 0) return recentDiff;
      return b.tokens.total - a.tokens.total;
    });
}

function globalSeries(metrics: Metrics[]): DashboardStats["series"] {
  return {
    tokenVolume: [
      lineSeries(metrics, "Unique processed token", SERIES_COLORS.newInput, (metric) => Math.max(0, metric.new_input_tokens || 0)),
      lineSeries(metrics, "Cached", SERIES_COLORS.cached, (metric) => (metric.cache_tokens >= 0 ? metric.cache_tokens : null)),
      lineSeries(metrics, "Generated", SERIES_COLORS.output, (metric) => Math.max(0, metric.output_tokens || 0)),
    ],
    generationSpeed: [lineSeries(metrics, "Generated tok/sec", SERIES_COLORS.generationSpeed, generationSpeed)],
    promptSpeed: [lineSeries(metrics, "Prompt tok/sec", SERIES_COLORS.promptSpeed, promptSpeed)],
    duration: [lineSeries(metrics, "Duration", SERIES_COLORS.duration, (metric) => (metric.duration_ms > 0 ? metric.duration_ms / 1000 : null))],
  };
}

export function seriesForModel(metrics: Metrics[]): DashboardStats["series"] {
  return globalSeries(sortMetrics(metrics));
}

export function summarizeDashboard(metrics: Metrics[], inFlight = 0): DashboardStats {
  const ordered = sortMetrics(metrics);

  return {
    metrics: ordered,
    models: modelSummaries(ordered),
    series: globalSeries(ordered),
    ...baseSummary(ordered, inFlight),
  };
}
