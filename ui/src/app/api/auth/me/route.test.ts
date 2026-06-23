import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("next/server", () => ({
  NextResponse: {
    json: (body: unknown, init: ResponseInit = {}) =>
      new Response(JSON.stringify(body), {
        status: init.status ?? 200,
        headers: init.headers,
      }),
  },
}));

import { GET } from "./route";

function makeRequestWithCookie(accessToken?: string) {
  return {
    cookies: {
      get: (name: string) =>
        name === "access_token" ? { value: accessToken } : null,
    },
  } as unknown as Parameters<typeof GET>[0];
}

describe("GET /api/auth/me", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns 401 when access token cookie is missing", async () => {
    const req = makeRequestWithCookie(undefined);
    const response = await GET(req);

    expect(response.status).toBe(401);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
    const data = await response.json();
    expect(data.detail).toBe("not authenticated");
  });

  it("returns user data on success", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () =>
        Promise.resolve({
          id: "user1",
          email: "test@example.com",
        }),
    });

    const req = makeRequestWithCookie("valid-token");
    const response = await GET(req);
    const data = await response.json();

    expect(response.status).toBe(200);
    expect(data.email).toBe("test@example.com");
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/auth/me"),
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer valid-token",
        }),
      }),
    );
  });

  it("returns 500 on network error", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

    const response = await GET(makeRequestWithCookie("valid-token"));
    expect(response.status).toBe(500);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
  });

  it("passes through backend error responses", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      json: () =>
        Promise.resolve({
          type: "about:blank",
          title: "Not Found",
          status: 404,
          detail: "user not found",
        }),
    });

    const response = await GET(makeRequestWithCookie("valid-token"));
    expect(response.status).toBe(404);
    const data = await response.json();
    expect(data.detail).toBe("user not found");
  });
});
