import { getTextContent, getImageUrls } from "./types";
import type { ContentPart } from "./types";

export interface CaptureChatMessage {
  role: "user" | "assistant" | "system";
  content: string;
  reasoning_content?: string;
  imageUrls?: string[];
}

export interface ExtractedChat {
  model?: string;
  messages: CaptureChatMessage[];
}

export interface SSEChat {
  reasoning: string;
  content: string;
}

export function extractRequestChat(body: string): ExtractedChat | null {
  try {
    const parsed = JSON.parse(body);
    if (!Array.isArray(parsed.messages)) return null;

    const messages: CaptureChatMessage[] = [];
    for (const msg of parsed.messages) {
      if (!msg || typeof msg !== "object") continue;
      const role = msg.role;
      if (role !== "user" && role !== "assistant" && role !== "system") continue;

      const content: string | ContentPart[] = msg.content ?? "";
      messages.push({
        role,
        content: getTextContent(content),
        imageUrls: getImageUrls(content),
      });
    }

    return { model: parsed.model, messages };
  } catch {
    return null;
  }
}

export function extractResponseChat(body: string): ExtractedChat | null {
  try {
    const parsed = JSON.parse(body);
    const message = parsed.choices?.[0]?.message;
    if (!message || typeof message !== "object") return null;

    const content = message.content ?? "";
    const reasoning_content = message.reasoning_content ?? "";

    return {
      model: parsed.model,
      messages: [
        {
          role: message.role || "assistant",
          content: typeof content === "string" ? content : JSON.stringify(content),
          reasoning_content:
            typeof reasoning_content === "string" ? reasoning_content : undefined,
        },
      ],
    };
  } catch {
    return null;
  }
}

export function extractSSEChat(body: string): SSEChat {
  const result: SSEChat = { reasoning: "", content: "" };
  for (const line of body.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed || !trimmed.startsWith("data: ")) continue;
    const data = trimmed.slice(6);
    if (data === "[DONE]") continue;
    try {
      const parsed = JSON.parse(data);
      const delta = parsed.choices?.[0]?.delta;
      if (delta?.content) result.content += delta.content;
      if (delta?.reasoning_content) result.reasoning += delta.reasoning_content;
    } catch {
      // skip unparseable lines
    }
  }
  return result;
}
