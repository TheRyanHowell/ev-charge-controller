import { describe, it, expect, vi } from "vitest";

import { GET } from "./route";

describe("GET /api/history", () => {
  it("forwards request and returns 200", async () => {
    const mockSessions = [{ id: "sess1", status: "completed" }];
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockSessions),
    });

    const request = { url: "http://localhost:3000/api/history" };
    const response = await GET(request);
    const data = await response.json();

    expect(response.status).toBe(200);
    expect(data).toEqual(mockSessions);
  });

  it("passes query params to backend", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([]),
    });

    const request = {
      url: "http://localhost:3000/api/history?vehicleId=rm1",
    };
    await GET(request);

    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("vehicleId=rm1"),
      expect.any(Object),
    );
  });

  it("returns RFC 7807 on backend error", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 502,
    });

    const request = { url: "http://localhost:3000/api/history" };
    const response = await GET(request);
    expect(response.status).toBe(502);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
    const data = await response.json();
    expect(data).toHaveProperty("type");
    expect(data).toHaveProperty("status", 502);
    expect(data).toHaveProperty("detail");
  });

  it("returns RFC 7807 on network error", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

    const request = { url: "http://localhost:3000/api/history" };
    const response = await GET(request);
    expect(response.status).toBe(500);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
  });

  it("passes through 204 status code", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 204,
    });

    const request = { url: "http://localhost:3000/api/history" };
    const response = await GET(request);
    expect(response.status).toBe(204);
  });
});
