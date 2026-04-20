<script lang="ts">
  interface Props {
    title: string;
    value: string;
    unit?: string;
    subtext?: string;
    tone?: "green" | "blue" | "yellow" | "orange" | "purple" | "neutral";
  }

  let { title, value, unit = "", subtext = "", tone = "neutral" }: Props = $props();

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

</script>

<section class="rounded-lg border border-card-border bg-surface p-4 shadow-sm min-h-[128px]">
  <div class="flex items-start justify-between gap-3">
    <h2 class="p-0 text-xs font-semibold uppercase tracking-wider text-txtsecondary">{title}</h2>
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
