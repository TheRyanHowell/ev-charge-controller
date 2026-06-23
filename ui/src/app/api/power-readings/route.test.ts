import { describe, it, expect, vi } from "vitest";

import { GET } from "./route";

describe("GET /api/power-readings", () => {
  it("returns readings from backend", async () => {
    const mockReadings = [{ timestamp: "2024-01-01T00:00:00Z", power: 3500 }];
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(mockReadings),
    });

    const response = await GET({
      url: "http://localhost:3000/api/power-readings",
    });
    const data = await response.json();
    expect(response.status).toBe(200);
    expect(data).toEqual(mockReadings);
  });

  it("strips unknown query params (SSRF protection)", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve([]),
    });

    const apiUrl = process.env.API_URL || "http://localhost:8080";
    await GET({ url: "http://localhost:3000/api/power-readings?limit=100" });
    expect(fetch).toHaveBeenCalledWith(
      `${apiUrl}/api/power-readings`,
      expect.any(Object),
    );
  });

  it("returns 204 for empty response", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
    });

    const response = await GET({
      url: "http://localhost:3000/api/power-readings",
    });
    expect(response.status).toBe(204);
  });

  it("returns 500 on network error", async () => {
    global.fetch = async () => {
      throw new Error("Network error");
    };

    const response = await GET({
      url: "http://localhost:3000/api/power-readings",
    });
    expect(response.status).toBe(500);
  });

  it("returns RFC 7807 when backend returns error", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 502,
    });

    const response = await GET({
      url: "http://localhost:3000/api/power-readings",
    });
    expect(response.status).toBe(502);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
    const data = await response.json();
    expect(data).toHaveProperty("type");
    expect(data).toHaveProperty("detail", "Failed to fetch power readings");
  });
});
