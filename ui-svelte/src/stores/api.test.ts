import { afterEach, describe, expect, it, vi } from "vitest";
import { getModelConfiguration } from "./api";

describe("api", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("fetches model configuration with URL-encoded model IDs", async () => {
    const configuration = {
      modelID: "team/qwen model",
      cmd: "llama-server -c 4096",
      proxy: "http://127.0.0.1:5800",
      checkEndpoint: "/health",
      ttl: 30,
      yaml: "cmd: llama-server -c 4096",
    };
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: vi.fn().mockResolvedValue(configuration),
    });
    vi.stubGlobal("fetch", fetchMock);

    await expect(getModelConfiguration("team/qwen model")).resolves.toEqual(configuration);
    expect(fetchMock).toHaveBeenCalledWith("/api/models/config/team%2Fqwen%20model");
  });

  it("returns null when model configuration is not found", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 404,
      }),
    );

    await expect(getModelConfiguration("missing")).resolves.toBeNull();
  });

  it("returns null when fetching model configuration fails", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("network unavailable")));

    await expect(getModelConfiguration("model")).resolves.toBeNull();
  });
});
