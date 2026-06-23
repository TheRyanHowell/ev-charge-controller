import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

vi.mock("next/server", () => {
  const cookieStore: {
    cookies: Record<string, Record<string, unknown>>;
    set(name: string, value: string, opts: Record<string, unknown>): void;
  } = {
    cookies: {} as Record<string, Record<string, unknown>>,
    set(name: string, value: string, opts: Record<string, unknown>): void {
      this.cookies[name] = { value, ...opts };
    },
  };

  const NextResponseFn = function NextResponse(
    body: unknown,
    init: ResponseInit = {},
  ) {
    const resp = new Response(body === null ? null : (body as BodyInit), init);
    return Object.assign(resp, { cookies: cookieStore });
  };

  return {
    NextResponse: Object.assign(NextResponseFn, {
      json: (body: unknown, init: ResponseInit = {}) => {
        const resp = new Response(JSON.stringify(body), {
          status: init.status ?? 200,
          headers: init.headers,
        });
        return Object.assign(resp, { cookies: cookieStore });
      },
    }),
  };
});

import { POST } from "./route";

function makeRequestWithCookie(refreshToken?: string) {
  return {
    cookies: {
      get: (name: string) =>
        name === "refresh_token" ? { value: refreshToken } : null,
    },
  } as unknown as Parameters<typeof POST>[0];
}

describe("POST /api/auth/refresh", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T12:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns 401 when refresh token cookie is missing", async () => {
    const req = makeRequestWithCookie(undefined);
    const response = await POST(req);

    expect(response.status).toBe(401);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
    const data = await response.json();
    expect(data.detail).toBe("refresh token required");
  });

  it("returns new tokens and sets cookies on success", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () =>
        Promise.resolve({
          accessToken: "new-access-token",
          expiresAt: "2026-06-20T12:02:00Z",
        }),
      headers: {
        get: (key: string) =>
          key === "set-cookie"
            ? "refresh_token=new-refresh-token; Path=/; HttpOnly"
            : "",
      },
    });

    const req = makeRequestWithCookie("old-refresh-token");
    const response = await POST(req);
    const data = await response.json();

    expect(response.status).toBe(200);
    expect(data.accessToken).toBe("new-access-token");
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/auth/refresh"),
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ refreshToken: "old-refresh-token" }),
      }),
    );
  });

  it("sets cookies with correct properties", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () =>
        Promise.resolve({
          accessToken: "new-access",
          expiresAt: "2026-06-20T12:02:00Z",
        }),
      headers: {
        get: (key: string) =>
          key === "set-cookie"
            ? "refresh_token=new-refresh; Path=/; HttpOnly"
            : "",
      },
    });

    const req = makeRequestWithCookie("old-refresh");
    const response = await POST(req);
    const cookies = (response as any).cookies;

    expect(cookies.cookies.access_token).toBeDefined();
    expect(cookies.cookies.access_token.value).toBe("new-access");
    expect(cookies.cookies.access_token.httpOnly).toBe(true);
    expect(cookies.cookies.access_token.sameSite).toBe("strict");
    // maxAge derived from expiresAt (2026-06-20T12:02:00Z − 2026-06-20T12:00:00Z = 120s)
    expect(cookies.cookies.access_token.maxAge).toBe(120);
    expect(cookies.cookies.refresh_token).toBeDefined();
    expect(cookies.cookies.refresh_token.value).toBe("new-refresh");
  });

  it("returns 500 on network error", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

    const response = await POST(makeRequestWithCookie("some-token"));
    expect(response.status).toBe(500);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
  });

  it("passes through backend error responses", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      json: () =>
        Promise.resolve({
          type: "about:blank",
          title: "Unauthorized",
          status: 401,
          detail: "token expired",
        }),
    });

    const response = await POST(makeRequestWithCookie("expired-token"));
    expect(response.status).toBe(401);
    const data = await response.json();
    expect(data.detail).toBe("token expired");
  });
});
