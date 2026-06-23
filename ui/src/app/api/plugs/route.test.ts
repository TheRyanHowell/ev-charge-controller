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

describe("plugs API route", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("GET", () => {
    it("returns plugs from backend", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve([{ id: "plug1", name: "Garage Plug", online: true }]),
      });

      const resp = await routes.GET();
      const body = await resp.json();

      expect(resp.status).toBe(200);
      expect(body).toEqual([
        { id: "plug1", name: "Garage Plug", online: true },
      ]);
    });

    it("returns 204 for empty response", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
      });

      const resp = await routes.GET();
      expect(resp.status).toBe(204);
    });

    it("returns RFC 7807 on network error", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      const resp = await routes.GET();
      expect(resp.status).toBe(500);
      expect(resp.headers.get("content-type")).toContain(
        "application/problem+json",
      );
    });

    it("returns RFC 7807 when backend returns error", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 502,
      });

      const resp = await routes.GET();
      expect(resp.status).toBe(502);
      const body = await resp.json();
      expect(body.detail).toBe("Failed to fetch plugs");
    });
  });

  describe("POST", () => {
    it("creates a plug and returns response", async () => {
      mockFetch.mockResolvedValueOnce({
        status: 201,
        ok: true,
        headers: new Headers({ "Content-Type": "application/json" }),
        text: async () =>
          JSON.stringify({
            plug: { id: "plug1", name: "Garage" },
            consoleCommands: "some commands",
          }),
      });

      const req = new Request("http://localhost/api/plugs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "Garage" }),
      });

      const resp = await routes.POST(req as never);
      const body = await resp.json();

      expect(resp.status).toBe(201);
      expect(body.plug.name).toBe("Garage");
      expect(mockFetch).toHaveBeenCalledWith(
        `${API_URL}/api/plugs`,
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ name: "Garage" }),
        }),
      );
    });

    it("returns 500 on network error", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      const req = new Request("http://localhost/api/plugs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "Garage" }),
      });

      const resp = await routes.POST(req as never);
      expect(resp.status).toBe(500);
    });

    it("returns 400 on invalid JSON body", async () => {
      const req = new Request("http://localhost/api/plugs", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: "not valid json",
      });

      const resp = await routes.POST(req as never);
      expect(resp.status).toBe(400);
      expect(resp.headers.get("content-type")).toContain(
        "application/problem+json",
      );
      const body = await resp.json();
      expect(body.detail).toBe("Invalid request body");
    });
  });
});
