<script lang="ts">
  import { inFlightRequests, metrics } from "../stores/api";

  function pct(value: number, total: number): string {
    if (total === 0) return "0.0%";
    return ((value / total) * 100).toFixed(1) + "%";
  }

  let stats = $derived.by(() => {
    const totalRequests = $metrics.length;
    const inFlight = $inFlightRequests;

    if (totalRequests === 0) {
      return {
        totalRequests: 0,
        totalNewInputTokens: 0,
        totalCachedTokens: 0,
        totalOutputTokens: 0,
        totalInputTokens: 0,
        cacheHitRate: "0.0",
        avgDurationMs: "0",
        avgGenSpeed: "0",
        inFlight,
      };
    }

    const totalNewInputTokens = $metrics.reduce((sum, m) => sum + (m.new_input_tokens || 0), 0);
    const totalCachedTokens = $metrics.reduce((sum, m) => sum + Math.max(0, m.cache_tokens || 0), 0);
    const totalOutputTokens = $metrics.reduce((sum, m) => sum + m.output_tokens, 0);
    const totalInputTokens = totalNewInputTokens + totalCachedTokens;

    // Cache hit rate: what fraction of input was served from cache
    const cacheHitRate = totalInputTokens > 0
      ? ((totalCachedTokens / totalInputTokens) * 100).toFixed(1)
      : "0.0";

    // Average duration across all requests
    const avgDurationMs = $metrics.reduce((sum, m) => sum + m.duration_ms, 0) / totalRequests;

    // Average generation speed (tokens/sec) across valid requests
    const validMetrics = $metrics.filter((m) => m.duration_ms > 0 && m.output_tokens > 0);
    const avgGenSpeed = validMetrics.length > 0
      ? validMetrics.reduce((sum, m) => sum + m.tokens_per_second, 0) / validMetrics.length
      : 0;

    return {
      totalRequests,
      totalNewInputTokens,
      totalCachedTokens,
      totalOutputTokens,
      totalInputTokens,
      cacheHitRate,
      avgDurationMs: avgDurationMs.toFixed(0),
      avgGenSpeed: avgGenSpeed.toFixed(1),
      inFlight,
    };
  });

  const nf = new Intl.NumberFormat();
</script>

<div class="card flex flex-col gap-4">
  <!-- Summary Cards Row -->
  <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
    <div class="rounded-lg bg-surface p-3 border border-card-border-inner">
      <div class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Cache Hit Rate</div>
      <div class="mt-1 flex items-baseline gap-2">
        <span class="text-2xl font-bold text-emerald-500 dark:text-emerald-400">{stats.cacheHitRate}%</span>
        {#if stats.totalCachedTokens > 0}
          <span class="text-xs text-gray-500 dark:text-gray-400">
            {nf.format(stats.totalCachedTokens)} / {nf.format(stats.totalInputTokens)} in
          </span>
        {/if}
      </div>
    </div>

    <div class="rounded-lg bg-surface p-3 border border-card-border-inner">
      <div class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Avg Gen Speed</div>
      <div class="mt-1 flex items-baseline gap-2">
        <span class="text-2xl font-bold text-blue-500 dark:text-blue-400">{stats.avgGenSpeed}</span>
        <span class="text-xs text-gray-500 dark:text-gray-400">tok/s</span>
      </div>
    </div>

    <div class="rounded-lg bg-surface p-3 border border-card-border-inner">
      <div class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Avg Duration</div>
      <div class="mt-1 flex items-baseline gap-2">
        <span class="text-2xl font-bold text-purple-500 dark:text-purple-400">{stats.avgDurationMs}</span>
        <span class="text-xs text-gray-500 dark:text-gray-400">ms/req</span>
      </div>
    </div>

    <div class="rounded-lg bg-surface p-3 border border-card-border-inner">
      <div class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Requests</div>
      <div class="mt-1 flex items-baseline gap-2">
        <span class="text-2xl font-bold text-gray-800 dark:text-white">{nf.format(stats.totalRequests)}</span>
        {#if stats.inFlight > 0}
          <span class="text-xs text-amber-500">+{stats.inFlight} pending</span>
        {/if}
      </div>
    </div>
  </div>

  <!-- Token Breakdown Table -->
  <div class="rounded-lg overflow-hidden border border-card-border-inner">
    <table class="min-w-full">
      <thead>
        <tr class="bg-secondary text-left">
          <th class="px-4 py-2.5 text-xs font-semibold uppercase tracking-wider text-txtmain w-32">Requests</th>
          <th class="px-4 py-2.5 text-xs font-semibold uppercase tracking-wider text-txtmain border-l border-card-border-inner">New Input Tokens</th>
          <th class="px-4 py-2.5 text-xs font-semibold uppercase tracking-wider text-txtmain border-l border-card-border-inner">Cached Tokens</th>
          <th class="px-4 py-2.5 text-xs font-semibold uppercase tracking-wider text-txtmain border-l border-card-border-inner">Generated Output</th>
        </tr>
      </thead>

      <tbody class="divide-y divide-card-border-inner">
        <!-- Counts row -->
        <tr class="bg-surface">
          <td class="px-4 py-3">
            <div class="text-sm font-semibold text-gray-900 dark:text-white">{nf.format(stats.totalRequests)} completed</div>
            {#if stats.inFlight > 0}
              <div class="text-xs text-amber-500">{stats.inFlight} in-flight</div>
            {/if}
          </td>

          <td class="px-4 py-3 border-l border-card-border-inner">
            <div class="flex flex-col gap-0.5">
              <span class="text-base font-bold">{nf.format(stats.totalNewInputTokens)}</span>
              <span class="text-xs text-gray-500 dark:text-gray-400">{pct(stats.totalNewInputTokens, stats.totalInputTokens)} of input</span>
            </div>
          </td>

          <td class="px-4 py-3 border-l border-card-border-inner">
            <div class="flex flex-col gap-0.5">
              <span class="text-base font-bold text-blue-400 dark:text-blue-300">{nf.format(stats.totalCachedTokens)}</span>
              <span class="text-xs text-gray-500 dark:text-gray-400">{pct(stats.totalCachedTokens, stats.totalInputTokens)} of input</span>
            </div>
          </td>

          <td class="px-4 py-3 border-l border-card-border-inner">
            <div class="flex flex-col gap-0.5">
              <span class="text-base font-bold">{nf.format(stats.totalOutputTokens)}</span>
              <span class="text-xs text-gray-500 dark:text-gray-400">{pct(stats.totalOutputTokens, stats.totalInputTokens)} of input</span>
            </div>
          </td>
        </tr>

        <!-- Totals row -->
        <tr class="bg-secondary/50">
          <td class="px-4 py-3 text-xs font-semibold uppercase tracking-wider text-txtmain">Total Input</td>
          <td class="px-4 py-3 border-l border-card-border-inner text-sm font-bold" colspan="2">
            {nf.format(stats.totalInputTokens)} tokens
          </td>
          <td class="px-4 py-3 border-l border-card-border-inner text-xs font-medium text-gray-500 dark:text-gray-400">
            output/input ratio: {pct(stats.totalOutputTokens, stats.totalInputTokens)}
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</div>
