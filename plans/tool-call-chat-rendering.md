# Plan: Add Function/Tool Call Support to Activity Capture Chat Rendering

## Context

The Activity tracking capture dialog has a **"Chat Rendering"** preview that visualizes OpenAI-format chat completions as a styled conversation. Currently it supports:

- User / assistant / system messages
- Multipart content (text + images)
- Reasoning content
- SSE streaming responses

However, it **does not support function/tool calls**, which are common in agent workflows. When a request contains `tool_calls` (assistant calling a function) or `role: "tool"` messages (function results), they are silently skipped or not rendered.

## Goal

Add support for extracting and rendering OpenAI-format **tool calls** and **tool responses** in the capture dialog's Chat Rendering preview, then rebuild llama-swap and verify via browser automation.

## OpenAI Formats to Support

### 1. Assistant requesting tool calls

```json
{
  "role": "assistant",
  "content": null,
  "tool_calls": [
    {
      "id": "call_abc123",
      "type": "function",
      "function": {
        "name": "get_weather",
        "arguments": "{\"location\":\"NYC\"}"
      }
    }
  ]
}
```

### 2. Tool result message

```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "{\"temperature\": 72, \"condition\": \"sunny\"}"
}
```

### 3. Response `choices[0].message.tool_calls`

Same structure as #1, returned by the model in the response body.

## Approach

### UI Data Model Changes

Extend `CaptureChatMessage` in `captureChat.ts` to include tool call fields:

- `tool_calls?: ToolCall[]` — for assistant messages that request tool calls
- `tool_call_id?: string` — for tool result messages

Add a new role `"tool"` to the allowed roles.

### Extraction Logic Changes

**`extractRequestChat`** — iterate messages array and:

- Parse `tool_calls` on assistant messages
- Parse `role: "tool"` with `tool_call_id`
- Keep backward compatibility for all existing message shapes

**`extractResponseChat`** — check `choices[0].message.tool_calls` and include them.

**`extractSSEChat`** — tool calls in SSE are less common but can appear in delta form (`delta.tool_calls`). We should accumulate them similarly to content/reasoning.

### Rendering Changes

**`CaptureChatRender.svelte`** — add a new message type for tool interactions:

- **Assistant + tool_calls:** render as an assistant bubble with a collapsible "Tool Calls" section inside it. Each tool call shows:
  - Function name (monospace, bold)
  - Arguments (pre-formatted JSON, collapsible if large)
  - Tool call ID (small, muted)
- **Tool results (`role: "tool"`):** render as a distinct bubble style (e.g., left border accent, muted bg) showing:
  - Tool call ID reference
  - Content (attempt pretty JSON, fallback to raw string)

### Combined Chat Changes

**`CaptureDialog.svelte`** — update `combinedChatMessages` so that tool messages from both request and response are included in the unified Chat Rendering view.

### Export / Copy Changes

Update `getCopyText()` and `getRequestCopyText()` to include tool call info when in render/chat tabs.

## Files to Modify

| File                                                | Action                                                                                                                  |
| --------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `ui-svelte/src/lib/captureChat.ts`                  | Extend `CaptureChatMessage` with tool call fields; update `extractRequestChat`, `extractResponseChat`, `extractSSEChat` |
| `ui-svelte/src/components/CaptureChatRender.svelte` | Add rendering for `tool_calls` on assistant messages and `role: "tool"` messages                                        |
| `ui-svelte/src/components/CaptureDialog.svelte`     | Update `combinedChatMessages`, copy/export text, tab detection for tool content                                         |

## Verification

1. Build UI: `cd ui-svelte && npm run build`
2. Build llama-swap binary: `make linux-amd64` (or `mac` if on Darwin)
3. Start llama-swap with a config that has activity captures enabled
4. Use `agent-browser` to generate captures containing tool calls:
   - Navigate to Activity page
   - Open a capture with tool calls
   - Verify "Chat Rendering" shows assistant tool call bubbles and tool result bubbles
   - Verify request body "Render" tab shows tool messages
   - Verify export/download includes tool data

## Questions for User

1. **Legacy `functions` API:** Should I also support the deprecated OpenAI `functions` / `function_call` format, or only the current `tools` / `tool_calls` API?
2. **Agent-browser test data:** Do you have a preferred way to generate tool-call traffic (e.g., a specific client, script, or should I craft a curl request to llama-swap for testing)?
