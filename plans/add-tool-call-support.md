# Plan: Add Function/Tool Call Support to Activity Chat Rendering

## Overview

Extend the CHAT RENDERING preview in the Activity monitor to support OpenAI-compatible **function calling** and **tool calls**. Currently only user/system/assistant messages with text/images are rendered. We need to handle:

1. **Assistant tool_calls** – messages where the model requests tool execution
2. **Tool result messages** – `role: "tool"` messages returning tool outputs
3. **Streaming tool_calls** – SSE deltas with `tool_calls` fragments
4. **Tool definitions** – the `tools` array from the request (optional, informational)

## OpenAI API Shapes

### Request – assistant history with tool_calls

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
        "arguments": "{\"location\":\"SF\"}"
      }
    }
  ]
}
```

### Request – tool result

```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "{\"temperature\":72}"
}
```

### Response – non-streaming assistant with tool_calls

`choices[0].message.tool_calls` same shape as above.

### Response – SSE streaming tool_calls

```json
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"lo"}}]}}]}
```

Arguments stream incrementally and must be accumulated.

## Files to Touch

| File                                                | Action                                                                                             |
| --------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| `ui-svelte/src/lib/types.ts`                        | Add `ToolCall`, `ToolFunction`, `ToolDefinition` interfaces                                        |
| `ui-svelte/src/lib/captureChat.ts`                  | Extend extraction helpers for tool calls, tool messages, streaming tool_calls, and top-level tools |
| `ui-svelte/src/components/CaptureChatRender.svelte` | Add UI rendering for tool calls and tool results                                                   |
| `ui-svelte/src/components/CaptureDialog.svelte`     | Update combined chat, copy text, and export logic                                                  |

## Step 1: Extend Types (`lib/types.ts`)

```typescript
export interface ToolCall {
  id: string;
  type: "function";
  function: {
    name: string;
    arguments: string;
  };
}

export interface ToolFunction {
  name: string;
  description?: string;
  parameters?: Record<string, unknown>;
}

export interface ToolDefinition {
  type: "function";
  function: ToolFunction;
}
```

## Step 2: Extend Extraction (`lib/captureChat.ts`)

- `CaptureChatMessage` gains `toolCalls?: ToolCall[]`, `toolCallId?: string`
- `ExtractedChat` gains `tools?: ToolDefinition[]`
- `extractRequestChat()`:
  - Extract `tools` array from top-level request
  - Handle `role === "tool"` → `toolCallId`
  - Handle `role === "assistant"` → extract `tool_calls`
- `extractResponseChat()`:
  - Extract `choices[0].message.tool_calls`
- `extractSSEChat()`:
  - Accumulate `delta.tool_calls` array, merging by `index`
  - Return `toolCalls: ToolCall[]` in addition to reasoning/content
- Add `unescapeLiteral` on tool arguments strings

## Step 3: Render UI (`CaptureChatRender.svelte`)

New visual elements:

- **Assistant with tool_calls:** show a "wrench" icon card per tool call
  - Function name as header
  - Arguments JSON pretty-printed in collapsible `<pre>` block
  - Try to parse and format arguments as pretty JSON
- **Tool result message (`role: "tool"`):** show a "database/check" icon card
  - `tool_call_id` label
  - Content rendered (pretty JSON if parseable, else raw text)
- Colors:
  - Tool call: `bg-amber-50 dark:bg-amber-900/20 border-amber-200 dark:border-amber-700/30`
  - Tool result: `bg-emerald-50 dark:bg-emerald-900/20 border-emerald-200 dark:border-emerald-700/30`

## Step 4: Update Dialog (`CaptureDialog.svelte`)

- `combinedChatMessages` must include assistant tool_calls and tool messages
- `getRequestCopyText()` / `getCopyText()` flatten tool calls into text
- Export includes tool data

## Step 5: Build & Test

```bash
cd ui-svelte && npm run build
cd .. && go build -ldflags="-X main.commit=dev -X main.version=dev -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o build/llama-swap-linux-amd64
```

Test scenarios:

1. Request with `tools` array and assistant `tool_calls` → renders tool call cards
2. Request with `role: "tool"` messages → renders tool result cards
3. SSE streaming response with `delta.tool_calls` → accumulates and renders
4. Combined chat view shows full conversation with tools

## Backwards Compatibility

All tool-related fields are optional. Existing captures without tools continue rendering exactly as before.
