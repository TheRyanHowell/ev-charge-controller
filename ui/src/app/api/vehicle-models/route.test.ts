import { describe, it, expect, vi } from "vitest";

import { GET } from "./route";

describe("GET /api/vehicle-models", () => {
  it("returns vehicle models from backend", async () => {
    const mockModels = [
      { id: "rm1", name: "Maeving RM1", capacityKwh: 2.026 },
      { id: "rm1s", name: "Maeving RM1S", capacityKwh: 5.46 },
    ];
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(mockModels),
    });

    const response = await GET();
    const data = await response.json();

    expect(response.status).toBe(200);
    expect(data).toEqual(mockModels);
  });

  it("returns 204 for empty response", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
    });

    const response = await GET();
    expect(response.status).toBe(204);
  });

  it("returns 500 on network error", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

    const response = await GET();
    expect(response.status).toBe(500);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
  });

  it("returns RFC 7807 when backend returns error", async () => {
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
    expect(data).toHaveProperty("detail", "Failed to fetch vehicle models");
  });
});
