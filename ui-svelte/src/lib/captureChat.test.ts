import { describe, it, expect } from "vitest";
import {
	extractRequestChat,
	extractResponseChat,
	extractSSEChat,
	tryFormatJson,
} from "./captureChat";

describe("extractRequestChat", () => {
	it("parses a basic chat request", () => {
		const body = JSON.stringify({
			model: "gpt-4",
			messages: [
				{ role: "system", content: "You are a helpful assistant." },
				{ role: "user", content: "Hello!" },
			],
		});
		const result = extractRequestChat(body);
		expect(result).not.toBeNull();
		expect(result!.model).toBe("gpt-4");
		expect(result!.messages).toHaveLength(2);
		expect(result!.messages[0].role).toBe("system");
		expect(result!.messages[1].role).toBe("user");
	});

	it("parses assistant message with tool_calls", () => {
		const body = JSON.stringify({
			model: "gpt-4",
			messages: [
				{
					role: "assistant",
					content: "Let me check the weather.",
					tool_calls: [
						{
							id: "call_123",
							type: "function",
							function: {
								name: "get_weather",
								arguments: '{"location":"NYC"}',
							},
						},
					],
				},
			],
		});
		const result = extractRequestChat(body);
		expect(result).not.toBeNull();
		expect(result!.messages).toHaveLength(1);
		const msg = result!.messages[0];
		expect(msg.role).toBe("assistant");
		expect(msg.toolCalls).toHaveLength(1);
		expect(msg.toolCalls![0].id).toBe("call_123");
		expect(msg.toolCalls![0].function.name).toBe("get_weather");
		expect(msg.toolCalls![0].function.arguments).toBe('{"location":"NYC"}');
	});

	it("parses tool role message with tool_call_id", () => {
		const body = JSON.stringify({
			model: "gpt-4",
			messages: [
				{
					role: "tool",
					content: '{"temperature": 72}',
					tool_call_id: "call_123",
				},
			],
		});
		const result = extractRequestChat(body);
		expect(result).not.toBeNull();
		const msg = result!.messages[0];
		expect(msg.role).toBe("tool");
		expect(msg.toolCallId).toBe("call_123");
		expect(msg.content).toBe('{"temperature": 72}');
	});

	it("parses request with tool definitions", () => {
		const body = JSON.stringify({
			model: "gpt-4",
			messages: [{ role: "user", content: "Hi" }],
			tools: [
				{
					type: "function",
					function: {
						name: "get_weather",
						description: "Get the weather for a location",
						parameters: {
							type: "object",
							properties: {
								location: { type: "string" },
							},
						},
					},
				},
			],
		});
		const result = extractRequestChat(body);
		expect(result).not.toBeNull();
		expect(result!.tools).toHaveLength(1);
		expect(result!.tools![0].function.name).toBe("get_weather");
		expect(result!.tools![0].function.description).toBe(
			"Get the weather for a location",
		);
		expect(result!.tools![0].function.parameters).toEqual({
			type: "object",
			properties: {
				location: { type: "string" },
			},
		});
	});

	it("skips malformed tool_calls entries", () => {
		const body = JSON.stringify({
			model: "gpt-4",
			messages: [
				{
					role: "assistant",
					content: "",
					tool_calls: [
						{ id: "call_1", type: "function", function: { name: "ok" } },
						{ id: "call_2", type: "function" }, // missing function.name
						null,
						"invalid",
					],
				},
			],
		});
		const result = extractRequestChat(body);
		expect(result).not.toBeNull();
		expect(result!.messages[0].toolCalls).toHaveLength(1);
		expect(result!.messages[0].toolCalls![0].function.name).toBe("ok");
	});
});

describe("extractResponseChat", () => {
	it("parses a standard chat response", () => {
		const body = JSON.stringify({
			model: "gpt-4",
			choices: [
				{
					message: {
						role: "assistant",
						content: "Hello! How can I help?",
					},
				},
			],
		});
		const result = extractResponseChat(body);
		expect(result).not.toBeNull();
		expect(result!.messages[0].content).toBe("Hello! How can I help?");
	});

	it("parses response with tool_calls", () => {
		const body = JSON.stringify({
			model: "gpt-4",
			choices: [
				{
					message: {
						role: "assistant",
						content: "",
						tool_calls: [
							{
								id: "call_abc",
								type: "function",
								function: {
									name: "search_web",
									arguments: '{"query":"golang"}',
								},
							},
						],
					},
				},
			],
		});
		const result = extractResponseChat(body);
		expect(result).not.toBeNull();
		const msg = result!.messages[0];
		expect(msg.role).toBe("assistant");
		expect(msg.toolCalls).toHaveLength(1);
		expect(msg.toolCalls![0].id).toBe("call_abc");
		expect(msg.toolCalls![0].function.name).toBe("search_web");
	});

	it("returns null for non-JSON body", () => {
		const result = extractResponseChat("not json");
		expect(result).toBeNull();
	});

	it("returns null when choices is missing", () => {
		const body = JSON.stringify({ model: "gpt-4" });
		const result = extractResponseChat(body);
		expect(result).toBeNull();
	});
});

describe("extractSSEChat", () => {
	it("extracts content from SSE stream", () => {
		const body = [
			'data: {"choices":[{"delta":{"content":"Hello"}}]}',
			'data: {"choices":[{"delta":{"content":" world"}}]}',
			"data: [DONE]",
		].join("\n");
		const result = extractSSEChat(body);
		expect(result.content).toBe("Hello world");
		expect(result.reasoning).toBe("");
		expect(result.toolCalls).toHaveLength(0);
	});

	it("extracts reasoning_content from SSE stream", () => {
		const body = [
			'data: {"choices":[{"delta":{"reasoning_content":"Let me think..."}}]}',
			'data: {"choices":[{"delta":{"content":"Done"}}]}',
		].join("\n");
		const result = extractSSEChat(body);
		expect(result.reasoning).toBe("Let me think...");
		expect(result.content).toBe("Done");
	});

	it("extracts single tool call from SSE stream", () => {
		const body = [
			'event: tool_calls\ndata: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"get_time","arguments":""}}]}}]}',
			'event: tool_calls\ndata: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\\""}}]}}]}',
			'event: tool_calls\ndata: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"zone\\":\\"UTC\\"}"}}]}}]}',
			"data: [DONE]",
		].join("\n");
		const result = extractSSEChat(body);
		expect(result.toolCalls).toHaveLength(1);
		expect(result.toolCalls[0].id).toBe("call_1");
		expect(result.toolCalls[0].function.name).toBe("get_time");
		expect(result.toolCalls[0].function.arguments).toBe('{"zone":"UTC"}');
	});

	it("extracts multiple tool calls from SSE stream", () => {
		const body = [
			'{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"func_a","arguments":""}}]}}]}',
			'{"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_b","type":"function","function":{"name":"func_b","arguments":""}}]}}]}',
			'{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\\"x\\":1}"}}]}}]}',
			'{"choices":[{"delta":{"tool_calls":[{"index":1,"function":{"arguments":"{\\"y\\":2}"}}]}}]}',
		]
			.map((line) => `data: ${line}`)
			.join("\n");
		const result = extractSSEChat(body);
		expect(result.toolCalls).toHaveLength(2);
		expect(result.toolCalls[0].id).toBe("call_a");
		expect(result.toolCalls[0].function.name).toBe("func_a");
		expect(result.toolCalls[0].function.arguments).toBe('{"x":1}');
		expect(result.toolCalls[1].id).toBe("call_b");
		expect(result.toolCalls[1].function.name).toBe("func_b");
		expect(result.toolCalls[1].function.arguments).toBe('{"y":2}');
	});

	it("ignores unparseable SSE lines", () => {
		const body = [
			"data: not json",
			'data: {"choices":[{"delta":{"content":"ok"}}]}',
			": comment line",
			"",
		].join("\n");
		const result = extractSSEChat(body);
		expect(result.content).toBe("ok");
	});

	it("accumulates tool call arguments across chunks", () => {
		const body = [
			'{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_x","function":{"name":"query","arguments":""}}]}}]}',
			'{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\\"q\\":\\""}}]}}]}',
			'{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"hello"}}]}}]}',
			'{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\\"}"}}]}}]}',
		]
			.map((line) => `data: ${line}`)
			.join("\n");
		const result = extractSSEChat(body);
		expect(result.toolCalls).toHaveLength(1);
		expect(result.toolCalls[0].function.arguments).toBe('{"q":"hello"}');
	});

	it("skips partial tool calls without a name", () => {
		const body = [
			'{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_y","function":{"arguments":"{\\"a\\":1}"}}]}}]}',
		]
			.map((line) => `data: ${line}`)
			.join("\n");
		const result = extractSSEChat(body);
		// No name provided, so tool call should not be included
		expect(result.toolCalls).toHaveLength(0);
	});
});

describe("tryFormatJson", () => {
	it("pretty-prints valid JSON", () => {
		const input = '{"a":1,"b":2}';
		const result = tryFormatJson(input);
		expect(result).toBe(JSON.stringify({ a: 1, b: 2 }, null, 2));
	});

	it("returns original string for invalid JSON", () => {
		const input = "not json";
		const result = tryFormatJson(input);
		expect(result).toBe("not json");
	});

	it("handles empty string", () => {
		const result = tryFormatJson("");
		expect(result).toBe("");
	});

	it("formats JSON arrays", () => {
		const input = "[1,2,3]";
		const result = tryFormatJson(input);
		expect(result).toBe(JSON.stringify([1, 2, 3], null, 2));
	});
});
