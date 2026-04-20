<script lang="ts">
  import { onMount } from "svelte";
  import { getPersistenceSettings, updatePersistenceSettings } from "../stores/api";
  import type { PersistenceSettings } from "../stores/api";

  let settings = $state<PersistenceSettings | null>(null);
  let loading = $state(true);
  let saving = $state(false);
  let message = $state("");
  let error = $state("");

  async function loadSettings(): Promise<void> {
    loading = true;
    error = "";
    const result = await getPersistenceSettings();
    settings = result;
    if (!result) {
      error = "Unable to load persistence settings.";
    }
    loading = false;
  }

  async function saveSettings(): Promise<void> {
    if (!settings) return;
    saving = true;
    message = "";
    error = "";
    try {
      settings = await updatePersistenceSettings(settings);
      message = "Settings saved.";
    } catch (err) {
      error = err instanceof Error ? err.message : "Unable to save persistence settings.";
    } finally {
      saving = false;
    }
  }

  function toggleUsage(): void {
    if (!settings) return;
    settings.usage_metrics_persistence = !settings.usage_metrics_persistence;
  }

  function toggleLogging(): void {
    if (!settings) return;
    settings.logging_enabled = !settings.logging_enabled;
  }

  function toggleActivity(): void {
    if (!settings) return;
    settings.activity_persistence = !settings.activity_persistence;
  }

  function toggleCapture(): void {
    if (!settings) return;
    settings.activity_capture_persistence = !settings.activity_capture_persistence;
  }

  function toggleCaptureRedaction(): void {
    if (!settings) return;
    settings.capture_redact_headers = !settings.capture_redact_headers;
  }

  function toggleActivityField(field: keyof PersistenceSettings["activity_fields"]): void {
    if (!settings) return;
    settings.activity_fields[field] = !settings.activity_fields[field];
  }

  onMount(() => {
    loadSettings();
  });
</script>

<div class="mx-auto flex max-w-[1200px] flex-col gap-5 p-2">
  <header class="flex flex-wrap items-end justify-between gap-4">
    <div>
      <h1 class="p-0 text-2xl font-bold">Settings</h1>
      <p class="mt-1 text-sm text-txtsecondary">Control what llama-swap stores in SQLite for dashboard and Activity history.</p>
    </div>
    <button
      type="button"
      onclick={saveSettings}
      disabled={loading || saving || !settings || !settings.db_path.trim()}
      class="rounded-md border border-card-border bg-surface px-4 py-2 text-sm font-semibold text-txtmain transition hover:border-card-border-inner disabled:opacity-60"
    >
      {saving ? "Saving" : "Save"}
    </button>
  </header>

  {#if loading}
    <div class="rounded-lg border border-card-border bg-surface p-6 text-sm text-txtsecondary">Loading settings</div>
  {:else if settings}
    <section class="rounded-lg border border-card-border bg-surface p-4 shadow-sm">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 class="p-0 text-lg font-semibold text-txtmain">SQLite Persistence</h2>
          <p class="mt-1 text-sm text-txtsecondary">Settings are saved in SQLite and apply to new completed requests.</p>
        </div>
        <span class={`rounded-md px-3 py-1 text-sm font-semibold ${settings.sqlite_available ? "bg-[#73bf69]/20 text-[#9ce391]" : "bg-[#f2495c]/20 text-[#ff9aa5]"}`}>
          {settings.sqlite_available ? "Available" : "Unavailable"}
        </span>
      </div>

      <div class="grid grid-cols-1 gap-3 md:grid-cols-[minmax(0,1fr)_180px]">
        <label class="rounded-md border border-card-border-inner bg-secondary p-3">
          <div class="text-xs uppercase tracking-wider text-txtsecondary">SQLite database file</div>
          <input
            class="mt-2 w-full rounded-md border border-card-border bg-surface px-3 py-2 text-sm font-semibold text-txtmain outline-none focus:border-[#5794f2]"
            bind:value={settings.db_path}
            placeholder="/path/to/llama-swap-metrics.db"
          />
          <div class="mt-2 text-xs text-txtsecondary">A missing file is created when settings are saved.</div>
        </label>
        <div class="rounded-md border border-card-border-inner bg-secondary p-3">
          <div class="text-xs uppercase tracking-wider text-txtsecondary">Retention</div>
          <div class="mt-1 text-sm font-semibold text-txtmain">{settings.retention_days} days</div>
        </div>
      </div>
    </section>

    <section class="rounded-lg border border-card-border bg-surface p-4 shadow-sm">
      <h2 class="p-0 text-lg font-semibold text-txtmain">Logging</h2>
      <div class="mt-4 grid grid-cols-1 gap-3 md:grid-cols-2">
        <label class="flex min-h-[96px] cursor-pointer flex-col gap-3 rounded-md border border-card-border-inner bg-secondary p-4">
          <div class="flex items-center justify-between gap-3">
            <span class="font-semibold text-txtmain">Enable logging</span>
            <input type="checkbox" checked={settings.logging_enabled} onchange={toggleLogging} />
          </div>
          <span class="text-sm text-txtsecondary">Controls proxy and upstream log collection, streaming, and log history.</span>
        </label>
        <div class="rounded-md border border-card-border-inner bg-secondary p-4 text-sm text-txtsecondary">
          {settings.logging_enabled ? "Logs are being collected." : "Logging is fully disabled and existing buffered logs are cleared."}
        </div>
      </div>
    </section>

    <section class="rounded-lg border border-card-border bg-surface p-4 shadow-sm">
      <h2 class="p-0 text-lg font-semibold text-txtmain">Saving Controls</h2>
      <div class="mt-4 grid grid-cols-1 gap-3 md:grid-cols-3">
        <label class="flex min-h-[112px] cursor-pointer flex-col gap-3 rounded-md border border-card-border-inner bg-secondary p-4">
          <div class="flex items-center justify-between gap-3">
            <span class="font-semibold text-txtmain">Dashboard usage metrics</span>
            <input type="checkbox" checked={settings.usage_metrics_persistence} onchange={toggleUsage} disabled={!settings.sqlite_available} />
          </div>
          <span class="text-sm text-txtsecondary">Stores token totals, speed, duration, and per-model usage for Dashboard historical ranges.</span>
        </label>

        <label class="flex min-h-[112px] cursor-pointer flex-col gap-3 rounded-md border border-card-border-inner bg-secondary p-4">
          <div class="flex items-center justify-between gap-3">
            <span class="font-semibold text-txtmain">Activity rows</span>
            <input type="checkbox" checked={settings.activity_persistence} onchange={toggleActivity} disabled={!settings.sqlite_available} />
          </div>
          <span class="text-sm text-txtsecondary">Stores completed request rows for Activity historical ranges.</span>
        </label>

        <label class="flex min-h-[112px] cursor-pointer flex-col gap-3 rounded-md border border-card-border-inner bg-secondary p-4">
          <div class="flex items-center justify-between gap-3">
            <span class="font-semibold text-txtmain">Activity captures</span>
            <input
              type="checkbox"
              checked={settings.activity_capture_persistence}
              onchange={toggleCapture}
              disabled={!settings.sqlite_available || !settings.activity_persistence}
            />
          </div>
          <span class="text-sm text-txtsecondary">Stores request and response payloads for Activity View after restart.</span>
        </label>
      </div>
      <label class="mt-3 flex cursor-pointer items-center justify-between gap-3 rounded-md border border-card-border-inner bg-secondary p-4">
        <span>
          <span class="block font-semibold text-txtmain">Headers redacted before saving</span>
          <span class="mt-1 block text-sm text-txtsecondary">When disabled, persisted captures include full request and response headers.</span>
        </span>
        <input
          type="checkbox"
          checked={settings.capture_redact_headers}
          onchange={toggleCaptureRedaction}
          disabled={!settings.sqlite_available || !settings.activity_capture_persistence}
        />
      </label>
    </section>

    <section class="rounded-lg border border-card-border bg-surface p-4 shadow-sm">
      <h2 class="p-0 text-lg font-semibold text-txtmain">Activity Fields Saved</h2>
      <p class="mt-1 text-sm text-txtsecondary">These choices apply to future Activity rows. ID and timestamp are always stored.</p>
      <div class="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <label class="flex cursor-pointer items-center justify-between gap-3 rounded-md border border-card-border-inner bg-secondary p-4">
          <span class="font-semibold text-txtmain">Model</span>
          <input type="checkbox" checked={settings.activity_fields.model} onchange={() => toggleActivityField("model")} disabled={!settings.sqlite_available || !settings.activity_persistence} />
        </label>
        <label class="flex cursor-pointer items-center justify-between gap-3 rounded-md border border-card-border-inner bg-secondary p-4">
          <span class="font-semibold text-txtmain">Token counts</span>
          <input type="checkbox" checked={settings.activity_fields.tokens} onchange={() => toggleActivityField("tokens")} disabled={!settings.sqlite_available || !settings.activity_persistence} />
        </label>
        <label class="flex cursor-pointer items-center justify-between gap-3 rounded-md border border-card-border-inner bg-secondary p-4">
          <span class="font-semibold text-txtmain">Speeds</span>
          <input type="checkbox" checked={settings.activity_fields.speeds} onchange={() => toggleActivityField("speeds")} disabled={!settings.sqlite_available || !settings.activity_persistence} />
        </label>
        <label class="flex cursor-pointer items-center justify-between gap-3 rounded-md border border-card-border-inner bg-secondary p-4">
          <span class="font-semibold text-txtmain">Duration</span>
          <input type="checkbox" checked={settings.activity_fields.duration} onchange={() => toggleActivityField("duration")} disabled={!settings.sqlite_available || !settings.activity_persistence} />
        </label>
      </div>
    </section>

    {#if settings.activity_capture_persistence && settings.capture_redact_headers}
      <div class="rounded-md border border-[#ff9830]/40 bg-[#ff9830]/10 p-4 text-sm text-[#ffcf9f]">
        Persisted captures can contain prompts, responses, and request metadata. Headers are redacted before saving.
      </div>
    {:else if settings.activity_capture_persistence}
      <div class="rounded-md border border-[#f2495c]/40 bg-[#f2495c]/10 p-4 text-sm text-[#ff9aa5]">
        Persisted captures can contain prompts, responses, request metadata, and full headers.
      </div>
    {/if}

    {#if message}
      <div class="rounded-md border border-[#73bf69]/40 bg-[#73bf69]/10 p-3 text-sm text-[#9ce391]">{message}</div>
    {/if}
    {#if error}
      <div class="rounded-md border border-[#f2495c]/40 bg-[#f2495c]/10 p-3 text-sm text-[#ff9aa5]">{error}</div>
    {/if}
  {:else}
    <div class="rounded-lg border border-card-border bg-surface p-6 text-sm text-txtsecondary">Settings are unavailable.</div>
  {/if}
</div>
