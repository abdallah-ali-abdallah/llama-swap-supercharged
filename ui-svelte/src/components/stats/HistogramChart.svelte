<script lang="ts">
  import type { HistogramBin, PercentileSummary } from "../../lib/metricsStats";

  interface Props {
    title: string;
    bins: HistogramBin[];
    percentiles: PercentileSummary;
    unit?: string;
  }

  let { title, bins, percentiles, unit = "" }: Props = $props();

  const width = 720;
  const height = 240;
  const padding = { top: 18, right: 20, bottom: 40, left: 52 };
  const chartWidth = width - padding.left - padding.right;
  const chartHeight = height - padding.top - padding.bottom;

  let maxCount = $derived(Math.max(0, ...bins.map((bin) => bin.count)));
  let rangeStart = $derived(bins.length > 0 ? bins[0].start : 0);
  let rangeEnd = $derived(bins.length > 0 ? bins[bins.length - 1].end : 1);
  let range = $derived(rangeEnd === rangeStart ? 1 : rangeEnd - rangeStart);

  function xPos(value: number): number {
    return padding.left + ((value - rangeStart) / range) * chartWidth;
  }

  function formatNumber(value: number): string {
    if (value >= 1000) return value.toLocaleString(undefined, { maximumFractionDigits: 0 });
    if (value >= 100) return value.toFixed(0);
    if (value >= 10) return value.toFixed(1);
    return value.toFixed(2);
  }
</script>

<section class="rounded-lg border border-card-border bg-surface p-4 shadow-sm">
  <div class="mb-3 flex flex-wrap items-start justify-between gap-3">
    <h2 class="p-0 text-sm font-semibold text-txtmain">{title}</h2>
    <div class="flex gap-2 text-xs text-txtsecondary">
      <span>P50 <strong class="text-txtmain">{formatNumber(percentiles.p50)}</strong></span>
      <span>P95 <strong class="text-txtmain">{formatNumber(percentiles.p95)}</strong></span>
      <span>P99 <strong class="text-txtmain">{formatNumber(percentiles.p99)}</strong></span>
    </div>
  </div>

  {#if bins.length > 0}
    <svg viewBox="0 0 {width} {height}" class="h-auto w-full" preserveAspectRatio="none" role="img" aria-label={title}>
      {#each [0, 1, 2, 3] as tick}
        {@const y = padding.top + (chartHeight / 3) * tick}
        {@const value = maxCount - (maxCount / 3) * tick}
        <line x1={padding.left} x2={width - padding.right} y1={y} y2={y} stroke="currentColor" class="text-card-border-inner" />
        <text x={padding.left - 8} y={y + 4} text-anchor="end" class="fill-txtsecondary text-[11px]">{formatNumber(value)}</text>
      {/each}

      {#each bins as bin}
        {@const x = xPos(bin.start)}
        {@const barWidth = Math.max(1, xPos(bin.end) - x - 2)}
        {@const barHeight = maxCount > 0 ? (bin.count / maxCount) * chartHeight : 0}
        {@const y = padding.top + chartHeight - barHeight}
        <rect x={x} y={y} width={barWidth} height={barHeight} rx="1.5" fill="#5794f2" opacity="0.82">
          <title>{`${formatNumber(bin.start)} - ${formatNumber(bin.end)}${unit ? ` ${unit}` : ""}: ${bin.count}`}</title>
        </rect>
      {/each}

      {#each [
        { label: "P50", value: percentiles.p50, color: "#73bf69" },
        { label: "P95", value: percentiles.p95, color: "#ff9830" },
        { label: "P99", value: percentiles.p99, color: "#f2495c" },
      ] as marker}
        {@const x = xPos(marker.value)}
        <line x1={x} x2={x} y1={padding.top} y2={padding.top + chartHeight} stroke={marker.color} stroke-width="2" stroke-dasharray="5 4" />
        <text x={x + 5} y={padding.top + 12} class="text-[11px] font-semibold" fill={marker.color}>{marker.label}</text>
      {/each}

      <text x={padding.left} y={height - 12} text-anchor="start" class="fill-txtsecondary text-[11px]">
        {formatNumber(rangeStart)}{unit ? ` ${unit}` : ""}
      </text>
      <text x={width - padding.right} y={height - 12} text-anchor="end" class="fill-txtsecondary text-[11px]">
        {formatNumber(rangeEnd)}{unit ? ` ${unit}` : ""}
      </text>
    </svg>
  {:else}
    <div class="flex h-[190px] items-center justify-center rounded-md border border-dashed border-card-border-inner text-sm text-txtsecondary">
      No speed distribution yet
    </div>
  {/if}
</section>

