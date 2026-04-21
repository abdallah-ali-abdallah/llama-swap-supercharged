<script lang="ts">
  import type { TokenTotals } from "../../lib/metricsStats";

  interface Props {
    title: string;
    tokens: TokenTotals;
  }

  let { title, tokens }: Props = $props();

  const nf = new Intl.NumberFormat();

  let segments = $derived([
    { label: "Unique processed token", value: tokens.newInput, color: "#73bf69" },
    { label: "Cached", value: tokens.cached, color: "#5794f2" },
    { label: "Generated", value: tokens.output, color: "#f2cc0c" },
  ]);

  function pct(value: number, total: number): string {
    return total > 0 ? `${((value / total) * 100).toFixed(1)}%` : "0.0%";
  }
</script>

<section class="rounded-lg border border-card-border bg-surface p-4 shadow-sm">
  <div class="mb-4 flex items-start justify-between gap-3">
    <h2 class="p-0 text-sm font-semibold text-txtmain">{title}</h2>
    <span class="text-xs font-semibold text-txtsecondary">{nf.format(tokens.total)} total</span>
  </div>

  <div class="h-5 overflow-hidden rounded-md bg-secondary">
    {#if tokens.total > 0}
      <div class="flex h-full w-full">
        {#each segments as segment}
          {#if segment.value > 0}
            <div style:width={pct(segment.value, tokens.total)} style:background={segment.color}>
              <span class="sr-only">{segment.label}: {nf.format(segment.value)}</span>
            </div>
          {/if}
        {/each}
      </div>
    {/if}
  </div>

  <div class="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-3">
    {#each segments as segment}
      <div class="rounded-md border border-card-border-inner bg-background/50 p-3">
        <div class="flex items-center gap-2 text-xs font-semibold text-txtsecondary">
          <span class="h-2.5 w-2.5 rounded-full" style:background={segment.color}></span>
          {segment.label}
        </div>
        <div class="mt-2 text-xl font-semibold text-txtmain">{nf.format(segment.value)}</div>
        <div class="mt-1 text-xs text-txtsecondary">{pct(segment.value, tokens.total)} of total</div>
      </div>
    {/each}
  </div>

  <div class="mt-4 rounded-md border border-card-border-inner bg-secondary/50 p-3 text-sm">
    <span class="font-semibold text-[#5794f2]">{(tokens.cacheHitRate * 100).toFixed(1)}%</span>
    <span class="text-txtsecondary"> cache hit rate across input tokens</span>
  </div>
</section>
