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

describe("plugs/[plugId]/schedule API route", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("GET", () => {
    it("returns schedule from backend", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({
            plugId: "plug1",
            time: "06:00",
            enabled: true,
          }),
      });

      const resp = await routes.GET(
        new Request("http://localhost/api/plugs/plug1/schedule") as never,
        makeParams("plug1"),
      );
      const body = await resp.json();

      expect(resp.status).toBe(200);
      expect(body.plugId).toBe("plug1");
      expect(body.enabled).toBe(true);
      expect(mockFetch).toHaveBeenCalledWith(
        `${API_URL}/api/plugs/plug1/schedule`,
        expect.any(Object),
      );
    });

    it("returns 204 for empty response", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
      });

      const resp = await routes.GET(
        new Request("http://localhost/api/plugs/plug1/schedule") as never,
        makeParams("plug1"),
      );
      expect(resp.status).toBe(204);
    });

    it("returns RFC 7807 on network error", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      const resp = await routes.GET(
        new Request("http://localhost/api/plugs/plug1/schedule") as never,
        makeParams("plug1"),
      );
      expect(resp.status).toBe(500);
    });

    it("returns RFC 7807 when backend returns error", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
      });

      const resp = await routes.GET(
        new Request("http://localhost/api/plugs/plug1/schedule") as never,
        makeParams("plug1"),
      );
      expect(resp.status).toBe(404);
      const body = await resp.json();
      expect(body.detail).toBe("Failed to fetch schedule");
    });
  });

  describe("PATCH", () => {
    it("updates schedule and returns 204", async () => {
      mockFetch.mockResolvedValueOnce({
        status: 204,
        ok: true,
        headers: new Headers(),
        text: async () => "",
      });

      const req = new Request("http://localhost/api/plugs/plug1/schedule", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ time: "06:00", enabled: true }),
      });

      const resp = await routes.PATCH(req as never, makeParams("plug1"));

      expect(resp.status).toBe(204);
      expect(mockFetch).toHaveBeenCalledWith(
        `${API_URL}/api/plugs/plug1/schedule`,
        expect.objectContaining({
          method: "PATCH",
          body: JSON.stringify({ time: "06:00", enabled: true }),
        }),
      );
    });

    it("returns 500 on network error", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      const req = new Request("http://localhost/api/plugs/plug1/schedule", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ time: "06:00", enabled: true }),
      });

      const resp = await routes.PATCH(req as never, makeParams("plug1"));
      expect(resp.status).toBe(500);
    });

    it("returns 400 on invalid JSON body", async () => {
      const req = new Request("http://localhost/api/plugs/plug1/schedule", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: "not valid json",
      });

      const resp = await routes.PATCH(req as never, makeParams("plug1"));
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

      const req = new Request("http://localhost/api/plugs/plug1/schedule", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ time: "06:00", enabled: true }),
      });

      const resp = await routes.PATCH(req as never, makeParams("plug1"));
      const text = await resp.text();
      expect(resp.status).toBe(500);
      expect(text).toBe("Internal Server Error");
    });
  });
});
