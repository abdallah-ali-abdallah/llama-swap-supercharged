<script lang="ts">
  import HistogramChart from "./HistogramChart.svelte";
  import TimeSeriesChart from "./TimeSeriesChart.svelte";
  import TokenComposition from "./TokenComposition.svelte";
  import type { Metrics } from "../../lib/types";
  import type { ModelMetricSummary } from "../../lib/metricsStats";
  import { seriesForModel } from "../../lib/metricsStats";

  interface Props {
    summary: ModelMetricSummary;
    metrics: Metrics[];
  }

  let { summary, metrics }: Props = $props();

  const nf = new Intl.NumberFormat();
  let modelMetrics = $derived(metrics.filter((metric) => (metric.model || "unknown") === summary.model));
  let series = $derived(seriesForModel(modelMetrics));

  function speed(value: number): string {
    return value > 0 ? value.toFixed(1) : "0.0";
  }

  function duration(value: number): string {
    return value > 0 ? `${(value / 1000).toFixed(3)}s` : "0.000s";
  }
</script>

<section class="space-y-4 border-t border-card-border pt-6">
  <div class="flex flex-wrap items-start justify-between gap-4">
    <div class="min-w-0">
      <h2 class="truncate p-0 text-lg font-semibold text-txtmain">{summary.model}</h2>
      <p class="mt-1 text-sm text-txtsecondary">
        {nf.format(summary.requests)} requests · {nf.format(summary.tokens.total)} total tokens
      </p>
    </div>

    <div class="grid grid-cols-2 gap-3 text-right sm:grid-cols-4">
      <div>
        <div class="text-xs uppercase tracking-wider text-txtsecondary">Cache</div>
        <div class="text-lg font-semibold text-[#5794f2]">{(summary.tokens.cacheHitRate * 100).toFixed(1)}%</div>
      </div>
      <div>
        <div class="text-xs uppercase tracking-wider text-txtsecondary">P50</div>
        <div class="text-lg font-semibold text-[#73bf69]">{speed(summary.generationSpeed.p50)}</div>
      </div>
      <div>
        <div class="text-xs uppercase tracking-wider text-txtsecondary">P95</div>
        <div class="text-lg font-semibold text-[#ff9830]">{speed(summary.generationSpeed.p95)}</div>
      </div>
      <div>
        <div class="text-xs uppercase tracking-wider text-txtsecondary">Avg duration</div>
        <div class="text-lg font-semibold text-txtmain">{duration(summary.duration.avg)}</div>
      </div>
    </div>
  </div>

  <div class="grid grid-cols-1 gap-4 xl:grid-cols-2">
    <TokenComposition title="Token Consumption" tokens={summary.tokens} />
    <HistogramChart title="Generation Speed Distribution" bins={summary.histogram} percentiles={summary.generationSpeed} unit="tok/s" />
    <TimeSeriesChart title="Token Volume" series={series.tokenVolume} unit="tokens" />
    <TimeSeriesChart title="Generation Speed" series={series.generationSpeed} unit="tok/s" />
    <TimeSeriesChart title="Prompt Processing Speed" series={series.promptSpeed} unit="tok/s" />
    <TimeSeriesChart title="Request Duration" series={series.duration} unit="s" valueFractionDigits={3} />
  </div>
</section>
