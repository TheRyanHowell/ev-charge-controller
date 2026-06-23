import { describe, it, expect, vi } from "vitest";

import { GET } from "./route";

describe("GET /api/soc-snapshots", () => {
  it("returns snapshots from backend", async () => {
    const mockSnapshots = [{ timestamp: "2024-01-01T00:00:00Z", soc: 75 }];
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(mockSnapshots),
    });

    const response = await GET({
      url: "http://localhost:3000/api/soc-snapshots",
    });
    const data = await response.json();
    expect(response.status).toBe(200);
    expect(data).toEqual(mockSnapshots);
  });

  it("strips unknown query params (SSRF protection)", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve([]),
    });

    const apiUrl = process.env.API_URL || "http://localhost:8080";
    await GET({ url: "http://localhost:3000/api/soc-snapshots?limit=50" });
    expect(fetch).toHaveBeenCalledWith(
      `${apiUrl}/api/soc-snapshots`,
      expect.any(Object),
    );
  });

  it("returns 204 for empty response", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
    });

    const response = await GET({
      url: "http://localhost:3000/api/soc-snapshots",
    });
    expect(response.status).toBe(204);
  });

  it("returns 500 on network error", async () => {
    global.fetch = async () => {
      throw new Error("Network error");
    };

    const response = await GET({
      url: "http://localhost:3000/api/soc-snapshots",
    });
    expect(response.status).toBe(500);
  });

  it("returns RFC 7807 when backend returns error", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 502,
    });

    const response = await GET({
      url: "http://localhost:3000/api/soc-snapshots",
    });
    expect(response.status).toBe(502);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
    const data = await response.json();
    expect(data).toHaveProperty("type");
    expect(data).toHaveProperty("detail", "Failed to fetch SOC snapshots");
  });
});
