<script lang="ts">
  import type { ChartSeries } from "../../lib/metricsStats";
  import { chartAreaPath, chartPath, smoothedPointSegments, validPointSegments } from "../../lib/chartPaths";
  import type { CurveMode } from "../../lib/chartPaths";

  interface Props {
    title: string;
    series: ChartSeries[];
    unit?: string;
    height?: number;
    toggleableLegend?: boolean;
    valueFractionDigits?: number;
    curve?: CurveMode;
    smoothSamples?: boolean;
    sampleBucketSize?: number;
    sampleSmoothingWindow?: number;
  }

  interface HoverPoint {
    x: number;
    y: number;
    color: string;
    typeLabel: string;
    valueLabel: string;
    timeLabel: string;
    width: number;
  }

  let {
    title,
    series,
    unit = "",
    height = 220,
    toggleableLegend = false,
    valueFractionDigits,
    curve = "linear",
    smoothSamples = false,
    sampleBucketSize = 12,
    sampleSmoothingWindow = 5,
  }: Props = $props();

  const width = 720;
  const padding = { top: 22, right: 18, bottom: 34, left: 54 };
  const chartWidth = width - padding.left - padding.right;
  const chartHeight = $derived(height - padding.top - padding.bottom);

  let hiddenSeries = $state<string[]>([]);
  let hoverPoint = $state<HoverPoint | null>(null);
  let visibleSeries = $derived(series.filter((item) => !hiddenSeries.includes(item.label)));
  let rawValidPoints = $derived(visibleSeries.flatMap((item) => item.points.filter((point) => point.y !== null)));
  let hasData = $derived(rawValidPoints.length > 0);
  let hasVisibleSeries = $derived(visibleSeries.length > 0);
  let xMin = $derived(hasData ? Math.min(...rawValidPoints.map((point) => point.x)) : 0);
  let xMax = $derived(hasData ? Math.max(...rawValidPoints.map((point) => point.x)) : 1);
  let xRange = $derived(xMax === xMin ? 1 : xMax - xMin);
  let sampleBucketRange = $derived((sampleBucketSize / chartWidth) * xRange);
  let displaySegments = $derived(
    new Map(
      visibleSeries.map((item) => [
        item.label,
        smoothSamples
          ? smoothedPointSegments(item.points, { bucketSize: sampleBucketRange, windowSize: sampleSmoothingWindow })
          : validPointSegments(item.points),
      ]),
    ),
  );
  let displayPoints = $derived([...displaySegments.values()].flat(2));
  let yMin = $derived(displayPoints.length > 0 ? Math.min(0, ...displayPoints.map((point) => point.y)) : 0);
  let yMax = $derived(displayPoints.length > 0 ? Math.max(...displayPoints.map((point) => point.y)) : 1);
  let yRange = $derived(yMax === yMin ? 1 : yMax - yMin);

  function xPos(x: number): number {
    return padding.left + ((x - xMin) / xRange) * chartWidth;
  }

  function yPos(y: number): number {
    return padding.top + chartHeight - ((y - yMin) / yRange) * chartHeight;
  }

  function positionedSegments(item: ChartSeries): Array<Array<{ x: number; y: number }>> {
    return (displaySegments.get(item.label) || []).map((segment) =>
      segment.map((point) => ({
        x: xPos(point.x),
        y: yPos(point.y),
      })),
    );
  }

  function displayPointsFor(item: ChartSeries): Array<{ x: number; y: number }> {
    return (displaySegments.get(item.label) || []).flat();
  }

  function formatNumber(value: number): string {
    if (valueFractionDigits !== undefined) {
      return value.toLocaleString(undefined, {
        minimumFractionDigits: valueFractionDigits,
        maximumFractionDigits: valueFractionDigits,
      });
    }

    if (value >= 1000) return value.toLocaleString(undefined, { maximumFractionDigits: 0 });
    if (value >= 100) return value.toFixed(0);
    if (value >= 10) return value.toFixed(1);
    return value.toFixed(2);
  }

  function formatTime(value: number): string {
    if (value < 1_000_000_000_000) return String(value);
    return new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit", second: "2-digit" }).format(new Date(value));
  }

  function pointLabel(item: ChartSeries, value: number): string {
    return `${item.label}: ${formatNumber(value)}${unit ? ` ${unit}` : ""}`;
  }

  function pointTooltipWidth(label: string): number {
    return Math.max(96, Math.min(230, label.length * 7 + 18));
  }

  function pointTooltipHeight(): number {
    return 55;
  }

  function tooltipX(point: HoverPoint): number {
    return Math.max(padding.left, Math.min(width - padding.right - point.width, point.x + 10));
  }

  function tooltipY(point: HoverPoint): number {
    const tooltipHeight = pointTooltipHeight();
    const above = point.y - tooltipHeight - 10;
    if (above >= padding.top) return above;
    return Math.min(padding.top + chartHeight - tooltipHeight, point.y + 14);
  }

  function showPointTooltip(item: ChartSeries, point: { x: number; y: number }): void {
    const typeLabel = `Type: ${item.label}`;
    const valueLabel = `Value: ${formatNumber(point.y)}${unit ? ` ${unit}` : ""}`;
    const timeLabel = `Captured: ${formatTime(point.x)}`;
    hoverPoint = {
      x: xPos(point.x),
      y: yPos(point.y),
      color: item.color,
      typeLabel,
      valueLabel,
      timeLabel,
      width: Math.max(pointTooltipWidth(typeLabel), pointTooltipWidth(valueLabel), pointTooltipWidth(timeLabel)),
    };
  }

  function hidePointTooltip(): void {
    hoverPoint = null;
  }

  function setSeriesVisible(label: string, visible: boolean): void {
    if (visible) {
      hiddenSeries = hiddenSeries.filter((hiddenLabel) => hiddenLabel !== label);
      return;
    }

    if (!hiddenSeries.includes(label)) {
      hiddenSeries = [...hiddenSeries, label];
    }
  }

  $effect(() => {
    const labels = series.map((item) => item.label);
    const nextHiddenSeries = hiddenSeries.filter((label) => labels.includes(label));
    if (nextHiddenSeries.length !== hiddenSeries.length) {
      hiddenSeries = nextHiddenSeries;
    }
  });
</script>

<section class="rounded-lg border border-card-border bg-surface p-4 shadow-sm">
  <div class="mb-3 flex items-start justify-between gap-3">
    <h2 class="p-0 text-sm font-semibold text-txtmain">{title}</h2>
    <div class="flex flex-wrap justify-end gap-x-3 gap-y-1 text-xs text-txtsecondary">
      {#each series as item (item.label)}
        {@const visible = !hiddenSeries.includes(item.label)}
        {#if toggleableLegend}
          <label
            class={`inline-flex items-center gap-1.5 rounded-md px-2 py-1 transition ${
              visible ? "text-txtmain hover:bg-secondary" : "text-txtsecondary opacity-55 hover:bg-secondary hover:opacity-80"
            }`}
          >
            <input
              type="checkbox"
              checked={visible}
              onchange={(event) => setSeriesVisible(item.label, event.currentTarget.checked)}
              class="h-3.5 w-3.5 rounded border-card-border bg-surface"
              style={`accent-color: ${item.color}`}
            />
            <span class="h-2 w-2 rounded-full" style:background={item.color} style:opacity={visible ? 1 : 0.35}></span>
            <span class={visible ? "" : "line-through"}>{item.label}</span>
          </label>
        {:else}
          <span class="inline-flex items-center gap-1.5">
            <span class="h-2 w-2 rounded-full" style:background={item.color}></span>
            {item.label}
          </span>
        {/if}
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

      {#each visibleSeries as item (item.label)}
        {#each positionedSegments(item) as segment}
          {@const area = chartAreaPath(segment, yPos(yMin), curve)}
          {@const path = chartPath(segment, curve)}
          {#if area}
            <path d={area} fill={item.color} opacity="0.12" />
          {/if}
          {#if path}
            <path d={path} fill="none" stroke={item.color} stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
          {/if}
        {/each}
        {#each displayPointsFor(item).slice(-36) as point}
          <circle
            cx={xPos(point.x)}
            cy={yPos(point.y || 0)}
            r="3"
            fill={item.color}
            opacity="0.85"
            role="img"
            aria-label={pointLabel(item, point.y || 0)}
            class="cursor-crosshair"
            onpointerenter={() => showPointTooltip(item, point)}
            onpointerleave={hidePointTooltip}
          >
            <title>{pointLabel(item, point.y || 0)}</title>
          </circle>
        {/each}
      {/each}

      {#if hoverPoint}
        <g class="pointer-events-none" data-chart-tooltip>
          <line
            x1={hoverPoint.x}
            x2={hoverPoint.x}
            y1={padding.top}
            y2={padding.top + chartHeight}
            stroke={hoverPoint.color}
            stroke-dasharray="4 4"
            opacity="0.38"
          />
          <circle cx={hoverPoint.x} cy={hoverPoint.y} r="5.5" fill="none" stroke={hoverPoint.color} stroke-width="2" />
          <g transform={`translate(${tooltipX(hoverPoint).toFixed(2)} ${tooltipY(hoverPoint).toFixed(2)})`}>
            <rect width={hoverPoint.width} height={pointTooltipHeight()} rx="4" fill="var(--color-surface)" stroke="var(--color-card-border)" />
            <circle cx="10" cy="13" r="3" fill={hoverPoint.color} />
            <text x="18" y="16" class="fill-txtmain text-[11px] font-semibold">{hoverPoint.typeLabel}</text>
            <text x="18" y="31" class="fill-txtmain text-[11px]">{hoverPoint.valueLabel}</text>
            <text x="18" y="46" class="fill-txtsecondary text-[11px]">{hoverPoint.timeLabel}</text>
          </g>
        </g>
      {/if}
    </svg>
  {:else}
    <div class="flex h-[180px] items-center justify-center rounded-md border border-dashed border-card-border-inner text-sm text-txtsecondary">
      {hasVisibleSeries ? "No data yet" : "Select a series to show"}
    </div>
  {/if}
</section>
