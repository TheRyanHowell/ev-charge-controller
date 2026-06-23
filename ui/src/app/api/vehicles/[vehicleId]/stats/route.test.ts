import { describe, it, expect, vi, beforeEach } from "vitest";

const API_URL = process.env.API_URL || "http://localhost:8080";

const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

vi.mock("@/lib/problem-details", () => ({
  problemResponse: (detail: string, status: number) =>
    new Response(
      JSON.stringify({ type: "about:blank", title: "Error", status, detail }),
      {
        status,
        headers: { "Content-Type": "application/problem+json" },
      },
    ),
}));

const routes = await import("./route");

function makeRequest(url: string) {
  const parsed = new URL(url);
  return {
    nextUrl: {
      searchParams: {
        get: (key: string) => parsed.searchParams.get(key),
      },
    },
  } as never;
}

function makeParams(vehicleId: string) {
  return {
    params: Promise.resolve({ vehicleId }),
  };
}

describe("vehicles/[vehicleId]/stats API route", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("GET returns stats from backend", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () =>
        Promise.resolve({
          totalSessions: 42,
          totalEnergyKwh: 85.5,
          avgSessionDuration: 3600,
        }),
    });

    const resp = await routes.GET(
      makeRequest("http://localhost/api/vehicles/rm1/stats"),
      makeParams("rm1"),
    );
    const body = await resp.json();

    expect(resp.status).toBe(200);
    expect(body.totalSessions).toBe(42);
    expect(mockFetch).toHaveBeenCalledWith(
      `${API_URL}/api/vehicles/rm1/stats?range=lifetime`,
      expect.any(Object),
    );
  });

  it("passes range query parameter to backend", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ totalSessions: 5 }),
    });

    await routes.GET(
      makeRequest("http://localhost/api/vehicles/rm1/stats?range=7d"),
      makeParams("rm1"),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      `${API_URL}/api/vehicles/rm1/stats?range=7d`,
      expect.any(Object),
    );
  });

  it("defaults range to lifetime when not provided", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ totalSessions: 0 }),
    });

    await routes.GET(
      makeRequest("http://localhost/api/vehicles/rm1/stats"),
      makeParams("rm1"),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      `${API_URL}/api/vehicles/rm1/stats?range=lifetime`,
      expect.any(Object),
    );
  });

  it("returns 400 for invalid vehicle ID (path traversal)", async () => {
    const resp = await routes.GET(
      makeRequest("http://localhost/api/vehicles/../../etc/passwd/stats"),
      makeParams("../../etc/passwd"),
    );

    expect(resp.status).toBe(400);
    expect(resp.headers.get("content-type")).toContain(
      "application/problem+json",
    );
    const body = await resp.json();
    expect(body.detail).toBe("Invalid vehicle ID format");
  });

  it("returns 400 for empty vehicle ID", async () => {
    const resp = await routes.GET(
      makeRequest("http://localhost/api/vehicles//stats"),
      makeParams(""),
    );

    expect(resp.status).toBe(400);
    const body = await resp.json();
    expect(body.detail).toBe("Invalid vehicle ID format");
  });

  it("returns 204 for empty response", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 204,
    });

    const resp = await routes.GET(
      makeRequest("http://localhost/api/vehicles/rm1/stats"),
      makeParams("rm1"),
    );
    expect(resp.status).toBe(204);
  });

  it("returns RFC 7807 on network error", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const resp = await routes.GET(
      makeRequest("http://localhost/api/vehicles/rm1/stats"),
      makeParams("rm1"),
    );
    expect(resp.status).toBe(500);
    expect(resp.headers.get("content-type")).toContain(
      "application/problem+json",
    );
  });

  it("returns RFC 7807 when backend returns error", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 404,
    });

    const resp = await routes.GET(
      makeRequest("http://localhost/api/vehicles/rm1/stats"),
      makeParams("rm1"),
    );
    expect(resp.status).toBe(404);
    const body = await resp.json();
    expect(body.detail).toBe("Failed to fetch vehicle stats");
  });
});
