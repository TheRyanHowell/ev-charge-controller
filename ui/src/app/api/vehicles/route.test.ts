import { describe, it, expect, vi } from "vitest";

import { GET, POST } from "./route";

describe("GET /api/vehicles", () => {
  it("returns vehicles on success", async () => {
    const mockVehicles = [
      { id: "rm1", name: "Maeving RM1", capacity_kwh: 2.026 },
    ];
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockVehicles),
    });

    const response = await GET();
    const data = await response.json();

    expect(response.status).toBe(200);
    expect(data).toEqual(mockVehicles);
  });

  it("returns RFC 7807 when backend is unavailable", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 502,
    });

    const response = await GET();
    expect(response.status).toBe(502);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
    const data = await response.json();
    expect(data).toHaveProperty("type");
    expect(data).toHaveProperty("title");
    expect(data).toHaveProperty("status", 502);
    expect(data).toHaveProperty("detail", "Failed to fetch vehicles");
  });

  it("returns 500 on network error", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

    const response = await GET();
    expect(response.status).toBe(500);
  });
});

describe("POST /api/vehicles", () => {
  it("proxies POST to backend with body", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 201,
      ok: true,
      text: async () => JSON.stringify({ id: "rm1" }),
      headers: { get: () => "application/json" },
    });

    const req = {
      json: async () => ({ name: "Maeving RM1", capacity_kwh: 2.026 }),
    };

    const response = await POST(req as never);
    const data = await response.json();

    expect(response.status).toBe(201);
    expect(data).toEqual({ id: "rm1" });
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/vehicles"),
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ name: "Maeving RM1", capacity_kwh: 2.026 }),
      }),
    );
  });

  it("returns 400 on invalid JSON body", async () => {
    const req = {
      json: async () => {
        throw new SyntaxError("invalid json");
      },
    };

    const response = await POST(req as never);

    expect(response.status).toBe(400);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
    const data = await response.json();
    expect(data.detail).toBe("Invalid request body");
  });
});
