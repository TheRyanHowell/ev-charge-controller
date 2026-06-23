import { describe, it, expect, vi, beforeEach } from "vitest";

const API_URL = process.env.API_URL || "http://localhost:8080";

const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

const routes = await import("./route");

function makeParams(plugId: string) {
  return {
    params: Promise.resolve({ plugId }),
  };
}

describe("plugs/[plugId]/provisioning API route", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("GET returns provisioning info from backend", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: () =>
        Promise.resolve({
          mqttHost: "mqtt.example.com",
          mqttPort: 1883,
          mqttUsername: "plug1",
          fullTopic: "evcc/plug1",
          topic: "plug1",
        }),
    });

    const resp = await routes.GET(
      new Request("http://localhost/api/plugs/plug1/provisioning") as never,
      makeParams("plug1"),
    );
    const body = await resp.json();

    expect(resp.status).toBe(200);
    expect(body.mqttUsername).toBe("plug1");
    expect(mockFetch).toHaveBeenCalledWith(
      `${API_URL}/api/plugs/plug1/provisioning`,
      expect.any(Object),
    );
  });

  it("returns 204 for empty response", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 204,
    });

    const resp = await routes.GET(
      new Request("http://localhost/api/plugs/plug1/provisioning") as never,
      makeParams("plug1"),
    );
    expect(resp.status).toBe(204);
  });

  it("returns RFC 7807 on network error", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const resp = await routes.GET(
      new Request("http://localhost/api/plugs/plug1/provisioning") as never,
      makeParams("plug1"),
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
      new Request("http://localhost/api/plugs/plug1/provisioning") as never,
      makeParams("plug1"),
    );
    expect(resp.status).toBe(404);
    const body = await resp.json();
    expect(body.detail).toBe("Failed to fetch provisioning info");
  });
});
