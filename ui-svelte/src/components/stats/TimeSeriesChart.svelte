<script lang="ts">
  import type { ChartSeries } from "../../lib/metricsStats";

  interface Props {
    title: string;
    series: ChartSeries[];
    unit?: string;
    height?: number;
  }

  let { title, series, unit = "", height = 220 }: Props = $props();

  const width = 720;
  const padding = { top: 22, right: 18, bottom: 34, left: 54 };
  const chartWidth = width - padding.left - padding.right;
  const chartHeight = $derived(height - padding.top - padding.bottom);

  let validPoints = $derived(series.flatMap((item) => item.points.filter((point) => point.y !== null)));
  let hasData = $derived(validPoints.length > 0);
  let xMin = $derived(hasData ? Math.min(...validPoints.map((point) => point.x)) : 0);
  let xMax = $derived(hasData ? Math.max(...validPoints.map((point) => point.x)) : 1);
  let yMin = $derived(hasData ? Math.min(0, ...validPoints.map((point) => point.y || 0)) : 0);
  let yMax = $derived(hasData ? Math.max(...validPoints.map((point) => point.y || 0)) : 1);
  let yRange = $derived(yMax === yMin ? 1 : yMax - yMin);
  let xRange = $derived(xMax === xMin ? 1 : xMax - xMin);

  function xPos(x: number): number {
    return padding.left + ((x - xMin) / xRange) * chartWidth;
  }

  function yPos(y: number): number {
    return padding.top + chartHeight - ((y - yMin) / yRange) * chartHeight;
  }

  function linePath(item: ChartSeries): string {
    const points = item.points.filter((point) => point.y !== null);
    if (points.length === 0) return "";

    return points
      .map((point, index) => {
        const command = index === 0 ? "M" : "L";
        return `${command} ${xPos(point.x).toFixed(2)} ${yPos(point.y || 0).toFixed(2)}`;
      })
      .join(" ");
  }

  function areaPath(item: ChartSeries): string {
    const points = item.points.filter((point) => point.y !== null);
    if (points.length < 2) return "";

    const line = linePath(item);
    const last = points[points.length - 1];
    const first = points[0];
    const baseline = yPos(yMin);

    return `${line} L ${xPos(last.x).toFixed(2)} ${baseline.toFixed(2)} L ${xPos(first.x).toFixed(2)} ${baseline.toFixed(2)} Z`;
  }

  function formatNumber(value: number): string {
    if (value >= 1000) return value.toLocaleString(undefined, { maximumFractionDigits: 0 });
    if (value >= 100) return value.toFixed(0);
    if (value >= 10) return value.toFixed(1);
    return value.toFixed(2);
  }

  function formatTime(value: number): string {
    if (value < 1_000_000_000_000) return String(value);
    return new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit", second: "2-digit" }).format(new Date(value));
  }
</script>

<section class="rounded-lg border border-card-border bg-surface p-4 shadow-sm">
  <div class="mb-3 flex items-start justify-between gap-3">
    <h2 class="p-0 text-sm font-semibold text-txtmain">{title}</h2>
    <div class="flex flex-wrap justify-end gap-x-3 gap-y-1 text-xs text-txtsecondary">
      {#each series as item (item.label)}
        <span class="inline-flex items-center gap-1.5">
          <span class="h-2 w-2 rounded-full" style:background={item.color}></span>
          {item.label}
        </span>
      {/each}
    </div>
  </div>

  {#if hasData}
    <svg viewBox="0 0 {width} {height}" class="h-auto w-full overflow-visible" preserveAspectRatio="none" role="img" aria-label={title}>
      {#each [0, 1, 2, 3] as tick}
        {@const y = padding.top + (chartHeight / 3) * tick}
        {@const value = yMax - (yRange / 3) * tick}
        <line x1={padding.left} x2={width - padding.right} y1={y} y2={y} stroke="currentColor" class="text-card-border-inner" />
        <text x={padding.left - 8} y={y + 4} text-anchor="end" class="fill-txtsecondary text-[11px]">
          {formatNumber(value)}
        </text>
      {/each}

      {#each [0, 1, 2, 3] as tick}
        {@const x = padding.left + (chartWidth / 3) * tick}
        {@const value = xMin + (xRange / 3) * tick}
        <line x1={x} x2={x} y1={padding.top} y2={padding.top + chartHeight} stroke="currentColor" class="text-card-border-inner" opacity="0.55" />
        <text x={x} y={height - 10} text-anchor={tick === 0 ? "start" : tick === 3 ? "end" : "middle"} class="fill-txtsecondary text-[11px]">
          {formatTime(value)}
        </text>
      {/each}

      {#each series as item (item.label)}
        {@const area = areaPath(item)}
        {@const path = linePath(item)}
        {#if area}
          <path d={area} fill={item.color} opacity="0.12" />
        {/if}
        {#if path}
          <path d={path} fill="none" stroke={item.color} stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
        {/if}
        {#each item.points.filter((point) => point.y !== null).slice(-36) as point}
          <circle cx={xPos(point.x)} cy={yPos(point.y || 0)} r="3" fill={item.color} opacity="0.85">
            <title>{`${item.label}: ${formatNumber(point.y || 0)}${unit ? ` ${unit}` : ""}`}</title>
          </circle>
        {/each}
      {/each}
    </svg>
  {:else}
    <div class="flex h-[180px] items-center justify-center rounded-md border border-dashed border-card-border-inner text-sm text-txtsecondary">
      No data yet
    </div>
  {/if}
</section>
