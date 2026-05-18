import { getTextContent, getImageUrls } from "./types";
import type { ContentPart, ToolCall, ToolDefinition } from "./types";

export interface CaptureChatMessage {
	role: "user" | "assistant" | "system" | "tool";
	content: string;
	reasoning_content?: string;
	imageUrls?: string[];
	toolCalls?: ToolCall[];
	toolCallId?: string;
}

export interface ExtractedChat {
	model?: string;
	messages: CaptureChatMessage[];
	tools?: ToolDefinition[];
}

export interface SSEChat {
	reasoning: string;
	content: string;
	toolCalls: ToolCall[];
}

/** Convert literal escape sequences like \\n → actual newline */
export function unescapeLiteral(text: string): string {
	return text
		.replace(/\\n/g, "\n")
		.replace(/\\t/g, "\t")
		.replace(/\\r/g, "\r")
		.replace(/\\\\/g, "\\");
}

function parseToolCalls(raw: unknown): ToolCall[] | undefined {
	if (!Array.isArray(raw)) return undefined;
	const calls: ToolCall[] = [];
	for (const item of raw) {
		if (!item || typeof item !== "object") continue;
		const tc = item as Record<string, unknown>;
		const fn = tc.function as Record<string, unknown> | undefined;
		if (!fn || typeof fn.name !== "string") continue;
		calls.push({
			id: typeof tc.id === "string" ? tc.id : "",
			type: (tc.type === "function"
				? "function"
				: typeof tc.type === "string"
					? tc.type
					: "function") as "function",
			function: {
				name: fn.name,
				arguments: typeof fn.arguments === "string" ? fn.arguments : "",
			},
		});
	}
	return calls.length > 0 ? calls : undefined;
}

function parseToolDefinitions(raw: unknown): ToolDefinition[] | undefined {
	if (!Array.isArray(raw)) return undefined;
	const tools: ToolDefinition[] = [];
	for (const item of raw) {
		if (!item || typeof item !== "object") continue;
		const t = item as Record<string, unknown>;
		const fn = t.function as Record<string, unknown> | undefined;
		if (!fn || typeof fn.name !== "string") continue;
		tools.push({
			type: (t.type === "function"
				? "function"
				: typeof t.type === "string"
					? t.type
					: "function") as "function",
			function: {
				name: fn.name,
				description:
					typeof fn.description === "string" ? fn.description : undefined,
				parameters:
					typeof fn.parameters === "object" && fn.parameters !== null
						? (fn.parameters as Record<string, unknown>)
						: undefined,
			},
		});
	}
	return tools.length > 0 ? tools : undefined;
}

export function extractRequestChat(body: string): ExtractedChat | null {
	try {
		const parsed = JSON.parse(body);
		if (!Array.isArray(parsed.messages)) return null;

		const messages: CaptureChatMessage[] = [];
		for (const msg of parsed.messages) {
			if (!msg || typeof msg !== "object") continue;
			const role = msg.role;
			if (
				role !== "user" &&
				role !== "assistant" &&
				role !== "system" &&
				role !== "tool"
			)
				continue;

			const content: string | ContentPart[] = msg.content ?? "";
			const toolCalls = parseToolCalls(msg.tool_calls);
			const toolCallId =
				typeof msg.tool_call_id === "string" ? msg.tool_call_id : undefined;

			messages.push({
				role,
				content: unescapeLiteral(getTextContent(content)),
				imageUrls: getImageUrls(content),
				toolCalls,
				toolCallId,
			});
		}

		const tools = parseToolDefinitions(parsed.tools);
		return { model: parsed.model, messages, tools };
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
		const toolCalls = parseToolCalls(message.tool_calls);

		return {
			model: parsed.model,
			messages: [
				{
					role: message.role || "assistant",
					content: unescapeLiteral(
						typeof content === "string" ? content : JSON.stringify(content),
					),
					reasoning_content:
						typeof reasoning_content === "string"
							? unescapeLiteral(reasoning_content)
							: undefined,
					toolCalls,
				},
			],
		};
	} catch {
		return null;
	}
}

export function extractSSEChat(body: string): SSEChat {
	const result: SSEChat = { reasoning: "", content: "", toolCalls: [] };
	const toolCallAccumulator: Record<number, Partial<ToolCall>> = {};

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

			const deltaToolCalls = delta?.tool_calls;
			if (Array.isArray(deltaToolCalls)) {
				for (const dtc of deltaToolCalls) {
					if (!dtc || typeof dtc !== "object") continue;
					const index = typeof dtc.index === "number" ? dtc.index : 0;
					const existing = toolCallAccumulator[index] || {};

					if (dtc.id) existing.id = dtc.id;
					if (dtc.type) existing.type = dtc.type;

					const fn = dtc.function as Record<string, string> | undefined;
					if (fn) {
						if (!existing.function)
							existing.function = { name: "", arguments: "" };
						if (fn.name) existing.function.name = fn.name;
						if (fn.arguments)
							existing.function.arguments =
								(existing.function.arguments || "") + fn.arguments;
					}

					toolCallAccumulator[index] = existing;
				}
			}
		} catch {
			// skip unparseable lines
		}
	}

	// Convert accumulated partial tool calls into full ToolCall objects
	const indices = Object.keys(toolCallAccumulator)
		.map(Number)
		.sort((a, b) => a - b);
	for (const idx of indices) {
		const acc = toolCallAccumulator[idx];
		if (acc.function && acc.function.name) {
			result.toolCalls.push({
				id: typeof acc.id === "string" ? acc.id : "",
				type: (acc.type === "function"
					? "function"
					: typeof acc.type === "string"
						? acc.type
						: "function") as "function",
				function: {
					name: acc.function.name,
					arguments: acc.function.arguments || "",
				},
			});
		}
	}

	return {
		reasoning: unescapeLiteral(result.reasoning),
		content: unescapeLiteral(result.content),
		toolCalls: result.toolCalls,
	};
}

export function tryFormatJson(value: string): string {
	try {
		const parsed = JSON.parse(value);
		return JSON.stringify(parsed, null, 2);
	} catch {
		return value;
	}
}
