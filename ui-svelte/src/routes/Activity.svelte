<script lang="ts">
  import { RefreshCw } from "lucide-svelte";
  import { metrics, getCapture, listMetrics } from "../stores/api";
  import Tooltip from "../components/Tooltip.svelte";
  import CaptureDialog from "../components/CaptureDialog.svelte";
  import type { Metrics, ReqRespCapture } from "../lib/types";

  const RANGE_OPTIONS = [
    { value: "realtime", label: "Realtime" },
    { value: "5m", label: "Past 5 minutes" },
    { value: "10m", label: "Past 10 minutes" },
    { value: "1h", label: "Past 1 hour" },
    { value: "8h", label: "Past 8 hours" },
    { value: "1d", label: "Past day" },
    { value: "1w", label: "Past week" },
    { value: "1mo", label: "Past month" },
    { value: "all", label: "All" },
    { value: "custom", label: "Custom" },
  ];

  function formatSpeed(speed: number): string {
    return speed < 0 ? "unknown" : speed.toFixed(2) + " t/s";
  }

  function formatDuration(ms: number): string {
    return (ms / 1000).toFixed(2) + "s";
  }

  function formatRelativeTime(timestamp: string): string {
    const now = new Date();
    const date = new Date(timestamp);
    const diffInSeconds = Math.floor((now.getTime() - date.getTime()) / 1000);

    // Handle future dates by returning "just now"
    if (diffInSeconds < 5) {
      return "now";
    }

    if (diffInSeconds < 60) {
      return `${diffInSeconds}s ago`;
    }

    const diffInMinutes = Math.floor(diffInSeconds / 60);
    if (diffInMinutes < 60) {
      return `${diffInMinutes}m ago`;
    }

    const diffInHours = Math.floor(diffInMinutes / 60);
    if (diffInHours < 24) {
      return `${diffInHours}h ago`;
    }

    return "a while ago";
  }

  let selectedRange = $state("realtime");
  let customFrom = $state("");
  let customTo = $state("");
  let historicalMetrics = $state<Metrics[]>([]);
  let historicalLoading = $state(false);
  let historicalTruncated = $state(false);
  let historicalError = $state("");
  let refreshTick = $state(0);
  let displayedMetrics = $derived(selectedRange === "realtime" ? $metrics : historicalMetrics);
  let sortedMetrics = $derived([...displayedMetrics].sort((a, b) => b.id - a.id));

  let selectedCapture = $state<ReqRespCapture | null>(null);
  let dialogOpen = $state(false);
  let loadingCaptureId = $state<number | null>(null);

  function dateTimeToISO(value: string): string | undefined {
    if (!value) return undefined;
    const parsed = new Date(value);
    return Number.isNaN(parsed.getTime()) ? undefined : parsed.toISOString();
  }

  function refreshHistorical(): void {
    refreshTick++;
  }

  async function viewCapture(id: number) {
    loadingCaptureId = id;
    const capture = await getCapture(id);
    loadingCaptureId = null;
    if (capture) {
      selectedCapture = capture;
      dialogOpen = true;
    }
  }

  function closeDialog() {
    dialogOpen = false;
    selectedCapture = null;
  }

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

    listMetrics({ range, from: fromISO, to: toISO, scope: "activity" })
      .then((result) => {
        if (cancelled) return;
        historicalMetrics = result.metrics;
        historicalTruncated = result.truncated;
      })
      .catch((error) => {
        if (cancelled) return;
        historicalMetrics = [];
        historicalTruncated = false;
        historicalError = error instanceof Error ? error.message : "Failed to load activity.";
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

<div class="flex flex-col gap-4 p-2">
  <header class="flex flex-wrap items-end justify-between gap-4">
    <div>
      <h1 class="text-2xl font-bold">Activity</h1>
      <p class="mt-1 text-sm text-txtsecondary">Completed LLM requests and captured request details.</p>
    </div>
    <div class="flex flex-col items-end gap-2">
      <div class="flex flex-wrap items-center justify-end gap-2">
        {#each RANGE_OPTIONS as option (option.value)}
          <button
            type="button"
            onclick={() => (selectedRange = option.value)}
            class={`rounded-md border px-3 py-2 text-sm font-semibold transition ${
              selectedRange === option.value
                ? "border-[#5794f2] bg-[#5794f2]/20 text-[#174a8b] dark:text-[#cfe2ff]"
                : "border-card-border bg-surface text-txtsecondary hover:border-card-border-inner hover:text-txtmain"
            }`}
          >
            {option.label}
          </button>
        {/each}
        {#if selectedRange !== "realtime"}
          <button
            type="button"
            onclick={refreshHistorical}
            disabled={historicalLoading}
            title="Refresh activity"
            class="inline-flex items-center gap-2 rounded-md border border-card-border bg-surface px-3 py-2 text-sm font-semibold text-txtsecondary transition hover:border-card-border-inner hover:text-txtmain disabled:opacity-60"
          >
            <RefreshCw size={15} class={historicalLoading ? "animate-spin" : ""} />
            Refresh
          </button>
        {/if}
      </div>
      {#if selectedRange === "custom"}
        <div class="flex flex-wrap items-center justify-end gap-2 text-sm text-txtsecondary">
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
      <div class="rounded-md border border-card-border bg-surface px-3 py-2 text-sm text-txtsecondary">
        {#if historicalLoading}
          Loading activity
        {:else if historicalError}
          {historicalError}
        {:else if displayedMetrics.length === 0}
          No activity
        {:else}
          {displayedMetrics.length.toLocaleString()} completed requests{selectedRange === "realtime" ? " in memory" : ""}
          {historicalTruncated ? " (limited)" : ""}
        {/if}
      </div>
    </div>
  </header>

  {#if displayedMetrics.length === 0}
    <div class="text-center py-8">
      <p class="text-gray-600">No activity data available</p>
    </div>
  {:else}
    <div class="card overflow-auto">
      <table class="min-w-full divide-y">
        <thead class="border-gray-200 dark:border-white/10">
          <tr class="text-left text-xs uppercase tracking-wider">
            <th class="px-6 py-3">ID</th>
            <th class="px-6 py-3">Time</th>
            <th class="px-6 py-3">Model</th>
            <th class="px-6 py-3">
              Cached <Tooltip content="prompt tokens from cache" />
            </th>
            <th class="px-6 py-3">
              Prompt <Tooltip content="new prompt tokens processed" />
            </th>
            <th class="px-6 py-3">Generated</th>
            <th class="px-6 py-3">Prompt Processing</th>
            <th class="px-6 py-3">Generation Speed</th>
            <th class="px-6 py-3">Duration</th>
            <th class="px-6 py-3">Capture</th>
          </tr>
        </thead>
        <tbody class="divide-y">
          {#each sortedMetrics as metric (metric.id)}
            <tr class="whitespace-nowrap text-sm border-gray-200 dark:border-white/10">
              <td class="px-4 py-4">{metric.id + 1}</td>
              <td class="px-6 py-4">{formatRelativeTime(metric.timestamp)}</td>
              <td class="px-6 py-4">{metric.model}</td>
              <td class="px-6 py-4">{metric.cache_tokens > 0 ? metric.cache_tokens.toLocaleString() : "-"}</td>
              <td class="px-6 py-4">{metric.new_input_tokens.toLocaleString()}</td>
              <td class="px-6 py-4">{metric.output_tokens.toLocaleString()}</td>
              <td class="px-6 py-4">{formatSpeed(metric.prompt_per_second)}</td>
              <td class="px-6 py-4">{formatSpeed(metric.tokens_per_second)}</td>
              <td class="px-6 py-4">{formatDuration(metric.duration_ms)}</td>
              <td class="px-6 py-4">
                {#if metric.has_capture}
                  <button
                    onclick={() => viewCapture(metric.id)}
                    disabled={loadingCaptureId === metric.id}
                    class="btn btn--sm"
                  >
                    {loadingCaptureId === metric.id ? "..." : "View"}
                  </button>
                {:else}
                  <span class="text-txtsecondary">-</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<CaptureDialog capture={selectedCapture} open={dialogOpen} onclose={closeDialog} />
