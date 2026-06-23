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

describe("plugs/[plugId]/regenerate-configuration API route", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("POST regenerates config and returns console commands", async () => {
    mockFetch.mockResolvedValueOnce({
      status: 200,
      ok: true,
      headers: new Headers({ "Content-Type": "application/json" }),
      text: async () =>
        JSON.stringify({
          consoleCommands: "MqttPassword newpass123\nMqttHost mqtt.example.com",
        }),
    });

    const req = new Request(
      "http://localhost/api/plugs/plug1/regenerate-configuration",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
      },
    );

    const resp = await routes.POST(req as never, makeParams("plug1"));
    const body = await resp.json();

    expect(resp.status).toBe(200);
    expect(body.consoleCommands).toContain("MqttPassword");
    expect(mockFetch).toHaveBeenCalledWith(
      `${API_URL}/api/plugs/plug1/regenerate-configuration`,
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({}),
      }),
    );
  });

  it("returns 500 on network error", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const req = new Request(
      "http://localhost/api/plugs/plug1/regenerate-configuration",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
      },
    );

    const resp = await routes.POST(req as never, makeParams("plug1"));
    expect(resp.status).toBe(500);
  });

  it("passes through non-JSON response", async () => {
    mockFetch.mockResolvedValueOnce({
      status: 500,
      ok: false,
      headers: new Headers({ "Content-Type": "text/plain" }),
      text: async () => "Internal Server Error",
    });

    const req = new Request(
      "http://localhost/api/plugs/plug1/regenerate-configuration",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
      },
    );

    const resp = await routes.POST(req as never, makeParams("plug1"));
    const text = await resp.text();
    expect(resp.status).toBe(500);
    expect(text).toBe("Internal Server Error");
  });
});
