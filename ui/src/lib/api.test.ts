import {
  ApiError,
  apiGet,
  apiGetSingle,
  apiOk,
  apiPatchNoContent,
  apiPatchRaw,
  apiPost,
  apiPostNullable,
  apiDelete,
} from "@/lib/api";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { z } from "zod";

// Mock fetch globally
const mockFetch = vi.fn();
beforeEach(() => {
  vi.stubGlobal("fetch", mockFetch);
  mockFetch.mockReset();
});
afterEach(() => {
  vi.unstubAllGlobals();
});

function okResponse(body: unknown, status = 200) {
  return {
    ok: status < 400,
    status,
    json: async () => body,
  };
}

function errorResponse(status: number, body = { title: "Bad Request" }) {
  return {
    ok: false,
    status,
    json: async () => body,
  };
}

describe("apiGet", () => {
  it("returns parsed array on success", async () => {
    mockFetch.mockResolvedValue(okResponse([{ id: "1" }]));
    const result = await apiGet("/test", z.object({ id: z.string() }));
    expect(result).toEqual([{ id: "1" }]);
  });

  it("returns empty array on 204", async () => {
    mockFetch.mockResolvedValue(okResponse(null, 204));
    const result = await apiGet("/test", z.object({ id: z.string() }));
    expect(result).toEqual([]);
  });

  it("throws ApiError on non-ok response", async () => {
    mockFetch.mockResolvedValue(errorResponse(400));
    await expect(apiGet("/test", z.object({ id: z.string() }))).rejects.toThrow(
      ApiError,
    );
  });

  it("throws Error on network failure", async () => {
    mockFetch.mockRejectedValue(new Error("network error"));
    await expect(apiGet("/test", z.object({ id: z.string() }))).rejects.toThrow(
      "Something went wrong",
    );
  });

  it("passes signal to fetch", async () => {
    const signal = AbortSignal.timeout(5000);
    mockFetch.mockResolvedValue(okResponse([]));
    await apiGet("/test", z.object({ id: z.string() }), { signal });
    expect(mockFetch).toHaveBeenCalledWith("/test", {
      cache: "no-store",
      signal,
    });
  });
});

describe("apiGetSingle", () => {
  it("returns parsed object on success", async () => {
    mockFetch.mockResolvedValue(okResponse({ id: "1" }));
    const result = await apiGetSingle("/test", z.object({ id: z.string() }));
    expect(result).toEqual({ id: "1" });
  });

  it("returns null on 204", async () => {
    mockFetch.mockResolvedValue(okResponse(null, 204));
    const result = await apiGetSingle("/test", z.object({ id: z.string() }));
    expect(result).toBeNull();
  });

  it("throws ApiError on non-ok response", async () => {
    mockFetch.mockResolvedValue(errorResponse(404));
    await expect(
      apiGetSingle("/test", z.object({ id: z.string() })),
    ).rejects.toThrow(ApiError);
  });

  it("throws Error on network failure", async () => {
    mockFetch.mockRejectedValue(new Error("network error"));
    await expect(
      apiGetSingle("/test", z.object({ id: z.string() })),
    ).rejects.toThrow("Something went wrong");
  });
});

describe("apiPost", () => {
  it("returns parsed object on success", async () => {
    mockFetch.mockResolvedValue(okResponse({ id: "new" }));
    const result = await apiPost("/test", z.object({ id: z.string() }), {
      name: "test",
    });
    expect(result).toEqual({ id: "new" });
  });

  it("throws ApiError on non-ok response", async () => {
    mockFetch.mockResolvedValue(errorResponse(400));
    await expect(
      apiPost("/test", z.object({ id: z.string() }), { name: "test" }),
    ).rejects.toThrow(ApiError);
  });

  it("throws Error on network failure", async () => {
    mockFetch.mockRejectedValue(new Error("network error"));
    await expect(
      apiPost("/test", z.object({ id: z.string() }), { name: "test" }),
    ).rejects.toThrow("Something went wrong");
  });
});

describe("apiPostNullable", () => {
  it("returns parsed object on success", async () => {
    mockFetch.mockResolvedValue(okResponse({ id: "new" }));
    const result = await apiPostNullable(
      "/test",
      z.object({ id: z.string() }),
      {
        name: "test",
      },
    );
    expect(result).toEqual({ id: "new" });
  });

  it("returns null on 204", async () => {
    mockFetch.mockResolvedValue(okResponse(null, 204));
    const result = await apiPostNullable(
      "/test",
      z.object({ id: z.string() }),
      {
        name: "test",
      },
    );
    expect(result).toBeNull();
  });

  it("throws ApiError on non-ok response", async () => {
    mockFetch.mockResolvedValue(errorResponse(400));
    await expect(
      apiPostNullable("/test", z.object({ id: z.string() }), { name: "test" }),
    ).rejects.toThrow(ApiError);
  });

  it("throws Error on network failure", async () => {
    mockFetch.mockRejectedValue(new Error("network error"));
    await expect(
      apiPostNullable("/test", z.object({ id: z.string() }), { name: "test" }),
    ).rejects.toThrow("Something went wrong");
  });
});

describe("apiPatchRaw", () => {
  it("returns true on success", async () => {
    mockFetch.mockResolvedValue(okResponse(null, 204));
    const result = await apiPatchRaw("/test", { name: "test" });
    expect(result).toBe(true);
  });

  it("returns false on non-ok response", async () => {
    mockFetch.mockResolvedValue(errorResponse(400));
    const result = await apiPatchRaw("/test", { name: "test" });
    expect(result).toBe(false);
  });

  it("returns false on network failure", async () => {
    mockFetch.mockRejectedValue(new Error("network error"));
    const result = await apiPatchRaw("/test", { name: "test" });
    expect(result).toBe(false);
  });
});

describe("apiPatchNoContent", () => {
  it("resolves on success", async () => {
    mockFetch.mockResolvedValue(okResponse(null, 204));
    await expect(apiPatchNoContent("/test", { name: "test" })).resolves.toBe(
      undefined,
    );
  });

  it("throws ApiError on non-ok response", async () => {
    mockFetch.mockResolvedValue(errorResponse(400));
    await expect(apiPatchNoContent("/test", { name: "test" })).rejects.toThrow(
      ApiError,
    );
  });

  it("throws Error on network failure", async () => {
    mockFetch.mockRejectedValue(new Error("network error"));
    await expect(apiPatchNoContent("/test", { name: "test" })).rejects.toThrow(
      "Something went wrong",
    );
  });
});

describe("apiDelete", () => {
  it("resolves on success", async () => {
    mockFetch.mockResolvedValue(okResponse(null, 204));
    await expect(apiDelete("/test")).resolves.toBe(undefined);
  });

  it("throws ApiError on non-ok response", async () => {
    mockFetch.mockResolvedValue(errorResponse(404));
    await expect(apiDelete("/test")).rejects.toThrow(ApiError);
  });

  it("throws Error on network failure", async () => {
    mockFetch.mockRejectedValue(new Error("network error"));
    await expect(apiDelete("/test")).rejects.toThrow("Something went wrong");
  });
});

describe("apiOk", () => {
  it("returns true on ok response", async () => {
    mockFetch.mockResolvedValue(okResponse(null, 200));
    const result = await apiOk("/test");
    expect(result).toBe(true);
  });

  it("returns false on non-ok response", async () => {
    mockFetch.mockResolvedValue(errorResponse(500));
    const result = await apiOk("/test");
    expect(result).toBe(false);
  });

  it("returns false on network failure", async () => {
    mockFetch.mockRejectedValue(new Error("network error"));
    const result = await apiOk("/test");
    expect(result).toBe(false);
  });
});

describe("ApiError", () => {
  it("has correct name and status", () => {
    const err = new ApiError("not found", 404);
    expect(err.name).toBe("ApiError");
    expect(err.message).toBe("not found");
    expect(err.status).toBe(404);
  });
});
