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

function makeParams(plugId: string) {
  return {
    params: Promise.resolve({ plugId }),
  };
}

describe("plugs/[plugId]/configure API route", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("POST configures plug and returns 204", async () => {
    mockFetch.mockResolvedValueOnce({
      status: 204,
      ok: true,
      headers: new Headers(),
      text: async () => "",
    });

    const req = new Request("http://localhost/api/plugs/plug1/configure", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ip: "192.168.1.100" }),
    });

    const resp = await routes.POST(req as never, makeParams("plug1"));

    expect(resp.status).toBe(204);
    expect(mockFetch).toHaveBeenCalledWith(
      `${API_URL}/api/plugs/plug1/configure`,
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ ip: "192.168.1.100" }),
      }),
    );
  });

  it("returns 500 on network error", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const req = new Request("http://localhost/api/plugs/plug1/configure", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ip: "192.168.1.100" }),
    });

    const resp = await routes.POST(req as never, makeParams("plug1"));
    expect(resp.status).toBe(500);
  });

  it("returns 400 on invalid JSON body", async () => {
    const req = new Request("http://localhost/api/plugs/plug1/configure", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "not valid json",
    });

    const resp = await routes.POST(req as never, makeParams("plug1"));
    expect(resp.status).toBe(400);
    const body = await resp.json();
    expect(body.detail).toBe("Invalid request body");
  });

  it("passes through non-JSON response", async () => {
    mockFetch.mockResolvedValueOnce({
      status: 500,
      ok: false,
      headers: new Headers({ "Content-Type": "text/plain" }),
      text: async () => "Internal Server Error",
    });

    const req = new Request("http://localhost/api/plugs/plug1/configure", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ip: "192.168.1.100" }),
    });

    const resp = await routes.POST(req as never, makeParams("plug1"));
    const text = await resp.text();
    expect(resp.status).toBe(500);
    expect(text).toBe("Internal Server Error");
  });
});
