<script lang="ts">
  import { isNarrow } from "../stores/theme";
  import ModelsPanel from "../components/ModelsPanel.svelte";
  import StatsPanel from "../components/StatsPanel.svelte";
  import ResizablePanels from "../components/ResizablePanels.svelte";

  let direction = $derived<"horizontal" | "vertical">($isNarrow ? "vertical" : "horizontal");
</script>

<ResizablePanels {direction} storageKey="models-panel-group">
  {#snippet leftPanel()}
    <ModelsPanel />
  {/snippet}
  {#snippet rightPanel()}
    <div class="flex flex-col h-full space-y-4 overflow-y-auto">
      {#if direction === "horizontal"}
        <StatsPanel />
      {:else}
        <StatsPanel />
      {/if}
    </div>
  {/snippet}
</ResizablePanels>
