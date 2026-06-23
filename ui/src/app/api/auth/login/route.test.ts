import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

vi.mock("next/server", () => {
  interface CookieStore {
    cookies: Record<string, Record<string, unknown>>;
    set(name: string, value: string, opts: Record<string, unknown>): void;
  }

  function createCookieStore(): CookieStore {
    return {
      cookies: {} as Record<string, Record<string, unknown>>,
      set(name: string, value: string, opts: Record<string, unknown>): void {
        this.cookies[name] = { value, ...opts };
      },
    };
  }

  const NextResponseFn = function NextResponse(
    body: unknown,
    init: ResponseInit = {},
  ) {
    const cookieStore = createCookieStore();
    const resp = new Response(body === null ? null : (body as BodyInit), init);
    return Object.assign(resp, { cookies: cookieStore });
  };

  return {
    NextResponse: Object.assign(NextResponseFn, {
      json: (body: unknown, init: ResponseInit = {}) => {
        const cookieStore = createCookieStore();
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

function makeRequest(body: Record<string, unknown>) {
  return {
    json: () => Promise.resolve(body),
  } as unknown as Parameters<typeof POST>[0];
}

function makeBadJsonRequest() {
  return {
    json: () => Promise.reject(new Error("invalid json")),
  } as unknown as Parameters<typeof POST>[0];
}

describe("POST /api/auth/login", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T12:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns tokens and sets cookies on successful login", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () =>
        Promise.resolve({
          accessToken: "test-access-token",
          expiresAt: "2026-06-20T12:02:00Z",
          user: { email: "test@example.com" },
        }),
      headers: {
        get: (key: string) =>
          key === "set-cookie"
            ? "refresh_token=test-refresh-token; Path=/; HttpOnly"
            : "",
      },
    });

    const req = makeRequest({ email: "test@example.com", password: "pass123" });
    const response = await POST(req);
    const data = await response.json();

    expect(response.status).toBe(200);
    expect(data.accessToken).toBe("test-access-token");
    expect(data.user.email).toBe("test@example.com");
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/auth/login"),
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          email: "test@example.com",
          password: "pass123",
        }),
      }),
    );
  });

  it("returns 400 on invalid JSON body", async () => {
    const response = await POST(makeBadJsonRequest());
    expect(response.status).toBe(400);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
    const data = await response.json();
    expect(data.detail).toBe("invalid JSON");
  });

  it("returns 500 on network error", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

    const response = await POST(
      makeRequest({ email: "test@example.com", password: "pass123" }),
    );
    expect(response.status).toBe(500);
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
          detail: "invalid credentials",
        }),
      headers: { get: () => "" },
    });

    const response = await POST(
      makeRequest({ email: "test@example.com", password: "wrong" }),
    );
    expect(response.status).toBe(401);
    const data = await response.json();
    expect(data.detail).toBe("invalid credentials");
  });

  it("sets cookies with correct properties", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () =>
        Promise.resolve({
          accessToken: "tok",
          expiresAt: "2026-06-20T12:02:00Z",
        }),
      headers: {
        get: (key: string) =>
          key === "set-cookie" ? "refresh_token=rftok; Path=/; HttpOnly" : "",
      },
    });

    const req = makeRequest({ email: "test@example.com", password: "pass" });
    const response = await POST(req);
    const cookies = (response as any).cookies;

    expect(cookies.cookies.access_token).toBeDefined();
    expect(cookies.cookies.access_token.value).toBe("tok");
    expect(cookies.cookies.access_token.httpOnly).toBe(true);
    expect(cookies.cookies.access_token.sameSite).toBe("lax");
    // maxAge derived from expiresAt (2026-06-20T12:02:00Z − 2026-06-20T12:00:00Z = 120s)
    expect(cookies.cookies.access_token.maxAge).toBe(120);
    expect(cookies.cookies.refresh_token).toBeDefined();
    expect(cookies.cookies.refresh_token.value).toBe("rftok");
    expect(cookies.cookies.refresh_token.httpOnly).toBe(true);
  });

  it("does not set cookies when tokens are missing", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({}),
      headers: { get: () => "" },
    });

    const req = makeRequest({ email: "test@example.com", password: "pass" });
    const response = await POST(req);
    const cookies = (response as any).cookies;

    expect(cookies.cookies.access_token).toBeUndefined();
    expect(cookies.cookies.refresh_token).toBeUndefined();
  });
});
