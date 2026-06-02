<script lang="ts">
  import { X, Octagon } from "lucide-svelte";
  import { streamLiveTokens, activityLive, cancelActivity } from "../stores/api";
  import type { LiveActivityRow, TokenStreamChunk } from "../lib/types";
  import CaptureChatRender from "./CaptureChatRender.svelte";

  interface Props {
    row: LiveActivityRow | null;
    open: boolean;
    onclose: () => void;
  }

  let { row, open, onclose }: Props = $props();

  let dialogEl: HTMLDialogElement | undefined = $state();
  let content = $state("");
  let reasoning = $state("");
  let done = $state(false);
  let connected = $state(false);
  let disconnectFn: (() => void) | null = null;
  let contentDiv: HTMLDivElement | undefined = $state();
  let cancelling = $state(false);

  // Keep row data fresh from the live store while dialog is open
  let liveRow = $derived.by(() => {
    if (!row) return null;
    const fromStore = ($activityLive).find((r) => r.id === row.id);
    return fromStore || row;
  });

  $effect(() => {
    if (open && dialogEl) {
      dialogEl.showModal();
    } else if (!open && dialogEl) {
      dialogEl.close();
    }
  });

  $effect(() => {
    if (disconnectFn) {
      disconnectFn();
      disconnectFn = null;
    }
    content = "";
    reasoning = "";
    done = false;
    connected = false;
    cancelling = false;

    if (open && row) {
      connected = true;
      disconnectFn = streamLiveTokens(row.id, {
        onChunk: (chunk: TokenStreamChunk) => {
          if (chunk.kind === "content") {
            content += chunk.text;
          } else if (chunk.kind === "reasoning") {
            reasoning += chunk.text;
          }
        },
        onDone: () => {
          done = true;
          connected = false;
        },
        onError: (err) => {
          console.error("Live token stream error:", err);
          connected = false;
        },
      });
    }

    return () => {
      if (disconnectFn) {
        disconnectFn();
        disconnectFn = null;
      }
    };
  });

  // Auto-scroll to bottom when content updates
  $effect(() => {
    content;
    reasoning;
    if (contentDiv) {
      requestAnimationFrame(() => {
        if (contentDiv) {
          contentDiv.scrollTop = contentDiv.scrollHeight;
        }
      });
    }
  });

  function handleClose() {
    onclose();
  }

  async function handleCancel() {
    if (!row || cancelling) return;
    cancelling = true;
    const ok = await cancelActivity(row.id);
    cancelling = false;
    if (ok) {
      connected = false;
      done = true;
      dialogEl?.close();
    }
  }

  function phaseLabel(r: LiveActivityRow): string {
    if (r.pp_progress !== undefined && r.pp_progress < 1) {
      return `Prompt Processing ${Math.round(r.pp_progress * 100)}%`;
    }
    if (content || reasoning || r.generated_tokens !== undefined) {
      return "Token Generation";
    }
    return "Waiting for response...";
  }
</script>

<dialog
  bind:this={dialogEl}
  onclose={handleClose}
  class="bg-surface text-txtmain rounded-lg shadow-xl max-w-4xl w-full max-h-[90vh] p-0 backdrop:bg-black/50 m-auto"
>
  {#if liveRow}
    <div class="flex flex-col max-h-[90vh]">
      <!-- Header -->
      <div
        class="flex justify-between items-start p-4 border-b border-card-border gap-4"
      >
        <div class="min-w-0">
          <h2 class="text-xl font-bold truncate">
            Live: {liveRow.model}
          </h2>
          <div class="mt-1 flex items-center gap-2 text-sm text-txtsecondary">
            <span>{phaseLabel(liveRow)}</span>
            {#if connected}
              <span class="relative flex h-2 w-2">
                <span
                  class="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75"
                ></span>
                <span
                  class="relative inline-flex h-2 w-2 rounded-full bg-emerald-500"
                ></span>
              </span>
            {:else if done}
              <span class="inline-flex items-center gap-1 text-emerald-600 dark:text-emerald-400 text-xs">
                <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/></svg>
                Complete
              </span>
            {/if}
          </div>
        </div>
        <div class="flex items-center gap-2 shrink-0">
          {#if !done && connected}
            <button
              type="button"
              onclick={handleCancel}
              disabled={cancelling}
              class="inline-flex items-center gap-1 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-1.5 text-xs font-semibold text-red-700 transition hover:bg-red-500/20 disabled:cursor-not-allowed disabled:opacity-60 dark:text-red-300"
              title="Cancel request"
            >
              <Octagon size={14} />
              {cancelling ? "Cancelling..." : "Cancel"}
            </button>
          {/if}
          <button
            type="button"
            onclick={() => dialogEl?.close()}
            class="text-txtsecondary hover:text-txtmain p-1 rounded transition"
            title="Close"
          >
            <X size={20} />
          </button>
        </div>
      </div>

      <!-- Prompt Processing Progress Bar -->
      {#if liveRow.pp_progress !== undefined && liveRow.pp_progress < 1}
        <div class="px-4 pt-3">
          <div class="flex items-center justify-between text-xs text-txtsecondary mb-1">
            <span>Prompt Processing</span>
            <span>{Math.round(Math.max(0, Math.min(1, liveRow.pp_progress)) * 100)}%</span>
          </div>
          <div class="h-2 w-full bg-gray-200 dark:bg-white/10 rounded-full overflow-hidden">
            <div
              class="h-full bg-[#5794f2] transition-all duration-500 rounded-full"
              style="width: {Math.max(0, Math.min(1, liveRow.pp_progress)) * 100}%"
            ></div>
          </div>
        </div>
      {/if}

      <!-- Token Stream Content -->
      <div
        bind:this={contentDiv}
        class="flex-1 overflow-y-auto p-4 min-h-0"
      >
        {#if content || reasoning}
          <div class="bg-background rounded border border-card-border">
            <CaptureChatRender {reasoning} {content} />
          </div>
        {:else}
          <div class="flex flex-col items-center justify-center text-txtsecondary py-16 gap-3">
            {#if connected}
              <span class="relative flex h-3 w-3">
                <span
                  class="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75"
                ></span>
                <span
                  class="relative inline-flex h-3 w-3 rounded-full bg-emerald-500"
                ></span>
              </span>
              <span>Waiting for tokens to stream...</span>
              {#if liveRow.pp_progress !== undefined}
                <span class="text-xs">Prompt is being processed</span>
              {/if}
            {:else}
              <span>No tokens received yet</span>
            {/if}
          </div>
        {/if}
      </div>

      <!-- Footer -->
      <div
        class="p-3 border-t border-card-border flex items-center justify-between gap-4"
      >
        <div class="flex items-center gap-3 text-sm text-txtsecondary">
          {#if liveRow.generated_tokens !== undefined}
            <span>
              <span class="font-medium text-txtmain">{liveRow.generated_tokens.toLocaleString()}</span>
              tokens
            </span>
          {/if}
          {#if liveRow.pp_speed !== undefined && liveRow.pp_speed > 0 && liveRow.pp_progress !== undefined && liveRow.pp_progress < 1}
            <span>{liveRow.pp_speed.toFixed(1)} t/s (PP)</span>
          {/if}
          {#if liveRow.tg_speed !== undefined && liveRow.tg_speed > 0}
            <span>{liveRow.tg_speed.toFixed(1)} t/s (TG)</span>
          {/if}
        </div>
        <button onclick={() => dialogEl?.close()} class="btn">Close</button>
      </div>
    </div>
  {/if}
</dialog>
