import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("next/server", () => {
  const cookieStore: {
    cookies: Record<string, Record<string, unknown>>;
    set(name: string, value: string, opts: Record<string, unknown>): void;
  } = {
    cookies: {} as Record<string, Record<string, unknown>>,
    set(name: string, value: string, opts: Record<string, unknown>) {
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

describe("POST /api/auth/logout", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("clears cookies and returns 204", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
    });

    const req = makeRequestWithCookie("some-refresh-token");
    const response = await POST(req);

    expect(response.status).toBe(204);
    const cookies = (response as any).cookies;
    expect(cookies.cookies.access_token).toBeDefined();
    expect(cookies.cookies.access_token.maxAge).toBe(-1);
    expect(cookies.cookies.refresh_token).toBeDefined();
    expect(cookies.cookies.refresh_token.maxAge).toBe(-1);
  });

  it("sends refresh token to backend for revocation", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
    });

    const req = makeRequestWithCookie("token-to-revoke");
    await POST(req);

    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/auth/logout"),
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ refreshToken: "token-to-revoke" }),
      }),
    );
  });

  it("clears cookies even when backend is unreachable (best effort)", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

    const req = makeRequestWithCookie("some-token");
    const response = await POST(req);

    expect(response.status).toBe(204);
    const cookies = (response as any).cookies;
    expect(cookies.cookies.access_token.maxAge).toBe(-1);
    expect(cookies.cookies.refresh_token.maxAge).toBe(-1);
  });

  it("works without refresh token cookie", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
    });

    const req = makeRequestWithCookie(undefined);
    const response = await POST(req);

    expect(response.status).toBe(204);
    expect(global.fetch).not.toHaveBeenCalled();
    const cookies = (response as any).cookies;
    expect(cookies.cookies.access_token.maxAge).toBe(-1);
    expect(cookies.cookies.refresh_token.maxAge).toBe(-1);
  });

  it("sets secure flag based on NODE_ENV", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
    });

    vi.stubEnv("NODE_ENV", "production");

    const req = makeRequestWithCookie("token");
    const response = await POST(req);
    const cookies = (response as any).cookies;

    expect(cookies.cookies.access_token.secure).toBe(true);
    expect(cookies.cookies.refresh_token.secure).toBe(true);

    vi.unstubAllEnvs();
  });
});
