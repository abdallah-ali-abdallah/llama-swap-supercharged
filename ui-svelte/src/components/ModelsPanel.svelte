<script lang="ts">
  import ModelConfigurationDialog from "./ModelConfigurationDialog.svelte";
  import { models, loadModel, unloadAllModels, unloadSingleModel, getModelConfiguration } from "../stores/api";
  import { isNarrow } from "../stores/theme";
  import { persistentStore } from "../stores/persistent";
  import type { Model, ModelConfiguration, ModelMemoryComponent, ModelMemorySnapshot } from "../lib/types";

  let isUnloading = $state(false);
  let menuOpen = $state(false);
  let selectedModel: Model | null = $state(null);
  let selectedConfiguration: ModelConfiguration | null = $state(null);
  let configDialogOpen = $state(false);
  let configLoading = $state(false);
  let expandedMemory = $state<Set<string>>(new Set());

  const showUnlistedStore = persistentStore<boolean>("showUnlisted", true);
  const showIdorNameStore = persistentStore<"id" | "name">("showIdorName", "id");

  let filteredModels = $derived.by(() => {
    const filtered = $models.filter((model) => $showUnlistedStore || !model.unlisted);
    const peerModels = filtered.filter((m) => m.peerID);

    // Group peer models by peerID
    const grouped = peerModels.reduce(
      (acc, model) => {
        const peerId = model.peerID || "unknown";
        if (!acc[peerId]) acc[peerId] = [];
        acc[peerId].push(model);
        return acc;
      },
      {} as Record<string, Model[]>
    );

    return {
      regularModels: filtered.filter((m) => !m.peerID),
      peerModelsByPeerId: grouped,
    };
  });

  async function handleUnloadAllModels(): Promise<void> {
    isUnloading = true;
    try {
      await unloadAllModels();
    } catch (e) {
      console.error(e);
    } finally {
      setTimeout(() => (isUnloading = false), 1000);
    }
  }

  function toggleIdorName(): void {
    showIdorNameStore.update((prev) => (prev === "name" ? "id" : "name"));
  }

  function toggleShowUnlisted(): void {
    showUnlistedStore.update((prev) => !prev);
  }

  function getModelDisplay(model: Model): string {
    return $showIdorNameStore === "id" ? model.id : (model.name || model.id);
  }

  async function viewConfiguration(model: Model): Promise<void> {
    selectedModel = model;
    selectedConfiguration = null;
    configDialogOpen = true;
    configLoading = true;
    try {
      selectedConfiguration = await getModelConfiguration(model.id);
    } finally {
      configLoading = false;
    }
  }

  function closeConfigurationDialog(): void {
    configDialogOpen = false;
  }

  function toggleMemory(modelID: string): void {
    const next = new Set(expandedMemory);
    if (next.has(modelID)) {
      next.delete(modelID);
    } else {
      next.add(modelID);
    }
    expandedMemory = next;
  }

  function formatBytes(bytes?: number): string {
    if (!bytes || bytes <= 0) return "-";
    const units = ["B", "KiB", "MiB", "GiB", "TiB"];
    let value = bytes;
    let unitIndex = 0;
    while (value >= 1024 && unitIndex < units.length - 1) {
      value /= 1024;
      unitIndex += 1;
    }
    const digits = value >= 100 || unitIndex === 0 ? 0 : value >= 10 ? 1 : 2;
    return `${value.toFixed(digits)} ${units[unitIndex]}`;
  }

  function sumComponents(memory: ModelMemorySnapshot | undefined, key: keyof Pick<ModelMemoryComponent, "model_bytes" | "kv_bytes" | "compute_bytes" | "output_bytes">): number {
    if (!memory) return 0;
    return [...(memory.devices ?? []), ...(memory.host ?? []), ...(memory.unknown ?? [])].reduce((sum, component) => sum + (component[key] ?? 0), 0);
  }

  function memorySummary(memory: ModelMemorySnapshot | undefined): string {
    if (!memory) return "";
    const parts = [
      ["model", sumComponents(memory, "model_bytes")],
      ["KV", sumComponents(memory, "kv_bytes")],
      ["compute", sumComponents(memory, "compute_bytes")],
    ].filter(([, bytes]) => Number(bytes) > 0);
    return parts.map(([label, bytes]) => `${label} ${formatBytes(Number(bytes))}`).join(" | ");
  }

  function hasMemoryDetails(model: Model): boolean {
    return Boolean(model.memory && ((model.memory.devices?.length ?? 0) > 0 || (model.memory.host?.length ?? 0) > 0 || (model.memory.unknown?.length ?? 0) > 0));
  }

  function memoryRows(memory: ModelMemorySnapshot): Array<{ label: string; rows: ModelMemoryComponent[] }> {
    return [
      { label: "Device", rows: memory.devices ?? [] },
      { label: "Host", rows: memory.host ?? [] },
      { label: "Other", rows: memory.unknown ?? [] },
    ].filter((group) => group.rows.length > 0);
  }
</script>

<div class="card h-full flex flex-col">
  <div class="shrink-0">
    <div class="flex justify-between items-baseline">
      <h2 class={$isNarrow ? "text-xl" : ""}>Models</h2>
      {#if $isNarrow}
        <div class="relative">
          <button class="btn text-base flex items-center gap-2 py-1" onclick={() => (menuOpen = !menuOpen)} aria-label="Toggle menu">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-5 h-5">
              <path fill-rule="evenodd" d="M3 6.75A.75.75 0 0 1 3.75 6h16.5a.75.75 0 0 1 0 1.5H3.75A.75.75 0 0 1 3 6.75ZM3 12a.75.75 0 0 1 .75-.75h16.5a.75.75 0 0 1 0 1.5H3.75A.75.75 0 0 1 3 12Zm0 5.25a.75.75 0 0 1 .75-.75h16.5a.75.75 0 0 1 0 1.5H3.75a.75.75 0 0 1-.75-.75Z" clip-rule="evenodd" />
            </svg>
          </button>
          {#if menuOpen}
            <div class="absolute right-0 mt-2 w-48 bg-surface border border-gray-200 dark:border-white/10 rounded shadow-lg z-20">
              <button
                class="w-full text-left px-4 py-2 hover:bg-secondary-hover flex items-center gap-2"
                onclick={() => { toggleIdorName(); menuOpen = false; }}
              >
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-5 h-5">
                  <path fill-rule="evenodd" d="M15.97 2.47a.75.75 0 0 1 1.06 0l4.5 4.5a.75.75 0 0 1 0 1.06l-4.5 4.5a.75.75 0 1 1-1.06-1.06l3.22-3.22H7.5a.75.75 0 0 1 0-1.5h11.69l-3.22-3.22a.75.75 0 0 1 0-1.06Zm-7.94 9a.75.75 0 0 1 0 1.06l-3.22 3.22H16.5a.75.75 0 0 1 0 1.5H4.81l3.22 3.22a.75.75 0 1 1-1.06 1.06l-4.5-4.5a.75.75 0 0 1 0-1.06l4.5-4.5a.75.75 0 0 1 1.06 0Z" clip-rule="evenodd" />
                </svg>
                {$showIdorNameStore === "id" ? "Show Name" : "Show ID"}
              </button>
              <button
                class="w-full text-left px-4 py-2 hover:bg-secondary-hover flex items-center gap-2"
                onclick={() => { toggleShowUnlisted(); menuOpen = false; }}
              >
                {#if $showUnlistedStore}
                  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-5 h-5">
                    <path d="M3.53 2.47a.75.75 0 0 0-1.06 1.06l18 18a.75.75 0 1 0 1.06-1.06l-18-18ZM22.676 12.553a11.249 11.249 0 0 1-2.631 4.31l-3.099-3.099a5.25 5.25 0 0 0-6.71-6.71L7.759 4.577a11.217 11.217 0 0 1 4.242-.827c4.97 0 9.185 3.223 10.675 7.69.12.362.12.752 0 1.113Z" />
                    <path d="M15.75 12c0 .18-.013.357-.037.53l-4.244-4.243A3.75 3.75 0 0 1 15.75 12ZM12.53 15.713l-4.243-4.244a3.75 3.75 0 0 0 4.244 4.243Z" />
                    <path d="M6.75 12c0-.619.107-1.213.304-1.764l-3.1-3.1a11.25 11.25 0 0 0-2.63 4.31c-.12.362-.12.752 0 1.114 1.489 4.467 5.704 7.69 10.675 7.69 1.5 0 2.933-.294 4.242-.827l-2.477-2.477A5.25 5.25 0 0 1 6.75 12Z" />
                  </svg>
                {:else}
                  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-5 h-5">
                    <path d="M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z" />
                    <path fill-rule="evenodd" d="M1.323 11.447C2.811 6.976 7.028 3.75 12.001 3.75c4.97 0 9.185 3.223 10.675 7.69.12.362.12.752 0 1.113-1.487 4.471-5.705 7.697-10.677 7.697-4.97 0-9.186-3.223-10.675-7.69a1.762 1.762 0 0 1 0-1.113ZM17.25 12a5.25 5.25 0 1 1-10.5 0 5.25 5.25 0 0 1 10.5 0Z" clip-rule="evenodd" />
                  </svg>
                {/if}
                {$showUnlistedStore ? "Hide Unlisted" : "Show Unlisted"}
              </button>
              <button
                class="w-full text-left px-4 py-2 hover:bg-secondary-hover flex items-center gap-2"
                onclick={() => { handleUnloadAllModels(); menuOpen = false; }}
                disabled={isUnloading}
              >
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-6 h-6">
                  <path fill-rule="evenodd" d="M12 2.25c-5.385 0-9.75 4.365-9.75 9.75s4.365 9.75 9.75 9.75 9.75-4.365 9.75-9.75S17.385 2.25 12 2.25Zm.53 5.47a.75.75 0 0 0-1.06 0l-3 3a.75.75 0 1 0 1.06 1.06l1.72-1.72v5.69a.75.75 0 0 0 1.5 0v-5.69l1.72 1.72a.75.75 0 1 0 1.06-1.06l-3-3Z" clip-rule="evenodd" />
                </svg>
                {isUnloading ? "Unloading..." : "Unload All"}
              </button>
            </div>
          {/if}
        </div>
      {/if}
    </div>
    {#if !$isNarrow}
      <div class="flex justify-between">
        <div class="flex gap-2">
          <button class="btn text-base flex items-center gap-2" onclick={toggleIdorName} style="line-height: 1.2">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-5 h-5">
              <path fill-rule="evenodd" d="M15.97 2.47a.75.75 0 0 1 1.06 0l4.5 4.5a.75.75 0 0 1 0 1.06l-4.5 4.5a.75.75 0 1 1-1.06-1.06l3.22-3.22H7.5a.75.75 0 0 1 0-1.5h11.69l-3.22-3.22a.75.75 0 0 1 0-1.06Zm-7.94 9a.75.75 0 0 1 0 1.06l-3.22 3.22H16.5a.75.75 0 0 1 0 1.5H4.81l3.22 3.22a.75.75 0 1 1-1.06 1.06l-4.5-4.5a.75.75 0 0 1 0-1.06l4.5-4.5a.75.75 0 0 1 1.06 0Z" clip-rule="evenodd" />
            </svg>
            {$showIdorNameStore === "id" ? "ID" : "Name"}
          </button>

          <button class="btn text-base flex items-center gap-2" onclick={toggleShowUnlisted} style="line-height: 1.2">
            {#if $showUnlistedStore}
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-5 h-5">
                <path d="M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z" />
                <path fill-rule="evenodd" d="M1.323 11.447C2.811 6.976 7.028 3.75 12.001 3.75c4.97 0 9.185 3.223 10.675 7.69.12.362.12.752 0 1.113-1.487 4.471-5.705 7.697-10.677 7.697-4.97 0-9.186-3.223-10.675-7.69a1.762 1.762 0 0 1 0-1.113ZM17.25 12a5.25 5.25 0 1 1-10.5 0 5.25 5.25 0 0 1 10.5 0Z" clip-rule="evenodd" />
              </svg>
            {:else}
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-5 h-5">
                <path d="M3.53 2.47a.75.75 0 0 0-1.06 1.06l18 18a.75.75 0 1 0 1.06-1.06l-18-18ZM22.676 12.553a11.249 11.249 0 0 1-2.631 4.31l-3.099-3.099a5.25 5.25 0 0 0-6.71-6.71L7.759 4.577a11.217 11.217 0 0 1 4.242-.827c4.97 0 9.185 3.223 10.675 7.69.12.362.12.752 0 1.113Z" />
                <path d="M15.75 12c0 .18-.013.357-.037.53l-4.244-4.243A3.75 3.75 0 0 1 15.75 12ZM12.53 15.713l-4.243-4.244a3.75 3.75 0 0 0 4.244 4.243Z" />
                <path d="M6.75 12c0-.619.107-1.213.304-1.764l-3.1-3.1a11.25 11.25 0 0 0-2.63 4.31c-.12.362-.12.752 0 1.114 1.489 4.467 5.704 7.69 10.675 7.69 1.5 0 2.933-.294 4.242-.827l-2.477-2.477A5.25 5.25 0 0 1 6.75 12Z" />
              </svg>
            {/if}
            unlisted
          </button>
        </div>
        <button class="btn text-base flex items-center gap-2" onclick={handleUnloadAllModels} disabled={isUnloading}>
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" class="w-6 h-6">
            <path fill-rule="evenodd" d="M12 2.25c-5.385 0-9.75 4.365-9.75 9.75s4.365 9.75 9.75 9.75 9.75-4.365 9.75-9.75S17.385 2.25 12 2.25Zm.53 5.47a.75.75 0 0 0-1.06 0l-3 3a.75.75 0 1 0 1.06 1.06l1.72-1.72v5.69a.75.75 0 0 0 1.5 0v-5.69l1.72 1.72a.75.75 0 1 0 1.06-1.06l-3-3Z" clip-rule="evenodd" />
          </svg>
          {isUnloading ? "Unloading..." : "Unload All"}
        </button>
      </div>
    {/if}
  </div>

  <div class="flex-1 overflow-y-auto">
    <table class="w-full">
      <thead class="sticky top-0 bg-card z-10">
        <tr class="text-left border-b border-gray-200 dark:border-white/10 bg-surface">
          <th>{$showIdorNameStore === "id" ? "Model ID" : "Name"}</th>
          <th></th>
          {#if !$isNarrow}
            <th>Memory</th>
          {/if}
          <th>State</th>
        </tr>
      </thead>
      <tbody>
        {#each filteredModels.regularModels as model (model.id)}
          <tr class="border-b hover:bg-secondary-hover border-gray-200">
            <td class={model.unlisted ? "text-txtsecondary" : ""}>
              <a href="/upstream/{model.id}/" class="font-semibold" target="_blank">
                {getModelDisplay(model)}
              </a>
              {#if model.description}
                <p class={model.unlisted ? "text-opacity-70" : ""}><em>{model.description}</em></p>
              {/if}
              {#if model.aliases && model.aliases.length > 0}
                <p class="text-xs text-txtsecondary">Aliases: {model.aliases.join(", ")}</p>
              {/if}
              {#if $isNarrow}
                <div class="mt-2 text-xs text-txtsecondary">
                  <span class="font-semibold text-txtmain">Memory:</span>
                  {#if model.memory}
                    <span>{formatBytes(model.memory.device_total_bytes)}</span>
                    {#if memorySummary(model.memory)}
                      <span class="block">{memorySummary(model.memory)}</span>
                    {/if}
                    {#if hasMemoryDetails(model)}
                      <button class="btn btn--sm mt-1" onclick={() => toggleMemory(model.id)}>
                        {expandedMemory.has(model.id) ? "Hide memory" : "Show memory"}
                      </button>
                    {/if}
                  {:else}
                    <span>-</span>
                  {/if}
                </div>
              {/if}
            </td>
            <td class="w-48">
              <div class="flex justify-end gap-2">
                <button class="btn btn--sm whitespace-nowrap" onclick={() => viewConfiguration(model)}>View configurations</button>
                {#if model.state === "stopped"}
                  <button class="btn btn--sm" onclick={() => loadModel(model.id)}>Load</button>
                {:else}
                  <button class="btn btn--sm" onclick={() => unloadSingleModel(model.id)} disabled={model.state !== "ready"}>Unload</button>
                {/if}
              </div>
            </td>
            {#if !$isNarrow}
              <td class="w-56">
                {#if model.memory}
                  <div class="text-sm">
                    <div class="flex items-center gap-2">
                      <span class="font-semibold">{formatBytes(model.memory.device_total_bytes)}</span>
                      {#if hasMemoryDetails(model)}
                        <button class="btn btn--sm" aria-label="Toggle memory details for {model.id}" onclick={() => toggleMemory(model.id)}>
                          {expandedMemory.has(model.id) ? "Hide" : "Details"}
                        </button>
                      {/if}
                    </div>
                    {#if memorySummary(model.memory)}
                      <p class="text-xs text-txtsecondary">{memorySummary(model.memory)}</p>
                    {/if}
                  </div>
                {:else}
                  <span class="text-txtsecondary">-</span>
                {/if}
              </td>
            {/if}
            <td class="w-20">
              <span class="w-16 text-center status status--{model.state}">{model.state}</span>
            </td>
          </tr>
          {#if expandedMemory.has(model.id) && model.memory}
            <tr class="border-b border-gray-200 bg-secondary/40">
              <td colspan={$isNarrow ? 3 : 4}>
                <div class="py-3 text-sm">
                  {#each memoryRows(model.memory) as group (group.label)}
                    <div class="mb-2 last:mb-0">
                      <p class="text-xs font-semibold uppercase text-txtsecondary">{group.label}</p>
                      {#each group.rows as row (row.name)}
                        <div class="grid gap-2 py-1 text-xs md:grid-cols-[minmax(10rem,1fr)_repeat(5,minmax(4rem,auto))]">
                          <span class="font-semibold text-txtmain">{row.name}</span>
                          <span>model {formatBytes(row.model_bytes)}</span>
                          <span>KV {formatBytes(row.kv_bytes)}</span>
                          <span>compute {formatBytes(row.compute_bytes)}</span>
                          <span>output {formatBytes(row.output_bytes)}</span>
                          <span>total {formatBytes(row.tracked_bytes)}</span>
                          {#if row.unaccounted_bytes}
                            <span class="text-txtsecondary">runtime {formatBytes(row.unaccounted_bytes)}</span>
                          {/if}
                        </div>
                      {/each}
                    </div>
                  {/each}
                  {#if model.memory.host_total_bytes > 0}
                    <p class="mt-2 text-xs text-txtsecondary">Host tracked: {formatBytes(model.memory.host_total_bytes)}</p>
                  {/if}
                </div>
              </td>
            </tr>
          {/if}
        {/each}
      </tbody>
    </table>

    {#if Object.keys(filteredModels.peerModelsByPeerId).length > 0}
      <h3 class="mt-8 mb-2">Peer Models</h3>
      {#each Object.entries(filteredModels.peerModelsByPeerId).sort(([a], [b]) => a.localeCompare(b)) as [peerId, peerModels] (peerId)}
        <div class="mb-4">
          <table class="w-full">
            <thead class="sticky top-0 bg-card z-10">
              <tr class="text-left border-b border-gray-200 dark:border-white/10 bg-surface">
                <th class="font-semibold">{peerId}</th>
              </tr>
            </thead>
            <tbody>
              {#each peerModels as model (model.id)}
                <tr class="border-b hover:bg-secondary-hover border-gray-200">
                  <td class="pl-8 {model.unlisted ? 'text-txtsecondary' : ''}">
                    <span>{model.id}</span>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/each}
    {/if}
  </div>
</div>

<ModelConfigurationDialog
  model={selectedModel}
  configuration={selectedConfiguration}
  open={configDialogOpen}
  loading={configLoading}
  onclose={closeConfigurationDialog}
/>
