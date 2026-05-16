<script lang="ts">
  import { renderMarkdown } from "../lib/markdown";
  import type { CaptureChatMessage } from "../lib/captureChat";
  import { Brain, ChevronDown, ChevronRight } from "lucide-svelte";

  interface Props {
    messages?: CaptureChatMessage[];
    reasoning?: string;
    content?: string;
  }

  let { messages = [], reasoning = "", content = "" }: Props = $props();

  let reasoningOpen = $state<Record<number, boolean>>({});
  let sseReasoningOpen = $state(false);

  const COPY_SVG = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>`;
  const CHECK_SVG = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>`;

  function codeBlockCopy(node: HTMLElement) {
    function attachButtons() {
      node
        .querySelectorAll<HTMLPreElement>("pre:not([data-copy-btn])")
        .forEach((pre) => {
          pre.setAttribute("data-copy-btn", "true");
          const btn = document.createElement("button");
          btn.className = "code-copy-btn";
          btn.title = "Copy code";
          btn.innerHTML = COPY_SVG;
          btn.addEventListener("click", async () => {
            const text =
              pre.querySelector("code")?.textContent ?? pre.textContent ?? "";
            try {
              if (navigator.clipboard && window.isSecureContext) {
                await navigator.clipboard.writeText(text);
              } else {
                const ta = document.createElement("textarea");
                ta.value = text;
                ta.style.cssText = "position:fixed;left:-9999px";
                document.body.appendChild(ta);
                ta.select();
                document.execCommand("copy");
                document.body.removeChild(ta);
              }
              btn.innerHTML = CHECK_SVG;
              btn.classList.add("copied");
              setTimeout(() => {
                btn.innerHTML = COPY_SVG;
                btn.classList.remove("copied");
              }, 2000);
            } catch (e) {
              console.error("copy failed", e);
            }
          });
          pre.appendChild(btn);
        });
    }
    attachButtons();
    const mo = new MutationObserver(attachButtons);
    mo.observe(node, { childList: true, subtree: true });
    return { destroy: () => mo.disconnect() };
  }

  function toggleReasoning(index: number) {
    reasoningOpen[index] = !reasoningOpen[index];
  }
</script>

<div class="flex flex-col gap-3 p-3">
  {#if messages && messages.length > 0}
    {#each messages as message, i}
      {#if message.role === "user"}
        <div class="flex justify-end">
          <div
            class="max-w-[85%] bg-primary text-btn-primary-text rounded-lg px-4 py-2"
          >
            {#if message.imageUrls && message.imageUrls.length > 0}
              <div class="mb-2 flex flex-wrap gap-2">
                {#each message.imageUrls as imageUrl, idx (idx)}
                  <a
                    href={imageUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="rounded border border-white/20 hover:opacity-80 transition-opacity"
                  >
                    <img
                      src={imageUrl}
                      alt="Image {idx + 1}"
                      class="max-w-[200px] rounded"
                    />
                  </a>
                {/each}
              </div>
            {/if}
            {#if message.content}
              <div class="whitespace-pre-wrap">{message.content}</div>
            {:else}
              <div class="italic opacity-70">(empty)</div>
            {/if}
          </div>
        </div>
      {:else if message.role === "system"}
        <div
          class="w-full bg-secondary/50 text-txtsecondary italic border-l-2 border-primary rounded-r px-3 py-2 text-sm"
        >
          {message.content || "(empty)"}
        </div>
      {:else}
        <div class="flex justify-start">
          <div
            class="max-w-[85%] bg-surface border border-gray-200 dark:border-white/10 rounded-lg px-4 py-2 w-full"
          >
            {#if message.reasoning_content}
              <div
                class="mb-3 border border-gray-200 dark:border-white/10 rounded overflow-hidden"
              >
                <button
                  class="w-full flex items-center gap-2 px-3 py-2 bg-gray-50 dark:bg-white/5 hover:bg-gray-100 dark:hover:bg-white/10 transition-colors text-sm"
                  onclick={() => toggleReasoning(i)}
                >
                  {#if reasoningOpen[i]}
                    <ChevronDown class="w-4 h-4" />
                  {:else}
                    <ChevronRight class="w-4 h-4" />
                  {/if}
                  <Brain class="w-4 h-4" />
                  <span class="font-medium">Reasoning</span>
                  <span class="text-txtsecondary ml-2"
                    >({message.reasoning_content.length} chars)</span
                  >
                </button>
                {#if reasoningOpen[i]}
                  <div
                    class="px-3 py-2 bg-gray-50/50 dark:bg-white/[0.02] text-sm text-txtsecondary whitespace-pre-wrap font-mono"
                  >
                    {message.reasoning_content}
                  </div>
                {/if}
              </div>
            {/if}
            {#if message.content}
              <div
                class="prose prose-sm dark:prose-invert max-w-none"
                use:codeBlockCopy
              >
                {@html renderMarkdown(message.content)}
              </div>
            {:else}
              <div class="italic text-txtsecondary">(empty)</div>
            {/if}
          </div>
        </div>
      {/if}
    {/each}
  {:else if reasoning || content}
    <div class="flex justify-start">
      <div
        class="max-w-[85%] bg-surface border border-gray-200 dark:border-white/10 rounded-lg px-4 py-2 w-full"
      >
        {#if reasoning}
          <div
            class="mb-3 border border-gray-200 dark:border-white/10 rounded overflow-hidden"
          >
            <button
              class="w-full flex items-center gap-2 px-3 py-2 bg-gray-50 dark:bg-white/5 hover:bg-gray-100 dark:hover:bg-white/10 transition-colors text-sm"
              onclick={() => (sseReasoningOpen = !sseReasoningOpen)}
            >
              {#if sseReasoningOpen}
                <ChevronDown class="w-4 h-4" />
              {:else}
                <ChevronRight class="w-4 h-4" />
              {/if}
              <Brain class="w-4 h-4" />
              <span class="font-medium">Reasoning</span>
              <span class="text-txtsecondary ml-2"
                >({reasoning.length} chars)</span
              >
            </button>
            {#if sseReasoningOpen}
              <div
                class="px-3 py-2 bg-gray-50/50 dark:bg-white/[0.02] text-sm text-txtsecondary whitespace-pre-wrap font-mono"
              >
                {reasoning}
              </div>
            {/if}
          </div>
        {/if}
        {#if content}
          <div
            class="prose prose-sm dark:prose-invert max-w-none"
            use:codeBlockCopy
          >
            {@html renderMarkdown(content)}
          </div>
        {:else}
          <div class="italic text-txtsecondary">(empty)</div>
        {/if}
      </div>
    </div>
  {:else}
    <div class="text-txtsecondary italic">(empty)</div>
  {/if}
</div>

<style>
  .prose :global(pre) {
    position: relative;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border, rgba(128, 128, 128, 0.2));
    border-radius: 0.375rem;
    padding: 0.75rem;
    padding-right: 2.5rem;
    overflow-x: auto;
    margin: 0.5rem 0;
  }

  .prose :global(.code-copy-btn) {
    position: absolute;
    top: 0.375rem;
    right: 0.375rem;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0.25rem;
    border-radius: 0.25rem;
    border: 1px solid var(--color-border);
    background: var(--color-surface);
    color: var(--color-txtsecondary);
    cursor: pointer;
    transition: background-color 0.15s;
    line-height: 0;
  }

  .prose :global(.code-copy-btn:hover) {
    background: var(--color-secondary);
  }

  .prose :global(.code-copy-btn.copied) {
    color: var(--color-success);
    opacity: 1;
  }

  .prose :global(code) {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 0.875em;
  }

  .prose :global(pre code) {
    background: none;
    padding: 0;
  }

  .prose :global(code:not(pre code)) {
    background-color: var(--color-surface);
    padding: 0.125rem 0.25rem;
    border-radius: 0.25rem;
    border: 1px solid var(--color-border, rgba(128, 128, 128, 0.2));
  }

  .prose :global(p) {
    margin: 0.5rem 0;
  }

  .prose :global(p:first-child) {
    margin-top: 0;
  }

  .prose :global(p:last-child) {
    margin-bottom: 0;
  }

  .prose :global(ul),
  .prose :global(ol) {
    margin: 0.5rem 0;
    padding-left: 1.5rem;
  }

  .prose :global(li) {
    margin: 0.25rem 0;
  }

  .prose :global(h1),
  .prose :global(h2),
  .prose :global(h3),
  .prose :global(h4) {
    margin: 1rem 0 0.5rem 0;
    font-weight: 600;
  }

  .prose :global(h1:first-child),
  .prose :global(h2:first-child),
  .prose :global(h3:first-child),
  .prose :global(h4:first-child) {
    margin-top: 0;
  }

  .prose :global(blockquote) {
    border-left: 3px solid var(--color-primary);
    padding-left: 1rem;
    margin: 0.5rem 0;
    font-style: italic;
  }

  .prose :global(a) {
    color: var(--color-primary);
    text-decoration: underline;
  }

  .prose :global(table) {
    width: 100%;
    border-collapse: collapse;
    margin: 0.5rem 0;
  }

  .prose :global(th),
  .prose :global(td) {
    border: 1px solid var(--color-border, rgba(128, 128, 128, 0.2));
    padding: 0.5rem;
    text-align: left;
  }

  .prose :global(th) {
    background-color: var(--color-surface);
    font-weight: 600;
  }

  /* Highlight.js theme overrides for dark mode */
  :global(.dark) .prose :global(.hljs) {
    background: transparent;
  }
</style>
