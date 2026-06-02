# Plan: Add Markdown Render Mode to Capture Dialog

## Overview

Add a third viewing mode (`raw` | `pretty` | `render`) to both **Request Body** and **Response Body** sections in the Activity monitor's capture dialog. The render mode extracts chat messages from OpenAI-format JSON and displays them as a styled conversation with proper markdown rendering, reasoning/content separation, and code highlighting.

Reuses existing infrastructure:
- `renderMarkdown()` from `lib/markdown.ts` (unified/remark/GFM/math/KaTeX/highlight.js)
- `getTextContent()` / `getImageUrls()` from `lib/types.ts`
- Prose styles from `ChatMessage.svelte`

---

## Files to Touch

| File | Action |
|---|---|
| `ui-svelte/src/lib/captureChat.ts` | **New** -- data extraction helpers |
| `ui-svelte/src/components/CaptureChatRender.svelte` | **New** -- markdown chat renderer component |
| `ui-svelte/src/components/CaptureDialog.svelte` | Modify -- integrate new tab + component |

---

## Step 1: Create `lib/captureChat.ts`

New file with pure functions to extract chat-shaped data from captured request/response bodies.

```typescript
// Extraction types
interface ChatMessage {
  role: "user" | "assistant" | "system";
  content: string;
  reasoning_content?: string;
  imageUrls?: string[];
}

interface ExtractedChat {
  model?: string;
  messages: ChatMessage[];
}

// Extract from OpenAI chat completions request
function extractRequestChat(body: string): ExtractedChat | null

// Extract from non-streaming JSON response
function extractResponseChat(body: string): ExtractedChat | null

// Extract from SSE stream (refactor existing parseSSEChat logic)
function extractSSEChat(body: string): { reasoning: string; content: string }
```

**Key logic details:**
- Parse JSON safely, return `null` on failure
- `messages` array: handle `content` as `string | ContentPart[]` using existing `getTextContent()` / `getImageUrls()`
- Response JSON: look at `choices[0].message.content` + `choices[0].message.reasoning_content`
- For assistant responses with reasoning, create two visual blocks: reasoning (collapsible) + content
- SSE: refactor the existing `parseSSEChat()` from `CaptureDialog.svelte` into this shared file

---

## Step 2: Create `components/CaptureChatRender.svelte`

New reusable component that takes an `ExtractedChat` or SSE result and renders it as a chat conversation.

```svelte
<script lang="ts">
  import { renderMarkdown } from "../lib/markdown";
  import type { ChatMessage } from "../lib/captureChat";
  import { Brain, ChevronDown, ChevronRight } from "lucide-svelte";

  interface Props {
    messages?: ChatMessage[];
    reasoning?: string;
    content?: string;
  }
</script>
```

**Layout:**
- Container with vertical flex, gap between messages
- **User messages:** right-aligned bubble, `bg-primary text-btn-primary-text`, max-width 85%
- **System messages:** full-width, muted styling (smaller text, italic, border-left accent)
- **Assistant messages:** left-aligned, `bg-surface border`, max-width 85%
  - If `reasoning_content` exists: collapsible reasoning block above content
    - Header bar with `Brain` icon, "Reasoning" label, char count
    - Toggle with `ChevronRight` / `ChevronDown`
    - Content in `whitespace-pre-wrap font-mono text-sm text-txtsecondary`
  - Content rendered via `{@html renderMarkdown(message.content)}`
  - Wrapped in `<div class="prose prose-sm dark:prose-invert max-w-none">`

**Code block copy buttons:**
- Reuse the same `codeBlockCopy` Svelte action from `ChatMessage.svelte`
- Attach MutationObserver to inject copy buttons into `<pre>` blocks

**Images:**
- If `imageUrls` present on a user message, show thumbnail grid above text
- Click to open modal (can be simpler: just `target="_blank"` or lightbox)

---

## Step 3: Modify `CaptureDialog.svelte`

### 3a. Update `BodyTab` type
```typescript
type BodyTab = "raw" | "pretty" | "chat" | "render";
```

### 3b. Add tab state
```typescript
let reqBodyTab: BodyTab = $state("pretty");
let respBodyTab: BodyTab = $state("pretty");
```

### 3c. Extract request/response chat data
Add derived values using the new extraction helpers:
```typescript
let requestChat = $derived.by(() => {
  if (!isRequestJson || !requestBodyRaw) return null;
  return extractRequestChat(requestBodyRaw);
});

let responseChat = $derived.by(() => {
  if (isSSE && responseBodyRaw) {
    const chat = extractSSEChat(responseBodyRaw);
    return chat.content || chat.reasoning ? chat : null;
  }
  if (isResponseJson && responseBodyRaw) {
    return extractResponseChat(responseBodyRaw);
  }
  return null;
});
```

### 3d. Tab button updates

**Request Body tabs** (when `isRequestJson`):
```
[Pretty] [Render] [Raw]
```
- Render tab shown when `requestChat` is not null
- Default to `"pretty"` when dialog opens, but if user switches to render, persist

**Response Body tabs**:
```
[Chat] [Render] [Pretty] [Raw]   (for SSE)
[Render] [Pretty] [Raw]          (for non-streaming JSON)
```
- "Chat" tab stays as the current plain-text SSE parser (backward compat)
- "Render" tab renders the same SSE data through markdown
- For non-streaming JSON responses, show Render/Pretty/Raw

### 3e. Content rendering updates

**Request Body section:**
```svelte
{#if reqBodyTab === "render" && requestChat}
  <CaptureChatRender messages={requestChat.messages} />
{:else}
  <pre>{displayedRequestBody}</pre>
{/if}
```

**Response Body section:**
```svelte
{#if respBodyTab === "render"}
  {#if isSSE}
    <CaptureChatRender reasoning={sseChat.reasoning} content={sseChat.content} />
  {:else if responseChat}
    <CaptureChatRender messages={responseChat.messages} />
  {/if}
{:else if respBodyTab === "chat"}
  <!-- existing plain-text chat view -->
{:else}
  <pre>{displayedResponseBody}</pre>
{/if}
```

### 3f. Copy button for render mode

Update `getCopyText()` to handle `"render"`:
```typescript
if (tab === "render") {
  // Flatten all visible text content
  return messages.map(m => `${m.role}: ${m.content}`).join("\n\n");
}
```

---

## Step 4: Styling Requirements

Add to `CaptureDialog.svelte` `<style>` or a new scoped style in `CaptureChatRender.svelte`:

```css
/* Reuse from ChatMessage.svelte */
.prose :global(pre) { position: relative; ... }
.prose :global(.code-copy-btn) { ... }
.prose :global(p) { margin: 0.5rem 0; }
.prose :global(p:first-child) { margin-top: 0; }
.prose :global(p:last-child) { margin-bottom: 0; }
/* ... etc */
```

**Chat bubble colors:**
- User: `bg-primary text-btn-primary-text`
- Assistant: `bg-surface border border-gray-200 dark:border-white/10`
- System: `bg-secondary/50 text-txtsecondary italic border-l-2 border-primary`

---

## Step 5: Edge Cases & Fallbacks

| Case | Behavior |
|---|---|
| Invalid JSON | Render tab hidden |
| Missing `messages` array | Render tab hidden |
| `content` is `null` | Show "(empty)" placeholder |
| `content` is `ContentPart[]` with images | Extract image URLs, show thumbnails |
| SSE with only reasoning, no content | Show reasoning block, no content section |
| SSE with only content, no reasoning | Show content directly (no reasoning header) |
| Binary/non-text response | Render tab not available |

---

## Step 6: Build & Test

```bash
cd /home/abdallah/apps/llama-swap/ui-svelte
npm run build
cd ..
go build -o build/llama-swap-linux-amd64
systemctl --user restart llama-swap.service
```

**Test scenarios:**
1. Click a capture with a `/v1/chat/completions` request -> Request Body -> Render tab -> should show user/system messages
2. Click a capture with a non-streaming response -> Response Body -> Render tab -> should show assistant reasoning + content as markdown
3. Click a capture with SSE streaming -> Response Body -> Render tab -> should show reasoning + content as rendered markdown
4. Verify code blocks have copy buttons
5. Verify KaTeX math renders correctly
6. Verify images in user messages display as thumbnails

---

## Summary of Changes

| What | Where |
|---|---|
| `extractRequestChat()`, `extractResponseChat()`, `extractSSEChat()` | New: `ui-svelte/src/lib/captureChat.ts` |
| `CaptureChatRender.svelte` component | New: `ui-svelte/src/components/CaptureChatRender.svelte` |
| Add `"render"` to `BodyTab` | `CaptureDialog.svelte` |
| Add `requestChat` / `responseChat` derived values | `CaptureDialog.svelte` |
| Add Render tab buttons to req/resp body sections | `CaptureDialog.svelte` |
| Add `{#if tab === "render"}` rendering branches | `CaptureDialog.svelte` |
| Refactor `parseSSEChat()` to shared file | `CaptureDialog.svelte` -> `captureChat.ts` |

---

This plan keeps the existing raw/pretty/chat tabs intact, adds the new render mode only when chat-shaped data is detected, reuses all existing markdown infrastructure, and presents the conversation in a familiar chat interface layout.
