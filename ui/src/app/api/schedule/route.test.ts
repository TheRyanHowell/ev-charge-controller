import { describe, it, expect, vi, beforeEach } from "vitest";

const API_URL = process.env.API_URL || "http://localhost:8080";

const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

// Mock problemResponse
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

// Dynamic import to apply mocks
const routes = await import("./route");

describe("schedule route", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("PATCH proxies to backend", async () => {
    mockFetch.mockResolvedValueOnce({
      status: 201,
      ok: true,
      headers: new Headers({ "Content-Type": "application/json" }),
      text: async () =>
        JSON.stringify({ id: "plug", time: "03:00", enabled: true }),
    });

    const req = new Request("http://localhost/api/schedule", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ time: "03:00", enabled: true }),
    });

    const resp = await routes.PATCH(req as never);
    const body = await resp.json();

    expect(resp.status).toBe(201);
    expect(body).toEqual({ id: "plug", time: "03:00", enabled: true });
    expect(mockFetch).toHaveBeenCalledWith(
      `${API_URL}/api/schedule`,
      expect.objectContaining({
        method: "PATCH",
        body: JSON.stringify({ time: "03:00", enabled: true }),
      }),
    );
  });

  it("GET proxies to backend", async () => {
    mockFetch.mockResolvedValueOnce({
      status: 200,
      ok: true,
      json: () => Promise.resolve({ id: "plug", time: "03:00", enabled: true }),
    });

    const resp = await routes.GET();
    const body = await resp.json();

    expect(resp.status).toBe(200);
    expect(body).toEqual({ id: "plug", time: "03:00", enabled: true });
  });

  it("GET returns problem response when backend returns 404", async () => {
    mockFetch.mockResolvedValueOnce({
      status: 404,
      ok: false,
      text: async () => "",
    });

    const resp = await routes.GET();
    const body = await resp.json();

    expect(resp.status).toBe(404);
    expect(body).toEqual({
      type: "about:blank",
      title: "Error",
      status: 404,
      detail: "Failed to fetch schedule",
    });
  });

  it("PATCH returns null when backend returns empty body", async () => {
    mockFetch.mockResolvedValueOnce({
      status: 204,
      ok: true,
      headers: new Headers(),
      text: async () => "",
    });

    const req = new Request("http://localhost/api/schedule", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ time: "03:00", enabled: true }),
    });

    const resp = await routes.PATCH(req as never);
    expect(resp.status).toBe(204);
  });

  it("PATCH passes through non-JSON response", async () => {
    mockFetch.mockResolvedValueOnce({
      status: 500,
      ok: false,
      headers: new Headers({ "Content-Type": "text/plain" }),
      text: async () => "Internal Server Error",
    });

    const req = new Request("http://localhost/api/schedule", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ time: "03:00", enabled: true }),
    });

    const resp = await routes.PATCH(req as never);
    const text = await resp.text();
    expect(resp.status).toBe(500);
    expect(text).toBe("Internal Server Error");
  });

  it("PATCH returns RFC 7807 on network error", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const req = new Request("http://localhost/api/schedule", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ time: "03:00", enabled: true }),
    });

    const resp = await routes.PATCH(req as never);
    expect(resp.status).toBe(500);
    expect(resp.headers.get("content-type")).toContain(
      "application/problem+json",
    );
  });

  it("GET returns RFC 7807 on network error", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const resp = await routes.GET();
    expect(resp.status).toBe(500);
    expect(resp.headers.get("content-type")).toContain(
      "application/problem+json",
    );
  });

  it("PATCH returns 400 on invalid JSON body", async () => {
    const req = new Request("http://localhost/api/schedule", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: "not valid json",
    });

    const resp = await routes.PATCH(req as never);
    expect(resp.status).toBe(400);
    expect(resp.headers.get("content-type")).toContain(
      "application/problem+json",
    );
    const body = await resp.json();
    expect(body.detail).toBe("Invalid request body");
  });
});
