<script lang="ts">
  import { inFlightRequests, metrics } from "../stores/api";
  import HistogramChart from "../components/stats/HistogramChart.svelte";
  import ModelBreakdown from "../components/stats/ModelBreakdown.svelte";
  import StatCard from "../components/stats/StatCard.svelte";
  import TimeSeriesChart from "../components/stats/TimeSeriesChart.svelte";
  import TokenComposition from "../components/stats/TokenComposition.svelte";
  import { summarizeDashboard } from "../lib/metricsStats";
  import type { ModelMetricSummary } from "../lib/metricsStats";

  const nf = new Intl.NumberFormat();

  let dashboard = $derived(summarizeDashboard($metrics, $inFlightRequests));

  function number(value: number): string {
    return nf.format(Math.round(value));
  }

  function decimal(value: number, digits = 1): string {
    return value.toLocaleString(undefined, { minimumFractionDigits: digits, maximumFractionDigits: digits });
  }

  function duration(ms: number): string {
    return ms > 0 ? `${(ms / 1000).toFixed(2)}s` : "0.00s";
  }

  function cacheRate(model: ModelMetricSummary): string {
    return `${(model.tokens.cacheHitRate * 100).toFixed(1)}%`;
  }

  function lastSeen(model: ModelMetricSummary): string {
    if (!model.latestTimestamp) return "never";

    const seconds = Math.max(0, Math.floor((Date.now() - model.latestTimestamp) / 1000));
    if (seconds < 60) return `${seconds}s ago`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    return new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" }).format(
      new Date(model.latestTimestamp),
    );
  }
</script>

<div class="mx-auto flex max-w-[1800px] flex-col gap-5 p-2">
  <header class="flex flex-wrap items-end justify-between gap-4">
    <div>
      <h1 class="p-0 text-2xl font-bold">Dashboard</h1>
      <p class="mt-1 text-sm text-txtsecondary">
        Real-time token consumption, cache efficiency, and generation performance.
      </p>
    </div>
    <div class="rounded-md border border-card-border bg-surface px-3 py-2 text-sm text-txtsecondary">
      {$metrics.length === 0 ? "Waiting for metrics" : `${nf.format($metrics.length)} completed requests in memory`}
    </div>
  </header>

  <section class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-5">
    <StatCard
      title="Total Tokens"
      value={number(dashboard.tokens.total)}
      subtext={`${number(dashboard.tokens.totalInput)} input + ${number(dashboard.tokens.output)} generated`}
      tone="purple"
    />
    <StatCard
      title="Cache Hit Rate"
      value={`${decimal(dashboard.tokens.cacheHitRate * 100)}%`}
      subtext={`${number(dashboard.tokens.cached)} cached / ${number(dashboard.tokens.totalInput)} input tokens`}
      tone="blue"
    />
    <StatCard
      title="Generation Speed P50"
      value={decimal(dashboard.generationSpeed.p50)}
      unit="tok/s"
      subtext={`P95 ${decimal(dashboard.generationSpeed.p95)} tok/s · P99 ${decimal(dashboard.generationSpeed.p99)} tok/s`}
      trend={dashboard.trend.generationSpeed}
      tone="green"
    />
    <StatCard
      title="Generated Tokens"
      value={number(dashboard.tokens.output)}
      subtext={`${number(dashboard.requests)} requests · ${number(dashboard.inFlight)} in flight`}
      trend={dashboard.trend.outputTokens}
      tone="yellow"
    />
    <StatCard
      title="Average Duration"
      value={duration(dashboard.duration.avg)}
      subtext={`P95 ${duration(dashboard.duration.p95)} · P99 ${duration(dashboard.duration.p99)}`}
      trend={dashboard.trend.duration}
      tone="orange"
    />
  </section>

  <section class="grid grid-cols-1 gap-4 xl:grid-cols-2">
    <TimeSeriesChart title="Generated Tokens" series={dashboard.series.tokenVolume} unit="tokens" />
    <TimeSeriesChart title="Generation Speed" series={dashboard.series.generationSpeed} unit="tok/s" />
    <TimeSeriesChart title="Prompt Processing Speed" series={dashboard.series.promptSpeed} unit="tok/s" />
    <TimeSeriesChart title="Request Duration" series={dashboard.series.duration} unit="ms" />
  </section>

  <section class="grid grid-cols-1 gap-4 xl:grid-cols-2">
    <HistogramChart title="TokenHistogram: Generation Speed Distribution" bins={dashboard.histogram} percentiles={dashboard.generationSpeed} unit="tok/s" />
    <TokenComposition title="Global Token Composition" tokens={dashboard.tokens} />
  </section>

  <section class="rounded-lg border border-card-border bg-surface p-4 shadow-sm">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="p-0 text-lg font-semibold text-txtmain">Per-Model Consumption Breakdown</h2>
        <p class="mt-1 text-sm text-txtsecondary">Grouped by the model field recorded on each completed request.</p>
      </div>
      <span class="rounded-md bg-secondary px-3 py-1 text-sm font-semibold text-txtsecondary">{dashboard.models.length} models</span>
    </div>

    {#if dashboard.models.length > 0}
      <div class="overflow-auto rounded-lg border border-card-border-inner">
        <table class="min-w-full text-sm">
          <thead class="bg-secondary text-left text-xs uppercase tracking-wider text-txtsecondary">
            <tr>
              <th class="px-4 py-3">Model</th>
              <th class="px-4 py-3 text-right">Requests</th>
              <th class="px-4 py-3 text-right">New input</th>
              <th class="px-4 py-3 text-right">Cached</th>
              <th class="px-4 py-3 text-right">Generated</th>
              <th class="px-4 py-3 text-right">Cache hit</th>
              <th class="px-4 py-3 text-right">P50 tok/s</th>
              <th class="px-4 py-3 text-right">P95 tok/s</th>
              <th class="px-4 py-3 text-right">Avg duration</th>
              <th class="px-4 py-3 text-right">Last seen</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-card-border-inner">
            {#each dashboard.models as model (model.model)}
              <tr class="whitespace-nowrap">
                <td class="max-w-[340px] truncate px-4 py-3 font-semibold text-txtmain">{model.model}</td>
                <td class="px-4 py-3 text-right">{number(model.requests)}</td>
                <td class="px-4 py-3 text-right">{number(model.tokens.newInput)}</td>
                <td class="px-4 py-3 text-right">{number(model.tokens.cached)}</td>
                <td class="px-4 py-3 text-right">{number(model.tokens.output)}</td>
                <td class="px-4 py-3 text-right text-[#5794f2]">{cacheRate(model)}</td>
                <td class="px-4 py-3 text-right text-[#73bf69]">{decimal(model.generationSpeed.p50)}</td>
                <td class="px-4 py-3 text-right text-[#ff9830]">{decimal(model.generationSpeed.p95)}</td>
                <td class="px-4 py-3 text-right">{duration(model.duration.avg)}</td>
                <td class="px-4 py-3 text-right text-txtsecondary">{lastSeen(model)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <div class="flex min-h-[160px] items-center justify-center rounded-md border border-dashed border-card-border-inner text-sm text-txtsecondary">
        Send requests through llama-swap to populate per-model consumption.
      </div>
    {/if}
  </section>

  {#if dashboard.models.length > 0}
    <section class="space-y-6">
      <div>
        <h2 class="p-0 text-lg font-semibold text-txtmain">Per-Model Drilldown</h2>
        <p class="mt-1 text-sm text-txtsecondary">Each active model gets matching consumption and performance charts.</p>
      </div>

      {#each dashboard.models as model (model.model)}
        <ModelBreakdown summary={model} metrics={dashboard.metrics} />
      {/each}
    </section>
  {/if}
</div>
