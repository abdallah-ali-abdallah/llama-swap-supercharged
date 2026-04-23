<script lang="ts">
  import { onMount } from "svelte";
  import { getPersistenceSettings, updatePersistenceSettings } from "../stores/api";
  import type { PersistenceSettings } from "../stores/api";

  let settings = $state<PersistenceSettings | null>(null);
  let loading = $state(true);
  let saving = $state(false);
  let message = $state("");
  let error = $state("");

  const countFormatter = new Intl.NumberFormat();
  const dateTimeFormatter = new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });

  type ActivityFieldKey = keyof PersistenceSettings["activity_fields"];

  const activityFieldOptions: Array<{
    key: ActivityFieldKey;
    title: string;
    description: string;
  }> = [
    {
      key: "model",
      title: "Model identity",
      description: "Save the resolved model name so Activity can show which backend handled each request.",
    },
    {
      key: "tokens",
      title: "Token counts",
      description: "Store prompt, generated, and cache token totals for usage analysis and cost tracking.",
    },
    {
      key: "speeds",
      title: "Generation speeds",
      description: "Keep prompt and generation throughput so you can compare model performance over time.",
    },
    {
      key: "duration",
      title: "Request duration",
      description: "Preserve total request time to troubleshoot latency and long-running responses.",
    },
  ];

  function formatCount(value: number | undefined): string {
    return countFormatter.format(value ?? 0);
  }

  function formatBytes(value: number | undefined): string {
    if (!value || value <= 0) {
      return "0 B";
    }

    const units = ["B", "KB", "MB", "GB", "TB"];
    let size = value;
    let unitIndex = 0;
    while (size >= 1024 && unitIndex < units.length - 1) {
      size /= 1024;
      unitIndex += 1;
    }

    const digits = unitIndex === 0 || size >= 10 ? 0 : 1;
    return `${size.toFixed(digits)} ${units[unitIndex]}`;
  }

  function formatDateTime(value: number): string {
    return dateTimeFormatter.format(new Date(value));
  }

  function formatStoredRange(oldest: number | undefined, newest: number | undefined): string {
    if (!oldest || !newest) {
      return "No rows stored yet";
    }
    if (oldest === newest) {
      return formatDateTime(oldest);
    }
    return `${formatDateTime(oldest)} to ${formatDateTime(newest)}`;
  }

  function formatFieldSummary(current: PersistenceSettings): string {
    const enabled = activityFieldOptions.filter((option) => current.activity_fields[option.key]).map((option) => option.title);
    return enabled.length ? enabled.join(", ") : "Only ID and timestamp are stored.";
  }

  function storageSummary(current: PersistenceSettings): string {
    if (current.yaml_available && current.sqlite_available) {
      return "YAML remains the source of truth. SQLite mirrors the active settings and stores retained history.";
    }
    if (current.sqlite_available) {
      return "SQLite is active and stores both the retention policy and any saved history.";
    }
    if (current.yaml_available) {
      return "The YAML file is present, but SQLite is currently unavailable so history cannot be written.";
    }
    return "History persistence is unavailable until SQLite can be opened.";
  }

  function loggingSummary(current: PersistenceSettings): string {
    return current.logging_enabled
      ? "Proxy and upstream logs are collected and can be streamed in the Logs view."
      : "Proxy and upstream log collection is fully disabled and buffered logs are cleared.";
  }

  function historySummary(current: PersistenceSettings): string {
    if (!current.sqlite_available) {
      return "History retention is offline because SQLite is unavailable.";
    }
    if (!current.usage_metrics_persistence && !current.activity_persistence) {
      return "Dashboard and Activity history are disabled. The UI will only show live-session data.";
    }
    if (current.activity_capture_persistence) {
      return current.capture_redact_headers
        ? "Activity stores rows and full request bodies, while headers are redacted before they reach disk."
        : "Activity stores rows, request bodies, and full request headers. Review this before saving sensitive traffic.";
    }
    if (current.activity_persistence) {
      return "Activity stores row summaries only. Request and response bodies are not persisted.";
    }
    return "Only Dashboard usage metrics are retained between restarts.";
  }

  function safetyNotes(current: PersistenceSettings): string[] {
    const notes: string[] = [];

    if (!current.sqlite_available) {
      notes.push("SQLite is unavailable, so no dashboard or activity history can survive a restart.");
    } else if (!current.usage_metrics_persistence && !current.activity_persistence) {
      notes.push("All long-term history is off. Dashboard and Activity will only show live data from the current session.");
    }

    if (!current.logging_enabled) {
      notes.push("Logging is disabled. The Logs view will stay empty until collection is turned back on.");
    }

    if (current.activity_capture_persistence && !current.capture_redact_headers) {
      notes.push("Saved captures include full request and response headers. Make sure secrets in headers are safe to retain.");
    } else if (current.activity_capture_persistence) {
      notes.push("Saved captures can still contain prompts, responses, and payload metadata even with header redaction enabled.");
    }

    if (current.activity_persistence) {
      notes.push("Field-level retention changes only affect future activity rows. Existing rows are not rewritten.");
    }

    return notes.slice(0, 3);
  }

  function saveButtonLabel(): string {
    return saving ? "Saving..." : "Save path changes";
  }

  function optionCardClasses(enabled: boolean, disabled = false): string {
    return [
      "flex h-full cursor-pointer flex-col gap-4 rounded-2xl border p-5 transition",
      enabled ? "border-[rgba(50,184,198,0.34)] bg-[rgba(50,184,198,0.08)]" : "border-card-border-inner bg-secondary/70",
      disabled ? "cursor-not-allowed opacity-60" : "hover:border-card-border hover:shadow-sm",
    ].join(" ");
  }

  function pillClasses(active: boolean): string {
    return active
      ? "border-[rgba(50,184,198,0.38)] bg-[rgba(50,184,198,0.12)] text-[#63d7e2] dark:text-[#8de7ef]"
      : "border-[rgba(242,73,92,0.3)] bg-[rgba(242,73,92,0.12)] text-[#d94157] dark:text-[#ff9aa5]";
  }

  function statusClasses(kind: "success" | "warning" | "danger"): string {
    if (kind === "success") {
      return "border-[rgba(50,184,198,0.34)] bg-[rgba(50,184,198,0.1)] text-[#0f7f8b] dark:text-[#8de7ef]";
    }
    if (kind === "warning") {
      return "border-[rgba(244,155,0,0.3)] bg-[rgba(244,155,0,0.12)] text-[#b36b00] dark:text-[#ffd08a]";
    }
    return "border-[rgba(242,73,92,0.32)] bg-[rgba(242,73,92,0.12)] text-[#bf3147] dark:text-[#ff9aa5]";
  }

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

  async function persistSettings(successMessage = "Settings saved."): Promise<void> {
    if (!settings) return;
    saving = true;
    message = "";
    error = "";
    try {
      settings = await updatePersistenceSettings(settings);
      message = successMessage;
    } catch (err) {
      error = err instanceof Error ? err.message : "Unable to save persistence settings.";
    } finally {
      saving = false;
    }
  }

  async function saveSettings(): Promise<void> {
    await persistSettings("Storage path saved.");
  }

  function toggleUsage(): void {
    if (!settings) return;
    settings.usage_metrics_persistence = !settings.usage_metrics_persistence;
    void persistSettings("Dashboard history setting saved.");
  }

  function toggleLogging(): void {
    if (!settings) return;
    settings.logging_enabled = !settings.logging_enabled;
    void persistSettings("Logging setting saved.");
  }

  function toggleActivity(): void {
    if (!settings) return;
    settings.activity_persistence = !settings.activity_persistence;
    void persistSettings("Activity row setting saved.");
  }

  function toggleCapture(): void {
    if (!settings) return;
    settings.activity_capture_persistence = !settings.activity_capture_persistence;
    void persistSettings("Activity capture setting saved.");
  }

  function toggleCaptureRedaction(): void {
    if (!settings) return;
    settings.capture_redact_headers = !settings.capture_redact_headers;
    void persistSettings("Header redaction setting saved.");
  }

  function toggleActivityField(field: ActivityFieldKey): void {
    if (!settings) return;
    settings.activity_fields[field] = !settings.activity_fields[field];
    void persistSettings("Saved activity fields updated.");
  }

  onMount(() => {
    loadSettings();
  });
</script>

<div class="settings-page mx-auto flex max-w-[1380px] flex-col gap-5 p-2">
  {#if loading}
    <div class="rounded-[28px] border border-card-border bg-surface p-6 text-sm text-txtsecondary shadow-sm">Loading settings...</div>
  {:else if settings}
    <section class="settings-hero rounded-[28px] border border-card-border px-6 py-6 shadow-sm">
      <div class="flex flex-col gap-6 xl:flex-row xl:items-start xl:justify-between">
        <div class="max-w-3xl">
          <div class="text-xs font-semibold uppercase tracking-[0.24em] text-txtsecondary">Storage and retention policy</div>
          <h1 class="mt-3 p-0 text-2xl font-bold">Settings</h1>
          <p class="mt-3 max-w-2xl text-sm leading-6 text-txtsecondary">
            Choose what llama-swap keeps after a restart, where it stores that history, and how much request data is safe to retain.
          </p>
          <p class="mt-3 text-sm leading-6 text-txtsecondary">
            Switches save immediately. Use the save button after editing the SQLite database path.
          </p>
        </div>

        <div class="flex min-w-0 flex-col items-stretch gap-3 xl:items-end">
          <div class="flex flex-wrap gap-2 xl:justify-end">
            <span class={`inline-flex items-center rounded-full border px-3 py-1 text-xs font-semibold ${pillClasses(settings.sqlite_available)}`}>
              SQLite {settings.sqlite_available ? "ready" : "offline"}
            </span>
            <span class={`inline-flex items-center rounded-full border px-3 py-1 text-xs font-semibold ${settings.yaml_available ? "border-[rgba(94,82,64,0.2)] bg-secondary/80 text-txtmain" : "border-card-border-inner bg-secondary/50 text-txtsecondary"}`}>
              YAML {settings.yaml_available ? "connected" : "not found"}
            </span>
          </div>
          <button
            type="button"
            onclick={saveSettings}
            disabled={loading || saving || !settings.db_path.trim()}
            class="inline-flex min-w-[180px] items-center justify-center rounded-xl border border-card-border bg-surface px-4 py-3 text-sm font-semibold text-txtmain transition hover:border-card-border-inner disabled:opacity-60"
          >
            {saveButtonLabel()}
          </button>
        </div>
      </div>

      <div class="mt-6 grid grid-cols-1 gap-3 lg:grid-cols-3">
        <div class="rounded-2xl border border-card-border-inner bg-surface/80 p-4 backdrop-blur-sm">
          <div class="text-xs font-semibold uppercase tracking-[0.22em] text-txtsecondary">Storage backend</div>
          <div class="mt-3 text-base font-semibold text-txtmain">{settings.sqlite_available ? "Persistent history is available" : "Persistent history is blocked"}</div>
          <p class="mt-2 text-sm leading-6 text-txtsecondary">{storageSummary(settings)}</p>
        </div>
        <div class="rounded-2xl border border-card-border-inner bg-surface/80 p-4 backdrop-blur-sm">
          <div class="text-xs font-semibold uppercase tracking-[0.22em] text-txtsecondary">Logging</div>
          <div class="mt-3 text-base font-semibold text-txtmain">{settings.logging_enabled ? "Logs are being collected" : "Logs are turned off"}</div>
          <p class="mt-2 text-sm leading-6 text-txtsecondary">{loggingSummary(settings)}</p>
        </div>
        <div class="rounded-2xl border border-card-border-inner bg-surface/80 p-4 backdrop-blur-sm">
          <div class="text-xs font-semibold uppercase tracking-[0.22em] text-txtsecondary">Retention snapshot</div>
          <div class="mt-3 text-base font-semibold text-txtmain">{settings.retention_days} day retention window</div>
          <p class="mt-2 text-sm leading-6 text-txtsecondary">{historySummary(settings)}</p>
        </div>
      </div>
    </section>

    <div class="grid grid-cols-1 gap-5 xl:grid-cols-[minmax(0,1.45fr)_minmax(320px,0.95fr)]">
      <div class="space-y-5">
        <section class="rounded-[24px] border border-card-border bg-surface p-5 shadow-sm">
          <div class="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
            <div>
              <h2 class="p-0 text-lg font-semibold text-txtmain">Storage backend</h2>
              <p class="mt-2 text-sm leading-6 text-txtsecondary">
                Define where the persisted database lives and review which configuration file currently controls saved settings.
              </p>
            </div>
            <div class="rounded-full border border-card-border-inner bg-secondary/70 px-3 py-1 text-xs font-semibold uppercase tracking-[0.2em] text-txtsecondary">
              Save required for path edits
            </div>
          </div>

          <div class="mt-5 grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_240px]">
            <label class="rounded-2xl border border-card-border-inner bg-secondary/55 p-4">
              <div class="text-xs font-semibold uppercase tracking-[0.2em] text-txtsecondary">SQLite database path</div>
              <div class="mt-2 text-sm leading-6 text-txtsecondary">
                This file stores saved metrics, activity history, and persisted captures. A missing file is created when the setting is saved.
              </div>
              <input
                class="mt-4 w-full rounded-xl border border-card-border bg-surface px-3 py-3 text-sm font-semibold text-txtmain outline-none transition focus:border-[#5794f2]"
                bind:value={settings.db_path}
                placeholder="/path/to/llama-swap-metrics.db"
              />
            </label>

            <div class="rounded-2xl border border-card-border-inner bg-secondary/55 p-4">
              <div class="text-xs font-semibold uppercase tracking-[0.2em] text-txtsecondary">Current retention</div>
              <div class="mt-3 text-2xl font-semibold text-txtmain">{settings.retention_days}</div>
              <div class="text-sm font-medium text-txtmain">days</div>
              <p class="mt-3 text-sm leading-6 text-txtsecondary">
                Older persisted rows are pruned outside this window to keep storage bounded.
              </p>
            </div>
          </div>

          {#if settings.yaml_available}
            <div class="mt-3 rounded-2xl border border-card-border-inner bg-secondary/45 p-4">
              <div class="text-xs font-semibold uppercase tracking-[0.2em] text-txtsecondary">YAML source file</div>
              <div class="mt-2 break-all text-sm font-semibold text-txtmain">{settings.yaml_path}</div>
              <p class="mt-2 text-sm leading-6 text-txtsecondary">
                YAML overrides conflicting values from SQLite, so this file remains the final authority when both are present.
              </p>
            </div>
          {/if}
        </section>

        <section class="rounded-[24px] border border-card-border bg-surface p-5 shadow-sm">
          <div>
            <h2 class="p-0 text-lg font-semibold text-txtmain">What gets stored</h2>
            <p class="mt-2 text-sm leading-6 text-txtsecondary">
              Turn each retention layer on only when it delivers value. The descriptions below explain exactly what each option writes to disk.
            </p>
          </div>

          <div class="mt-5 grid grid-cols-1 gap-3 lg:grid-cols-2">
            <label class={optionCardClasses(settings.logging_enabled)}>
              <div class="flex items-start justify-between gap-4">
                <div>
                  <div class="text-sm font-semibold text-txtmain">Live logging</div>
                  <div class="mt-1 text-xs font-semibold uppercase tracking-[0.18em] text-txtsecondary">Logs view</div>
                </div>
                <span class="settings-switch">
                  <input type="checkbox" class="sr-only" checked={settings.logging_enabled} onchange={toggleLogging} />
                  <span class={`settings-switch-track ${settings.logging_enabled ? "settings-switch-track--on" : ""}`}>
                    <span class={`settings-switch-thumb ${settings.logging_enabled ? "settings-switch-thumb--on" : ""}`}></span>
                  </span>
                </span>
              </div>
              <p class="text-sm leading-6 text-txtsecondary">
                Collect proxy and upstream logs for the Logs page. Turning this off stops collection entirely and clears buffered log output.
              </p>
            </label>

            <label class={optionCardClasses(settings.usage_metrics_persistence, !settings.sqlite_available)}>
              <div class="flex items-start justify-between gap-4">
                <div>
                  <div class="text-sm font-semibold text-txtmain">Dashboard usage metrics</div>
                  <div class="mt-1 text-xs font-semibold uppercase tracking-[0.18em] text-txtsecondary">Historical dashboard trends</div>
                </div>
                <span class="settings-switch">
                  <input
                    type="checkbox"
                    class="sr-only"
                    checked={settings.usage_metrics_persistence}
                    onchange={toggleUsage}
                    disabled={!settings.sqlite_available}
                  />
                  <span class={`settings-switch-track ${settings.usage_metrics_persistence ? "settings-switch-track--on" : ""} ${!settings.sqlite_available ? "settings-switch-track--disabled" : ""}`}>
                    <span class={`settings-switch-thumb ${settings.usage_metrics_persistence ? "settings-switch-thumb--on" : ""}`}></span>
                  </span>
                </span>
              </div>
              <p class="text-sm leading-6 text-txtsecondary">
                Store token totals, request durations, throughput, and model-level usage so Dashboard can render past ranges instead of only live traffic.
              </p>
            </label>

            <label class={optionCardClasses(settings.activity_persistence, !settings.sqlite_available)}>
              <div class="flex items-start justify-between gap-4">
                <div>
                  <div class="text-sm font-semibold text-txtmain">Activity row summaries</div>
                  <div class="mt-1 text-xs font-semibold uppercase tracking-[0.18em] text-txtsecondary">Historical activity list</div>
                </div>
                <span class="settings-switch">
                  <input
                    type="checkbox"
                    class="sr-only"
                    checked={settings.activity_persistence}
                    onchange={toggleActivity}
                    disabled={!settings.sqlite_available}
                  />
                  <span class={`settings-switch-track ${settings.activity_persistence ? "settings-switch-track--on" : ""} ${!settings.sqlite_available ? "settings-switch-track--disabled" : ""}`}>
                    <span class={`settings-switch-thumb ${settings.activity_persistence ? "settings-switch-thumb--on" : ""}`}></span>
                  </span>
                </span>
              </div>
              <p class="text-sm leading-6 text-txtsecondary">
                Save one completed row per request for the Activity page, including the request timestamp and any extra fields enabled below.
              </p>
            </label>

            <label class={optionCardClasses(settings.activity_capture_persistence, !settings.sqlite_available || !settings.activity_persistence)}>
              <div class="flex items-start justify-between gap-4">
                <div>
                  <div class="text-sm font-semibold text-txtmain">Activity request captures</div>
                  <div class="mt-1 text-xs font-semibold uppercase tracking-[0.18em] text-txtsecondary">Payload replay and debugging</div>
                </div>
                <span class="settings-switch">
                  <input
                    type="checkbox"
                    class="sr-only"
                    checked={settings.activity_capture_persistence}
                    onchange={toggleCapture}
                    disabled={!settings.sqlite_available || !settings.activity_persistence}
                  />
                  <span class={`settings-switch-track ${settings.activity_capture_persistence ? "settings-switch-track--on" : ""} ${!settings.sqlite_available || !settings.activity_persistence ? "settings-switch-track--disabled" : ""}`}>
                    <span class={`settings-switch-thumb ${settings.activity_capture_persistence ? "settings-switch-thumb--on" : ""}`}></span>
                  </span>
                </span>
              </div>
              <p class="text-sm leading-6 text-txtsecondary">
                Persist full request and response payloads for Activity detail views after a restart. This depends on Activity row summaries being enabled.
              </p>
            </label>
          </div>

          <label class={`mt-3 flex cursor-pointer flex-col gap-4 rounded-2xl border p-5 transition ${settings.capture_redact_headers ? "border-[rgba(50,184,198,0.34)] bg-[rgba(50,184,198,0.08)]" : "border-card-border-inner bg-secondary/70"} ${!settings.sqlite_available || !settings.activity_capture_persistence ? "cursor-not-allowed opacity-60" : "hover:border-card-border hover:shadow-sm"}`}>
            <div class="flex items-start justify-between gap-4">
              <div>
                <div class="text-sm font-semibold text-txtmain">Redact headers before saving captures</div>
                <div class="mt-1 text-xs font-semibold uppercase tracking-[0.18em] text-txtsecondary">Privacy control for stored payloads</div>
              </div>
              <span class="settings-switch">
                <input
                  type="checkbox"
                  class="sr-only"
                  checked={settings.capture_redact_headers}
                  onchange={toggleCaptureRedaction}
                  disabled={!settings.sqlite_available || !settings.activity_capture_persistence}
                />
                <span class={`settings-switch-track ${settings.capture_redact_headers ? "settings-switch-track--on" : ""} ${!settings.sqlite_available || !settings.activity_capture_persistence ? "settings-switch-track--disabled" : ""}`}>
                  <span class={`settings-switch-thumb ${settings.capture_redact_headers ? "settings-switch-thumb--on" : ""}`}></span>
                </span>
              </span>
            </div>
            <p class="text-sm leading-6 text-txtsecondary">
              Keep this on to strip request and response headers before captures are written. Turning it off stores headers exactly as they were seen by llama-swap.
            </p>
          </label>
        </section>

        <section class="rounded-[24px] border border-card-border bg-surface p-5 shadow-sm">
          <div class="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
            <div>
              <h2 class="p-0 text-lg font-semibold text-txtmain">Activity row details</h2>
              <p class="mt-2 text-sm leading-6 text-txtsecondary">
                These controls decide which optional columns are saved with future Activity rows. Request ID and timestamp are always retained.
              </p>
            </div>
            <div class="rounded-full border border-card-border-inner bg-secondary/70 px-3 py-1 text-xs font-semibold uppercase tracking-[0.2em] text-txtsecondary">
              {formatFieldSummary(settings)}
            </div>
          </div>

          <div class="mt-5 grid grid-cols-1 gap-3 md:grid-cols-2">
            {#each activityFieldOptions as option}
              <label class={optionCardClasses(settings.activity_fields[option.key], !settings.sqlite_available || !settings.activity_persistence)}>
                <div class="flex items-start justify-between gap-4">
                  <div>
                    <div class="text-sm font-semibold text-txtmain">{option.title}</div>
                    <div class="mt-1 text-xs font-semibold uppercase tracking-[0.18em] text-txtsecondary">Saved with each future row</div>
                  </div>
                  <span class="settings-switch">
                    <input
                      type="checkbox"
                      class="sr-only"
                      checked={settings.activity_fields[option.key]}
                      onchange={() => toggleActivityField(option.key)}
                      disabled={!settings.sqlite_available || !settings.activity_persistence}
                    />
                    <span class={`settings-switch-track ${settings.activity_fields[option.key] ? "settings-switch-track--on" : ""} ${!settings.sqlite_available || !settings.activity_persistence ? "settings-switch-track--disabled" : ""}`}>
                      <span class={`settings-switch-thumb ${settings.activity_fields[option.key] ? "settings-switch-thumb--on" : ""}`}></span>
                    </span>
                  </span>
                </div>
                <p class="text-sm leading-6 text-txtsecondary">{option.description}</p>
              </label>
            {/each}
          </div>
        </section>
      </div>

      <aside class="space-y-5">
        <section class="rounded-[24px] border border-card-border bg-surface p-5 shadow-sm">
          <h2 class="p-0 text-lg font-semibold text-txtmain">Storage health</h2>
          <p class="mt-2 text-sm leading-6 text-txtsecondary">
            A quick view of what is already stored on disk and how much history is available for charts and Activity lookups.
          </p>

          <div class="mt-5 grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div class="rounded-2xl border border-card-border-inner bg-secondary/55 p-4">
              <div class="text-xs font-semibold uppercase tracking-[0.18em] text-txtsecondary">SQLite status</div>
              <div class="mt-2 text-base font-semibold text-txtmain">{settings.sqlite_available ? "Available" : "Unavailable"}</div>
              <div class="mt-2 text-sm leading-6 text-txtsecondary">
                {settings.sqlite_available ? "The database can accept new history and settings snapshots." : "The database cannot be opened, so persistence features are blocked."}
              </div>
            </div>
            <div class="rounded-2xl border border-card-border-inner bg-secondary/55 p-4">
              <div class="text-xs font-semibold uppercase tracking-[0.18em] text-txtsecondary">Config source</div>
              <div class="mt-2 text-base font-semibold text-txtmain">{settings.yaml_available ? "YAML plus SQLite" : "SQLite only"}</div>
              <div class="mt-2 text-sm leading-6 text-txtsecondary">
                {settings.yaml_available ? "YAML values win on conflict and are mirrored into SQLite." : "The running retention policy is read directly from SQLite."}
              </div>
            </div>
          </div>

          {#if settings.stats}
            <div class="mt-5 grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div class="rounded-2xl border border-card-border-inner bg-secondary/55 p-4">
                <div class="text-xs font-semibold uppercase tracking-[0.18em] text-txtsecondary">Database footprint</div>
                <div class="mt-2 text-xl font-semibold text-txtmain">{formatBytes(settings.stats.total_size_bytes)}</div>
                <div class="mt-2 text-xs leading-5 text-txtsecondary">
                  DB {formatBytes(settings.stats.db_size_bytes)} / WAL {formatBytes(settings.stats.wal_size_bytes)} / SHM {formatBytes(settings.stats.shm_size_bytes)}
                </div>
              </div>
              <div class="rounded-2xl border border-card-border-inner bg-secondary/55 p-4">
                <div class="text-xs font-semibold uppercase tracking-[0.18em] text-txtsecondary">Metric rows</div>
                <div class="mt-2 text-xl font-semibold text-txtmain">{formatCount(settings.stats.usage_metrics_rows)}</div>
                <div class="mt-2 text-xs leading-5 text-txtsecondary">{formatStoredRange(settings.stats.oldest_metric_ms, settings.stats.newest_metric_ms)}</div>
              </div>
              <div class="rounded-2xl border border-card-border-inner bg-secondary/55 p-4">
                <div class="text-xs font-semibold uppercase tracking-[0.18em] text-txtsecondary">Activity rows</div>
                <div class="mt-2 text-xl font-semibold text-txtmain">{formatCount(settings.stats.activity_rows)}</div>
                <div class="mt-2 text-xs leading-5 text-txtsecondary">{formatStoredRange(settings.stats.oldest_activity_ms, settings.stats.newest_activity_ms)}</div>
              </div>
              <div class="rounded-2xl border border-card-border-inner bg-secondary/55 p-4">
                <div class="text-xs font-semibold uppercase tracking-[0.18em] text-txtsecondary">Capture payloads</div>
                <div class="mt-2 text-xl font-semibold text-txtmain">{formatCount(settings.stats.activity_captures)}</div>
                <div class="mt-2 text-xs leading-5 text-txtsecondary">
                  {formatBytes(settings.stats.capture_bytes)} stored across {formatCount(settings.stats.settings_rows)} settings rows
                </div>
              </div>
            </div>
          {:else if settings.sqlite_available}
            <div class="mt-5 rounded-2xl border border-card-border-inner bg-secondary/55 p-4 text-sm leading-6 text-txtsecondary">
              Database statistics are not available right now, but SQLite can still be used for persistence.
            </div>
          {/if}
        </section>

        {#if settings.yaml_conflicts?.length}
          <section class="rounded-[24px] border border-[rgba(244,155,0,0.3)] bg-[rgba(244,155,0,0.08)] p-5 shadow-sm">
            <h2 class="p-0 text-lg font-semibold text-txtmain">YAML overrides currently active</h2>
            <p class="mt-2 text-sm leading-6 text-txtsecondary">
              These values were found in SQLite, but the YAML file supplied different values and took precedence.
            </p>

            <div class="mt-4 space-y-3">
              {#each settings.yaml_conflicts as conflict}
                <div class="rounded-2xl border border-[rgba(244,155,0,0.22)] bg-surface/45 p-4">
                  <div class="text-sm font-semibold text-txtmain">{conflict.field}</div>
                  <div class="mt-2 text-sm leading-6 text-txtsecondary">YAML value: {conflict.yaml_value}</div>
                  <div class="text-sm leading-6 text-txtsecondary">SQLite value: {conflict.sqlite_value}</div>
                </div>
              {/each}
            </div>
          </section>
        {/if}

        <section class="rounded-[24px] border border-card-border bg-surface p-5 shadow-sm">
          <h2 class="p-0 text-lg font-semibold text-txtmain">Safety notes</h2>
          <p class="mt-2 text-sm leading-6 text-txtsecondary">
            The current combination of settings has a few practical implications worth keeping visible.
          </p>

          <div class="mt-4 space-y-3">
            {#if safetyNotes(settings).length}
              {#each safetyNotes(settings) as note, index}
                <div class={`rounded-2xl border p-4 text-sm leading-6 ${statusClasses(index === 0 && !settings.sqlite_available ? "danger" : settings.activity_capture_persistence && !settings.capture_redact_headers ? "warning" : "success")}`}>
                  {note}
                </div>
              {/each}
            {:else}
              <div class={`rounded-2xl border p-4 text-sm leading-6 ${statusClasses("success")}`}>
                Persistence is enabled with no immediate warnings. Review capture retention periodically if traffic may contain sensitive prompts or responses.
              </div>
            {/if}
          </div>
        </section>

        {#if settings.activity_capture_persistence}
          <section class={`rounded-[24px] border p-5 shadow-sm ${settings.capture_redact_headers ? statusClasses("warning") : statusClasses("danger")}`}>
            <h2 class="p-0 text-lg font-semibold text-current">Capture sensitivity</h2>
            <p class="mt-2 text-sm leading-6">
              {settings.capture_redact_headers
                ? "Persisted captures still include prompts, responses, and request metadata. Headers are redacted before storage."
                : "Persisted captures include prompts, responses, request metadata, and full headers. Treat the database as sensitive."}
            </p>
          </section>
        {/if}

        {#if message}
          <div class={`rounded-2xl border p-4 text-sm ${statusClasses("success")}`}>{message}</div>
        {/if}

        {#if error}
          <div class={`rounded-2xl border p-4 text-sm ${statusClasses("danger")}`}>{error}</div>
        {/if}
      </aside>
    </div>
  {:else}
    <div class="rounded-[28px] border border-card-border bg-surface p-6 text-sm text-txtsecondary shadow-sm">Settings are unavailable.</div>
  {/if}
</div>

<style>
  .settings-hero {
    background:
      radial-gradient(circle at top right, rgba(50, 184, 198, 0.2), transparent 34%),
      radial-gradient(circle at bottom left, rgba(94, 82, 64, 0.14), transparent 28%),
      linear-gradient(180deg, rgba(255, 255, 255, 0.04), rgba(255, 255, 255, 0)),
      var(--color-surface);
  }

  .settings-switch {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .settings-switch-track {
    position: relative;
    display: inline-flex;
    height: 28px;
    width: 48px;
    border-radius: 999px;
    border: 1px solid var(--color-card-border);
    background: color-mix(in srgb, var(--color-secondary) 85%, transparent);
    transition:
      background-color 160ms ease,
      border-color 160ms ease,
      opacity 160ms ease;
  }

  .settings-switch-track--on {
    border-color: rgba(50, 184, 198, 0.38);
    background: rgba(50, 184, 198, 0.85);
  }

  .settings-switch-track--disabled {
    opacity: 0.65;
  }

  .settings-switch-thumb {
    position: absolute;
    top: 3px;
    left: 3px;
    height: 20px;
    width: 20px;
    border-radius: 999px;
    background: var(--color-surface);
    box-shadow: 0 2px 8px rgba(15, 23, 42, 0.18);
    transition:
      transform 160ms ease,
      background-color 160ms ease;
  }

  .settings-switch-thumb--on {
    transform: translateX(20px);
    background: rgba(255, 255, 255, 0.98);
  }
</style>
