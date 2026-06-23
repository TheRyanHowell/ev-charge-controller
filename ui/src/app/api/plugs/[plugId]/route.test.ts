import type { NextRequest } from "next/server";

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

describe("plugs/[plugId] API route", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("GET", () => {
    it("returns plug from backend", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () =>
          Promise.resolve({ id: "plug1", name: "Garage Plug", online: true }),
      });

      const resp = await routes.GET(
        new Request("http://localhost/api/plugs/plug1") as never,
        makeParams("plug1"),
      );
      const body = await resp.json();

      expect(resp.status).toBe(200);
      expect(body.id).toBe("plug1");
      expect(mockFetch).toHaveBeenCalledWith(
        `${API_URL}/api/plugs/plug1`,
        expect.any(Object),
      );
    });

    it("returns RFC 7807 on network error", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      const resp = await routes.GET(
        new Request("http://localhost/api/plugs/plug1") as never,
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
        new Request("http://localhost/api/plugs/plug1") as never,
        makeParams("plug1"),
      );
      expect(resp.status).toBe(404);
      const body = await resp.json();
      expect(body.detail).toBe("Failed to fetch plug");
    });
  });

  describe("PATCH", () => {
    it("updates plug and returns response", async () => {
      mockFetch.mockResolvedValueOnce({
        status: 200,
        ok: true,
        headers: new Headers({ "Content-Type": "application/json" }),
        text: async () => JSON.stringify({ id: "plug1", name: "Updated Plug" }),
      });

      const req = new Request("http://localhost/api/plugs/plug1", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "Updated Plug" }),
      });

      const resp = await routes.PATCH(req as never, makeParams("plug1"));
      const body = await resp.json();

      expect(resp.status).toBe(200);
      expect(body.name).toBe("Updated Plug");
      expect(mockFetch).toHaveBeenCalledWith(
        `${API_URL}/api/plugs/plug1`,
        expect.objectContaining({
          method: "PATCH",
          body: JSON.stringify({ name: "Updated Plug" }),
        }),
      );
    });

    it("returns 204 on no-content response", async () => {
      mockFetch.mockResolvedValueOnce({
        status: 204,
        ok: true,
        headers: new Headers(),
        text: async () => "",
      });

      const req = new Request("http://localhost/api/plugs/plug1", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "Updated Plug" }),
      });

      const resp = await routes.PATCH(req as never, makeParams("plug1"));
      expect(resp.status).toBe(204);
    });

    it("returns 500 on network error", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      const req = new Request("http://localhost/api/plugs/plug1", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "Updated Plug" }),
      });

      const resp = await routes.PATCH(req as never, makeParams("plug1"));
      expect(resp.status).toBe(500);
    });

    it("returns 400 on invalid JSON body", async () => {
      const req = new Request("http://localhost/api/plugs/plug1", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: "not valid json",
      });

      const resp = await routes.PATCH(req as never, makeParams("plug1"));
      expect(resp.status).toBe(400);
      const body = await resp.json();
      expect(body.detail).toBe("Invalid request body");
    });
  });

  describe("DELETE", () => {
    it("deletes plug and returns 204", async () => {
      mockFetch.mockResolvedValueOnce({
        status: 204,
        ok: true,
        headers: new Headers(),
        text: async () => "",
      });

      const resp = await routes.DELETE(
        new Request("http://localhost/api/plugs/plug1", {
          method: "DELETE",
        }) as unknown as NextRequest,
        makeParams("plug1"),
      );

      expect(resp.status).toBe(204);
      expect(mockFetch).toHaveBeenCalledWith(
        `${API_URL}/api/plugs/plug1`,
        expect.objectContaining({ method: "DELETE" }),
      );
    });

    it("returns 500 on network error", async () => {
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      const resp = await routes.DELETE(
        new Request("http://localhost/api/plugs/plug1", {
          method: "DELETE",
        }) as unknown as NextRequest,
        makeParams("plug1"),
      );
      expect(resp.status).toBe(500);
    });
  });
});
