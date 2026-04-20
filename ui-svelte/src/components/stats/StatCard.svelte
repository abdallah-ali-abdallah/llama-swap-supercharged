<script lang="ts">
  interface Props {
    title: string;
    value: string;
    unit?: string;
    subtext?: string;
    trend?: number | null;
    tone?: "green" | "blue" | "yellow" | "orange" | "purple" | "neutral";
  }

  let { title, value, unit = "", subtext = "", trend = null, tone = "neutral" }: Props = $props();

  const toneClass = $derived(
    {
      green: "text-[#73bf69]",
      blue: "text-[#5794f2]",
      yellow: "text-[#f2cc0c]",
      orange: "text-[#ff9830]",
      purple: "text-[#b877d9]",
      neutral: "text-txtmain",
    }[tone],
  );

  let trendLabel = $derived.by(() => {
    if (trend === null) return "";
    const sign = trend > 0 ? "+" : "";
    return `${sign}${(trend * 100).toFixed(1)}%`;
  });
</script>

<section class="rounded-lg border border-card-border bg-surface p-4 shadow-sm min-h-[128px]">
  <div class="flex items-start justify-between gap-3">
    <h2 class="p-0 text-xs font-semibold uppercase tracking-wider text-txtsecondary">{title}</h2>
    {#if trendLabel}
      <span class="rounded-md bg-secondary px-2 py-0.5 text-xs font-semibold {trend && trend < 0 ? 'text-error' : 'text-success'}">
        {trendLabel}
      </span>
    {/if}
  </div>

  <div class="mt-5 flex items-baseline gap-2">
    <span class="text-4xl font-semibold leading-none {toneClass}">{value}</span>
    {#if unit}
      <span class="text-sm font-medium text-txtsecondary">{unit}</span>
    {/if}
  </div>

  {#if subtext}
    <p class="mt-3 text-xs leading-5 text-txtsecondary">{subtext}</p>
  {/if}
</section>

