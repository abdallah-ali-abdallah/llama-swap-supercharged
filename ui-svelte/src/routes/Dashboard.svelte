<script lang="ts">
  import { RefreshCw } from "lucide-svelte";
  import { inFlightRequests, listMetrics, metrics } from "../stores/api";
  import HistogramChart from "../components/stats/HistogramChart.svelte";
  import StatCard from "../components/stats/StatCard.svelte";
  import TimeSeriesChart from "../components/stats/TimeSeriesChart.svelte";
  import TokenComposition from "../components/stats/TokenComposition.svelte";
  import { metricsWithinWindow, summarizeDashboard } from "../lib/metricsStats";
  import type { ModelMetricSummary } from "../lib/metricsStats";
  import type { Metrics } from "../lib/types";

  const nf = new Intl.NumberFormat();
  const REALTIME_WINDOW_MS = 3 * 60 * 1000;
  const RANGE_OPTIONS = [
    { value: "realtime", label: "Realtime" },
    { value: "5m", label: "Past 5 min" },
    { value: "10m", label: "Past 10 min" },
    { value: "1h", label: "Past 1 hour" },
    { value: "8h", label: "Past 8 hours" },
    { value: "1d", label: "Past day" },
    { value: "1w", label: "Past week" },
    { value: "1mo", label: "Past month" },
    { value: "all", label: "All" },
    { value: "custom", label: "Custom" },
  ];

  let selectedRange = $state("realtime");
  let customFrom = $state("");
  let customTo = $state("");
  let historicalMetrics = $state<Metrics[]>([]);
  let historicalLoading = $state(false);
  let historicalTruncated = $state(false);
  let historicalError = $state("");
  let refreshTick = $state(0);
  let realtimeNow = $state(Date.now());
  let displayedMetrics = $derived(selectedRange === "realtime" ? metricsWithinWindow($metrics, realtimeNow, REALTIME_WINDOW_MS) : historicalMetrics);
  let dashboard = $derived(summarizeDashboard(displayedMetrics, selectedRange === "realtime" ? $inFlightRequests : 0));

  function number(value: number): string {
    return nf.format(Math.round(value));
  }

  function decimal(value: number, digits = 1): string {
    return value.toLocaleString(undefined, { minimumFractionDigits: digits, maximumFractionDigits: digits });
  }

  function duration(ms: number): string {
    return ms > 0 ? `${(ms / 1000).toFixed(3)}s` : "0.000s";
  }

  function cacheRate(model: ModelMetricSummary): string {
    return `${(model.tokens.cacheHitRate * 100).toFixed(1)}%`;
  }

  function percent(value: number): string {
    return `${(value * 100).toFixed(1)}%`;
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

  function dateTimeToISO(value: string): string | undefined {
    if (!value) return undefined;
    const parsed = new Date(value);
    return Number.isNaN(parsed.getTime()) ? undefined : parsed.toISOString();
  }

  function refreshHistorical(): void {
    refreshTick++;
  }

  $effect(() => {
    if (selectedRange !== "realtime") return;

    realtimeNow = Date.now();
    const interval = window.setInterval(() => {
      realtimeNow = Date.now();
    }, 1000);

    return () => {
      window.clearInterval(interval);
    };
  });

  $effect(() => {
    const range = selectedRange;
    const from = customFrom;
    const to = customTo;
    const tick = refreshTick;

    if (range === "realtime") {
      historicalLoading = false;
      historicalTruncated = false;
      historicalError = "";
      return;
    }

    const fromISO = range === "custom" ? dateTimeToISO(from) : undefined;
    const toISO = range === "custom" ? dateTimeToISO(to) : undefined;
    if (range === "custom" && !fromISO && !toISO) {
      historicalMetrics = [];
      historicalLoading = false;
      historicalTruncated = false;
      historicalError = "Select a custom start or end time.";
      return;
    }

    let cancelled = false;
    historicalLoading = true;
    historicalError = "";

    listMetrics({ range, from: fromISO, to: toISO })
      .then((result) => {
        if (cancelled) return;
        historicalMetrics = result.metrics;
        historicalTruncated = result.truncated;
      })
      .catch((error) => {
        if (cancelled) return;
        historicalMetrics = [];
        historicalTruncated = false;
        historicalError = error instanceof Error ? error.message : "Failed to load metrics.";
      })
      .finally(() => {
        if (!cancelled) historicalLoading = false;
      });

    tick;
    return () => {
      cancelled = true;
    };
  });
</script>

<div class="mx-auto flex max-w-[1800px] flex-col gap-5 p-2">
  <header class="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
    <div class="min-w-0">
      <h1 class="p-0 text-2xl font-bold">Dashboard</h1>
      <p class="mt-1 text-sm text-txtsecondary">
        Real-time token consumption, cache efficiency, generation performance, and speculative decoding health.
      </p>
    </div>
    <div class="flex min-w-0 flex-col gap-2 xl:items-end">
      <div class="flex min-w-0 flex-col gap-2 xl:items-end">
        <div class="flex min-w-0 items-center gap-2 xl:flex-nowrap">
          <div class="flex min-w-0 flex-nowrap items-center gap-2 overflow-x-auto pb-1 xl:justify-end xl:pb-0">
            {#each RANGE_OPTIONS as option (option.value)}
              <button
                type="button"
                onclick={() => (selectedRange = option.value)}
                class={`shrink-0 whitespace-nowrap rounded-md border px-3 py-2 text-sm font-semibold transition ${
                  selectedRange === option.value
                    ? "border-[#5794f2] bg-[#5794f2]/20 text-[#174a8b] dark:text-[#cfe2ff]"
                    : "border-card-border bg-surface text-txtsecondary hover:border-card-border-inner hover:text-txtmain"
                }`}
              >
                {option.label}
              </button>
            {/each}
          </div>
          <div class="flex h-10 w-[108px] shrink-0 justify-end">
            {#if selectedRange !== "realtime"}
              <button
                type="button"
                onclick={refreshHistorical}
                disabled={historicalLoading}
                title="Refresh metrics"
                class="inline-flex items-center gap-2 rounded-md border border-card-border bg-surface px-3 py-2 text-sm font-semibold text-txtsecondary transition hover:border-card-border-inner hover:text-txtmain disabled:opacity-60"
              >
                <RefreshCw size={15} class={historicalLoading ? "animate-spin" : ""} />
                Refresh
              </button>
            {/if}
          </div>
        </div>
      </div>
      {#if selectedRange === "custom"}
        <div class="flex flex-wrap items-center gap-2 text-sm text-txtsecondary xl:justify-end">
          <input
            type="datetime-local"
            bind:value={customFrom}
            class="rounded-md border border-card-border bg-secondary px-3 py-2 text-txtmain outline-none focus:border-[#5794f2]"
            aria-label="Custom range start"
          />
          <span>to</span>
          <input
            type="datetime-local"
            bind:value={customTo}
            class="rounded-md border border-card-border bg-secondary px-3 py-2 text-txtmain outline-none focus:border-[#5794f2]"
            aria-label="Custom range end"
          />
        </div>
      {/if}
      <div class="rounded-md border border-card-border bg-surface px-3 py-2 text-sm text-txtsecondary xl:self-end">
        {#if historicalLoading}
          Loading metrics
        {:else if historicalError}
          {historicalError}
        {:else if displayedMetrics.length === 0}
          Waiting for metrics
        {:else}
          {nf.format(displayedMetrics.length)} completed requests{selectedRange === "realtime" ? " in last 3 min" : ""}
          {historicalTruncated ? " (limited)" : ""}
        {/if}
      </div>
    </div>
  </header>

  <section class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-6">
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
      tone="green"
    />
    <StatCard
      title="Generated Tokens"
      value={number(dashboard.tokens.output)}
      subtext={`${number(dashboard.requests)} requests · ${number(dashboard.inFlight)} in flight`}
      tone="yellow"
    />
    <StatCard
      title="Average Duration"
      value={duration(dashboard.duration.avg)}
      subtext={`P95 ${duration(dashboard.duration.p95)} · P99 ${duration(dashboard.duration.p99)}`}
      tone="orange"
    />
    <StatCard
      title="Draft Tokens"
      value={number(dashboard.tokens.draftGenerated)}
      subtext={dashboard.tokens.draftGenerated > 0
        ? `${number(dashboard.tokens.draftAccepted)} accepted · ${decimal(dashboard.draftEfficiency * 100)}% overall acceptance`
        : "No speculative decoding samples in range"}
      tone="neutral"
    />
  </section>

  <section class="grid grid-cols-1 gap-4 xl:grid-cols-2">
    <TimeSeriesChart title="Generated Tokens" series={dashboard.series.tokenVolume} unit="tokens" toggleableLegend curve="smooth" smoothSamples />
    <TimeSeriesChart title="Generation Speed" series={dashboard.series.generationSpeed} unit="tok/s" curve="smooth" smoothSamples />
    <TimeSeriesChart title="Prompt Processing Speed" series={dashboard.series.promptSpeed} unit="tok/s" curve="smooth" smoothSamples />
    <TimeSeriesChart title="Request Duration" series={dashboard.series.duration} unit="s" valueFractionDigits={3} curve="smooth" smoothSamples />
  </section>

  <section class="grid grid-cols-1 gap-4 xl:grid-cols-2">
    <TimeSeriesChart
      title="Draft Acceptance Rate"
      series={dashboard.series.draftAcceptance}
      unit="%"
      valueFractionDigits={1}
      curve="smooth"
      smoothSamples
    />
    <div class="rounded-lg border border-card-border bg-surface p-4 shadow-sm">
      <div class="flex items-start justify-between gap-3">
        <div>
          <h2 class="p-0 text-xs font-semibold uppercase tracking-wider text-txtsecondary">Speculative Decoding</h2>
          <p class="mt-2 text-sm text-txtsecondary">
            {#if dashboard.tokens.draftGenerated > 0}
              {number(dashboard.tokens.draftGenerated)} drafted tokens with {number(dashboard.tokens.draftAccepted)} accepted across {number(dashboard.draftAcceptance.count)} requests.
            {:else}
              No speculative decoding samples in the selected time range.
            {/if}
          </p>
        </div>
      </div>

      <div class="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-2">
        <StatCard
          title="Acceptance P50"
          value={`${decimal(dashboard.draftAcceptance.p50)}%`}
          subtext={`P95 ${decimal(dashboard.draftAcceptance.p95)}% · Max ${decimal(dashboard.draftAcceptance.max)}%`}
          tone="green"
        />
        <StatCard
          title="Rejected Drafts"
          value={number(Math.max(0, dashboard.tokens.draftGenerated - dashboard.tokens.draftAccepted))}
          subtext={`${decimal(dashboard.draftEfficiency * 100)}% accepted overall`}
          tone="yellow"
        />
      </div>
    </div>
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
              <th class="px-4 py-3 text-right">Unique processed token</th>
              <th class="px-4 py-3 text-right">Cached</th>
              <th class="px-4 py-3 text-right">Generated</th>
              <th class="px-4 py-3 text-right">Total tokens</th>
              <th class="px-4 py-3 text-right">Processed %</th>
              <th class="px-4 py-3 text-right">Generated %</th>
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
                <td class="px-4 py-3 text-right font-semibold text-txtmain">{number(model.tokens.total)}</td>
                <td class="px-4 py-3 text-right">{percent(model.share.totalTokens)}</td>
                <td class="px-4 py-3 text-right">{percent(model.share.generatedTokens)}</td>
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
</div>
