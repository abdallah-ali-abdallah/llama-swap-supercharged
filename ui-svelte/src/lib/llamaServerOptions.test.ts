import { describe, expect, it } from "vitest";
import { LLAMA_SERVER_OPTIONS, interpretLlamaServerCommand, parseCommandLine } from "./llamaServerOptions";

describe("llamaServerOptions", () => {
  it("parses quoted command lines", () => {
    expect(parseCommandLine(`llama-server --model "/models/Qwen 3.gguf" --api-prefix '/llama api'`)).toEqual([
      "llama-server",
      "--model",
      "/models/Qwen 3.gguf",
      "--api-prefix",
      "/llama api",
    ]);
  });

  it("interprets llama-server flags with aliases and inline values", () => {
    const result = interpretLlamaServerCommand(
      "llama-server -m /models/model.gguf -c 4096 -fa 1 --n-gpu-layers=99 -np 4 --no-mmap",
    );

    expect(result.provider).toBe("llama.cpp");
    expect(result.unknownOptions).toEqual([]);
    expect(result.options.map((item) => [item.option.id, item.value])).toEqual([
      ["model", "/models/model.gguf"],
      ["ctx-size", "4096"],
      ["flash-attn", "1"],
      ["n-gpu-layers", "99"],
      ["parallel", "4"],
      ["no-mmap", null],
    ]);
    expect(result.highlights).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ id: "ctx-size", value: "4,096 tokens" }),
        expect.objectContaining({ id: "flash-attn", value: "on" }),
        expect.objectContaining({ id: "n-gpu-layers", value: "99" }),
        expect.objectContaining({ id: "vision", value: "disabled" }),
      ]),
    );
  });

  it("shows vision as enabled when a multimodal projector is configured", () => {
    const result = interpretLlamaServerCommand("llama-server -m /models/model.gguf --mmproj /models/mmproj.gguf");

    expect(result.highlights).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          id: "vision",
          value: "enabled",
          note: "Multimodal processing is enabled with /models/mmproj.gguf.",
        }),
      ]),
    );
  });

  it("shows vision state for URL, auto, explicit disabled, and absent projector cases", () => {
    expect(
      interpretLlamaServerCommand("llama-server --mmproj-url https://example.test/mmproj.gguf").highlights,
    ).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          id: "vision",
          value: "enabled",
          note: "Multimodal processing is enabled with https://example.test/mmproj.gguf.",
        }),
      ]),
    );

    expect(interpretLlamaServerCommand("llama-server --mmproj-auto").highlights).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          id: "vision",
          value: "auto",
          note: "llama.cpp will automatically use an available multimodal projector.",
        }),
      ]),
    );

    expect(interpretLlamaServerCommand("llama-server --no-mmproj").highlights).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          id: "vision",
          value: "disabled",
          note: "Multimodal projector loading is explicitly disabled.",
          tone: "warning",
        }),
      ]),
    );

    expect(interpretLlamaServerCommand("llama-server -m /models/model.gguf").highlights).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          id: "vision",
          value: "disabled",
          note: "No multimodal projector is configured.",
        }),
      ]),
    );
  });

  it("uses upstream batch-size wording", () => {
    expect(LLAMA_SERVER_OPTIONS.find((option) => option.id === "batch-size")).toEqual(
      expect.objectContaining({
        label: "Logical maximum batch size",
        explanation: "Sets the logical maximum batch size.",
      }),
    );
    expect(LLAMA_SERVER_OPTIONS.find((option) => option.id === "ubatch-size")).toEqual(
      expect.objectContaining({
        label: "Physical maximum batch size",
        explanation: "Sets the physical maximum batch size, also known as the micro-batch size.",
      }),
    );

    const result = interpretLlamaServerCommand("llama-server -b 4096 -ub 2048");
    expect(result.highlights).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          id: "batch-size",
          note: "Logical maximum batch size used during prompt processing.",
        }),
        expect.objectContaining({
          id: "ubatch-size",
          label: "Micro-batch",
          note: "Physical maximum batch size, also known as the micro-batch size.",
        }),
      ]),
    );
  });

  it("keeps unrecognized flags visible", () => {
    const result = interpretLlamaServerCommand("llama-server --future-flag enabled --ctx-size 8192");

    expect(result.unknownOptions).toEqual([{ flag: "--future-flag", value: "enabled", raw: "--future-flag enabled" }]);
    expect(result.options.find((item) => item.option.id === "ctx-size")?.value).toBe("8192");
  });
});
