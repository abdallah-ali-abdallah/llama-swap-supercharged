<script lang="ts">
  import type { Model, ModelConfiguration } from "../lib/types";
  import {
    LLAMA_SERVER_OPTION_SOURCE,
    getOptionSourceLabel,
    interpretLlamaServerCommand,
    type ConfigHighlight,
    type ModelConfigInterpretation,
  } from "../lib/llamaServerOptions";

  interface Props {
    model: Model | null;
    configuration: ModelConfiguration | null;
    open: boolean;
    loading: boolean;
    onclose: () => void;
  }

  let { model, configuration, open, loading, onclose }: Props = $props();

  let dialogEl: HTMLDialogElement | undefined = $state();
  let activeTab: "interpreted" | "yaml" = $state("interpreted");
  let copied = $state(false);

  $effect(() => {
    if (open && dialogEl) {
      dialogEl.showModal();
    } else if (!open && dialogEl) {
      dialogEl.close();
    }
  });

  $effect(() => {
    if (open) {
      activeTab = "interpreted";
      copied = false;
    }
  });

  let interpretation: ModelConfigInterpretation | null = $derived.by(() => {
    if (!configuration?.cmd) return null;
    return interpretLlamaServerCommand(configuration.cmd);
  });

  async function copyYaml(): Promise<void> {
    if (!configuration?.yaml) return;
    try {
      await navigator.clipboard.writeText(configuration.yaml);
      copied = true;
      setTimeout(() => (copied = false), 1500);
    } catch {
      copied = false;
    }
  }

  function handleDialogClose(): void {
    onclose();
  }

  function highlightClass(highlight: ConfigHighlight): string {
    if (highlight.tone === "good") return "border-success/50 bg-success/10";
    if (highlight.tone === "warning") return "border-warning/60 bg-warning/10";
    return "border-card-border bg-background";
  }

  function displayValue(value: string | null): string {
    return value === null || value === "" ? "enabled" : value;
  }
</script>

<dialog
  bind:this={dialogEl}
  onclose={handleDialogClose}
  class="bg-surface text-txtmain rounded-lg shadow-xl max-w-6xl w-[calc(100vw-2rem)] max-h-[90vh] p-0 backdrop:bg-black/50 m-auto"
>
  <div class="flex max-h-[90vh] flex-col">
    <div class="flex items-start justify-between gap-4 border-b border-card-border p-4">
      <div class="min-w-0">
        <h2 class="pb-0 text-xl font-bold">Model configuration</h2>
        {#if model}
          <p class="mt-1 truncate font-mono text-sm text-txtsecondary">{model.id}</p>
        {/if}
      </div>
      <button
        onclick={() => dialogEl?.close()}
        class="text-2xl leading-none text-txtsecondary hover:text-txtmain"
        aria-label="Close model configuration"
      >
        &times;
      </button>
    </div>

    <div class="flex items-center justify-between gap-3 border-b border-card-border px-4 py-3">
      <div class="flex gap-1">
        <button class="tab-btn" class:tab-btn-active={activeTab === "interpreted"} onclick={() => (activeTab = "interpreted")}>
          Interpreted
        </button>
        <button class="tab-btn" class:tab-btn-active={activeTab === "yaml"} onclick={() => (activeTab = "yaml")}>
          Raw YAML
        </button>
      </div>
      {#if activeTab === "yaml" && configuration?.yaml}
        <button class="btn btn--sm" onclick={copyYaml}>{copied ? "Copied" : "Copy"}</button>
      {/if}
    </div>

    <div class="flex-1 overflow-y-auto p-4">
      {#if loading}
        <div class="grid min-h-64 place-items-center text-txtsecondary">Loading...</div>
      {:else if !configuration}
        <div class="grid min-h-64 place-items-center text-txtsecondary">Configuration unavailable</div>
      {:else if activeTab === "yaml"}
        <pre class="max-h-[65vh] overflow-auto rounded-md border border-card-border bg-background p-4 text-sm leading-relaxed"><code>{configuration.yaml || "No YAML available"}</code></pre>
      {:else}
        <div class="space-y-5">
          <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <div class="rounded-md border border-card-border bg-background p-3">
              <p class="text-xs uppercase text-txtsecondary">Provider</p>
              <p class="mt-1 font-semibold">{interpretation?.provider === "llama.cpp" ? "llama.cpp" : "Unknown"}</p>
            </div>
            <div class="rounded-md border border-card-border bg-background p-3">
              <p class="text-xs uppercase text-txtsecondary">Executable</p>
              <p class="mt-1 truncate font-mono text-sm">{interpretation?.executable || "none"}</p>
            </div>
            <div class="rounded-md border border-card-border bg-background p-3">
              <p class="text-xs uppercase text-txtsecondary">Proxy</p>
              <p class="mt-1 truncate font-mono text-sm">{configuration.proxy || "none"}</p>
            </div>
            <div class="rounded-md border border-card-border bg-background p-3">
              <p class="text-xs uppercase text-txtsecondary">TTL</p>
              <p class="mt-1 font-semibold">{configuration.ttl === 0 ? "global/default" : `${configuration.ttl}s`}</p>
            </div>
          </div>

          {#if interpretation && interpretation.highlights.length > 0}
            <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              {#each interpretation.highlights as highlight (highlight.id)}
                <div class="rounded-md border p-3 {highlightClass(highlight)}">
                  <div class="flex items-start justify-between gap-3">
                    <p class="font-semibold">{highlight.label}</p>
                    <span class="rounded bg-surface px-2 py-0.5 font-mono text-xs">{highlight.value}</span>
                  </div>
                  <p class="mt-2 text-sm text-txtsecondary">{highlight.note}</p>
                </div>
              {/each}
            </div>
          {/if}

          <section>
            <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
              <h3 class="pb-0 text-lg">Command</h3>
              <a class="text-sm text-primary hover:underline" href={LLAMA_SERVER_OPTION_SOURCE} target="_blank" rel="noreferrer">
                {getOptionSourceLabel()}
              </a>
            </div>
            <pre class="overflow-auto rounded-md border border-card-border bg-background p-3 text-sm"><code>{configuration.cmd}</code></pre>
          </section>

          {#if interpretation && interpretation.categories.length > 0}
            {#each interpretation.categories as group (group.category)}
              <section>
                <h3 class="pb-2 text-lg">{group.category}</h3>
                <div class="overflow-hidden rounded-md border border-card-border">
                  <table class="w-full text-sm">
                    <thead class="bg-background text-left">
                      <tr>
                        <th class="w-52">Parameter</th>
                        <th class="w-44">Value</th>
                        <th>Interpretation</th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each group.options as parsed, index (`${parsed.flag}-${index}`)}
                        <tr class="border-t border-card-border">
                          <td class="font-mono">{parsed.flag}</td>
                          <td class="break-all font-mono">{displayValue(parsed.value)}</td>
                          <td>
                            <span class="font-semibold">{parsed.option.label}</span>
                            <span class="text-txtsecondary"> - {parsed.option.explanation}</span>
                          </td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                </div>
              </section>
            {/each}
          {:else}
            <div class="rounded-md border border-card-border bg-background p-4 text-txtsecondary">
              No llama.cpp parameters were found in this command.
            </div>
          {/if}

          {#if interpretation && interpretation.unknownOptions.length > 0}
            <section>
              <h3 class="pb-2 text-lg">Unrecognized</h3>
              <div class="overflow-hidden rounded-md border border-warning/60">
                <table class="w-full text-sm">
                  <thead class="bg-warning/10 text-left">
                    <tr>
                      <th class="w-52">Flag</th>
                      <th>Value</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each interpretation.unknownOptions as unknown, index (`${unknown.flag}-${index}`)}
                      <tr class="border-t border-card-border">
                        <td class="font-mono">{unknown.flag}</td>
                        <td class="break-all font-mono">{displayValue(unknown.value)}</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            </section>
          {/if}
        </div>
      {/if}
    </div>
  </div>
</dialog>

<style>
  .tab-btn {
    border: 1px solid transparent;
    border-radius: 6px;
    padding: 0.35rem 0.75rem;
    color: var(--color-txtsecondary);
  }

  .tab-btn-active {
    border-color: var(--color-card-border);
    background: var(--color-background);
    color: var(--color-txtmain);
  }
</style>
