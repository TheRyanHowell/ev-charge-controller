import { describe, it, expect, vi } from "vitest";

import { PATCH, DELETE } from "./route";

function makeRequest(vehicleId: string, body: Record<string, unknown>) {
  return {
    json: async () => body,
  };
}

function makeParams(vehicleId: string) {
  return {
    params: Promise.resolve({ vehicleId }),
  };
}

describe("PATCH /api/vehicles/:vehicleId", () => {
  it("proxies PATCH to backend with body", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 204,
    });

    const req = makeRequest("rm1", { currentPercent: 50, targetPercent: 80 });
    const ctx = makeParams("rm1");
    const response = await PATCH(req as never, ctx);

    expect(response.status).toBe(204);
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/vehicles/rm1"),
      expect.objectContaining({
        method: "PATCH",
        body: JSON.stringify({ currentPercent: 50, targetPercent: 80 }),
      }),
    );
  });

  it("returns 200 with JSON body on success", async () => {
    const mockData = { id: "rm1", currentPercent: 50 };
    global.fetch = vi.fn().mockResolvedValue({
      status: 200,
      ok: true,
      text: async () => JSON.stringify(mockData),
      headers: { get: () => "application/json" },
    });

    const req = makeRequest("rm1", { currentPercent: 50 });
    const ctx = makeParams("rm1");
    const response = await PATCH(req as never, ctx);
    const data = await response.json();

    expect(response.status).toBe(200);
    expect(data).toEqual(mockData);
  });

  it("passes through non-JSON response", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 500,
      ok: false,
      text: async () => "Internal Server Error",
      headers: { get: () => "text/plain" },
    });

    const req = makeRequest("rm1", { currentPercent: 50 });
    const ctx = makeParams("rm1");
    const response = await PATCH(req as never, ctx);
    const text = await response.text();

    expect(response.status).toBe(500);
    expect(text).toBe("Internal Server Error");
  });

  it("returns RFC 7807 on network error", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

    const req = makeRequest("rm1", { currentPercent: 50 });
    const ctx = makeParams("rm1");
    const response = await PATCH(req as never, ctx);

    expect(response.status).toBe(500);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
    const data = await response.json();
    expect(data).toHaveProperty("type");
    expect(data).toHaveProperty("status", 500);
  });

  it("handles empty response body", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 404,
      ok: false,
      text: async () => "",
    });

    const req = makeRequest("nonexistent", { currentPercent: 50 });
    const ctx = makeParams("nonexistent");
    const response = await PATCH(req as never, ctx);

    expect(response.status).toBe(404);
  });

  it("returns 400 on invalid JSON body", async () => {
    const req = {
      json: async () => {
        throw new Error("invalid json");
      },
    };
    const ctx = makeParams("rm1");
    const response = await PATCH(req as never, ctx);

    expect(response.status).toBe(400);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
    const data = await response.json();
    expect(data.detail).toBe("Invalid request body");
  });

  it("returns 400 on invalid vehicle ID format", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 204,
    });

    const req = makeRequest("rm1", { currentPercent: 50 });
    const ctx = makeParams("invalid/id");
    const response = await PATCH(req as never, ctx);

    expect(response.status).toBe(400);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
    const data = await response.json();
    expect(data.detail).toBe("Invalid vehicle ID format");
  });
});

describe("DELETE /api/vehicles/:vehicleId", () => {
  it("proxies DELETE to backend", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 204,
    });

    const req = {} as never;
    const ctx = makeParams("rm1");
    const response = await DELETE(req, ctx);

    expect(response.status).toBe(204);
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/vehicles/rm1"),
      expect.objectContaining({
        method: "DELETE",
      }),
    );
  });

  it("returns 400 on invalid vehicle ID format", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 204,
    });

    const req = {} as never;
    const ctx = makeParams("invalid/id");
    const response = await DELETE(req, ctx);

    expect(response.status).toBe(400);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
    const data = await response.json();
    expect(data.detail).toBe("Invalid vehicle ID format");
  });

  it("passes through error response from backend", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 500,
      ok: false,
      text: async () => "Internal Server Error",
      headers: { get: () => "text/plain" },
    });

    const req = {} as never;
    const ctx = makeParams("rm1");
    const response = await DELETE(req, ctx);
    const text = await response.text();

    expect(response.status).toBe(500);
    expect(text).toBe("Internal Server Error");
  });

  it("returns 500 on network error", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

    const req = {} as never;
    const ctx = makeParams("rm1");
    const response = await DELETE(req, ctx);

    expect(response.status).toBe(500);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
  });
});
