import { describe, it, expect, vi, beforeEach } from "vitest";

// ---- types (erased at runtime - no hoisting issue) ----
type CookieOpts = Record<string, unknown>;
type CookieStore = {
  _store: Record<string, { value: string } & CookieOpts>;
  _deleted: Set<string>;
  get(name: string): { value: string } | undefined;
  getAll(): Array<{ name: string; value: string }>;
  set(name: string, value: string, opts?: CookieOpts): void;
  delete(name: string): void;
};

// ---- mock classes via vi.hoisted so they're available inside vi.mock factories ----
const { MockNextRequest, MockNextResponse } = vi.hoisted(() => {
  function makeCookieStore(): CookieStore {
    const _store: Record<string, { value: string } & CookieOpts> = {};
    const _deleted = new Set<string>();
    return {
      _store,
      _deleted,
      get: (name) => _store[name],
      getAll: () =>
        Object.entries(_store).map(([n, v]) => ({ name: n, value: v.value })),
      set: (name, value, opts = {}) => {
        _store[name] = { value, ...opts };
        _deleted.delete(name);
      },
      delete: (name) => {
        Reflect.deleteProperty(_store, name);
        _deleted.add(name);
      },
    };
  }

  class MockNextResponse {
    status: number;
    headers: Headers;
    cookies: CookieStore;
    _body: unknown;

    constructor(body: unknown, init: ResponseInit = {}) {
      this.status = init.status ?? 200;
      this.headers = new Headers(init.headers as HeadersInit);
      this.cookies = makeCookieStore();
      this._body = body;
    }

    async json(): Promise<unknown> {
      return typeof this._body === "string"
        ? (JSON.parse(this._body) as unknown)
        : this._body;
    }

    static next(init?: { request?: { headers?: Headers } }): MockNextResponse {
      const res = new MockNextResponse(null, { status: 200 });
      (res as unknown as { _requestInit: unknown })._requestInit = init;
      return res;
    }

    static redirect(url: URL | string, init?: ResponseInit): MockNextResponse {
      const location = url instanceof URL ? url.toString() : url;
      const res = new MockNextResponse(null, { status: init?.status ?? 307 });
      res.headers.set("Location", location);
      return res;
    }

    static json(data: unknown, init?: ResponseInit): MockNextResponse {
      const res = new MockNextResponse(JSON.stringify(data), {
        status: init?.status ?? 200,
      });
      const ct =
        (init?.headers as Record<string, string> | undefined)?.[
          "Content-Type"
        ] ?? "application/json";
      res.headers.set("Content-Type", ct);
      return res;
    }
  }

  class MockNextRequest {
    url: string;
    nextUrl: URL;
    cookies: CookieStore;
    headers: Headers;

    constructor(url: string, init: { cookies?: Record<string, string> } = {}) {
      this.url = url;
      this.nextUrl = new URL(url);
      this.cookies = makeCookieStore();
      for (const [name, value] of Object.entries(init.cookies ?? {})) {
        this.cookies.set(name, value);
      }
      this.headers = new Headers();
      if (init.cookies) {
        const cookieStr = Object.entries(init.cookies)
          .map(([k, v]) => `${k}=${v}`)
          .join("; ");
        this.headers.set("cookie", cookieStr);
      }
    }
  }

  return { MockNextRequest, MockNextResponse };
});

vi.mock("next/server", () => ({
  NextRequest: MockNextRequest,
  NextResponse: MockNextResponse,
}));

vi.mock("@/lib/jwt", () => ({
  isTokenExpiringSoon: vi.fn(),
}));

vi.mock("@/lib/auth-refresh", () => ({
  refreshTokens: vi.fn(),
  accessCookieMaxAge: vi.fn(),
  isCookieSecure: () => false,
  REFRESH_TOKEN_MAX_AGE: 2592000,
}));

import { refreshTokens, accessCookieMaxAge } from "@/lib/auth-refresh";
import { isTokenExpiringSoon } from "@/lib/jwt";

import { proxy as middleware } from "./proxy";

const mockIsExpiring = vi.mocked(isTokenExpiringSoon);
const mockRefreshTokens = vi.mocked(refreshTokens);
const mockAccessCookieMaxAge = vi.mocked(accessCookieMaxAge);

function makeRequest(path: string, cookieValues: Record<string, string> = {}) {
  return new MockNextRequest(`http://localhost${path}`, {
    cookies: cookieValues,
  });
}

describe("middleware", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsExpiring.mockReturnValue(false);
    mockAccessCookieMaxAge.mockReturnValue(120);
  });

  describe("skipped paths", () => {
    it("passes through /login without auth check", async () => {
      const req = makeRequest("/login");
      const res = await middleware(req as never);
      expect(res.status).toBe(200);
      expect(mockRefreshTokens).not.toHaveBeenCalled();
    });

    it("passes through /api/auth/login", async () => {
      const req = makeRequest("/api/auth/login");
      const res = await middleware(req as never);
      expect(res.status).toBe(200);
      expect(mockRefreshTokens).not.toHaveBeenCalled();
    });

    it("passes through /api/auth/register", async () => {
      const req = makeRequest("/api/auth/register");
      const res = await middleware(req as never);
      expect(res.status).toBe(200);
    });

    it("passes through /api/auth/refresh", async () => {
      const req = makeRequest("/api/auth/refresh");
      const res = await middleware(req as never);
      expect(res.status).toBe(200);
    });

    it("passes through /api/auth/logout", async () => {
      const req = makeRequest("/api/auth/logout");
      const res = await middleware(req as never);
      expect(res.status).toBe(200);
    });

    it("passes through /api/health without auth check", async () => {
      const req = makeRequest("/api/health");
      const res = await middleware(req as never);
      expect(res.status).toBe(200);
      expect(mockRefreshTokens).not.toHaveBeenCalled();
    });
  });

  describe("valid access token", () => {
    it("passes through without refresh when token is valid", async () => {
      mockIsExpiring.mockReturnValue(false);
      const req = makeRequest("/dashboard", { access_token: "valid-token" });
      const res = await middleware(req as never);
      expect(res.status).toBe(200);
      expect(mockRefreshTokens).not.toHaveBeenCalled();
    });

    it("passes through API route without refresh when token is valid", async () => {
      mockIsExpiring.mockReturnValue(false);
      const req = makeRequest("/api/charge-sessions", {
        access_token: "valid-token",
      });
      const res = await middleware(req as never);
      expect(res.status).toBe(200);
      expect(mockRefreshTokens).not.toHaveBeenCalled();
    });
  });

  describe("expired access token + valid refresh token", () => {
    it("refreshes and sets new cookies on page route", async () => {
      mockIsExpiring.mockReturnValue(true);
      mockRefreshTokens.mockResolvedValue({
        accessToken: "new-access",
        refreshToken: "new-refresh",
        expiresAt: "2026-06-20T14:00:00Z",
      });

      const req = makeRequest("/dashboard", {
        access_token: "expired-token",
        refresh_token: "valid-refresh",
      });
      const res = await middleware(req as never);
      const cookies = res.cookies as unknown as CookieStore;

      expect(mockRefreshTokens).toHaveBeenCalledWith("valid-refresh");
      expect(res.status).toBe(200);
      expect(cookies.get("access_token")?.value).toBe("new-access");
      expect(cookies.get("refresh_token")?.value).toBe("new-refresh");
    });

    it("refreshes when access token is missing but refresh token exists", async () => {
      mockRefreshTokens.mockResolvedValue({
        accessToken: "new-access",
        refreshToken: "new-refresh",
        expiresAt: "2026-06-20T14:00:00Z",
      });

      const req = makeRequest("/dashboard", { refresh_token: "valid-refresh" });
      const res = await middleware(req as never);

      expect(mockRefreshTokens).toHaveBeenCalledWith("valid-refresh");
      expect(res.status).toBe(200);
    });

    it("sets access token maxAge from accessCookieMaxAge", async () => {
      mockIsExpiring.mockReturnValue(true);
      mockAccessCookieMaxAge.mockReturnValue(240);
      mockRefreshTokens.mockResolvedValue({
        accessToken: "new-access",
        refreshToken: "new-refresh",
        expiresAt: "2026-06-20T14:04:00Z",
      });

      const req = makeRequest("/dashboard", {
        access_token: "expired",
        refresh_token: "valid-refresh",
      });
      const res = await middleware(req as never);
      const cookies = res.cookies as unknown as CookieStore;

      expect(mockAccessCookieMaxAge).toHaveBeenCalledWith(
        "2026-06-20T14:04:00Z",
      );
      expect(cookies.get("access_token")).toMatchObject({
        value: "new-access",
        maxAge: 240,
        httpOnly: true,
        sameSite: "lax",
        path: "/",
      });
      expect(cookies.get("refresh_token")).toMatchObject({
        value: "new-refresh",
        maxAge: 2592000,
        httpOnly: true,
      });
    });
  });

  describe("expired access token + refresh fails - page route", () => {
    it("redirects to /login?reason=session-expired", async () => {
      mockIsExpiring.mockReturnValue(true);
      mockRefreshTokens.mockResolvedValue(null);

      const req = makeRequest("/dashboard", {
        access_token: "expired",
        refresh_token: "bad-refresh",
      });
      const res = await middleware(req as never);

      expect(res.status).toBe(307);
      expect(res.headers.get("Location")).toContain(
        "/login?reason=session-expired",
      );
    });

    it("clears only access_token on failed refresh (preserves refresh_token for retry)", async () => {
      mockIsExpiring.mockReturnValue(true);
      mockRefreshTokens.mockResolvedValue(null);

      const req = makeRequest("/dashboard", {
        access_token: "expired",
        refresh_token: "bad-refresh",
      });
      const res = await middleware(req as never);
      const cookies = res.cookies as unknown as CookieStore;

      expect(cookies._deleted.has("access_token")).toBe(true);
      expect(cookies._deleted.has("refresh_token")).toBe(false);
    });
  });

  describe("expired access token + refresh fails - API route", () => {
    it("returns 401 problem+json", async () => {
      mockIsExpiring.mockReturnValue(true);
      mockRefreshTokens.mockResolvedValue(null);

      const req = makeRequest("/api/charge-sessions", {
        access_token: "expired",
        refresh_token: "bad-refresh",
      });
      const res = await middleware(req as never);

      expect(res.status).toBe(401);
      expect(res.headers.get("Content-Type")).toContain(
        "application/problem+json",
      );
    });

    it("clears only access_token on failed refresh (API route)", async () => {
      mockIsExpiring.mockReturnValue(true);
      mockRefreshTokens.mockResolvedValue(null);

      const req = makeRequest("/api/vehicles", {
        access_token: "expired",
        refresh_token: "bad-refresh",
      });
      const res = await middleware(req as never);
      const cookies = res.cookies as unknown as CookieStore;

      expect(cookies._deleted.has("access_token")).toBe(true);
      expect(cookies._deleted.has("refresh_token")).toBe(false);
    });
  });

  describe("no tokens", () => {
    it("redirects page route to /login", async () => {
      const req = makeRequest("/dashboard");
      const res = await middleware(req as never);

      expect(res.status).toBe(307);
      expect(res.headers.get("Location")).toMatch(/\/login$/);
    });

    it("returns 401 for API route", async () => {
      const req = makeRequest("/api/vehicles");
      const res = await middleware(req as never);

      expect(res.status).toBe(401);
    });

    it("does not call refreshTokens when no refresh token", async () => {
      const req = makeRequest("/dashboard");
      await middleware(req as never);
      expect(mockRefreshTokens).not.toHaveBeenCalled();
    });
  });
});
