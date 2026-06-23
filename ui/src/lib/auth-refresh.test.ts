import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import {
  extractRefreshTokenFromSetCookie,
  refreshAccessTokenPair,
  refreshTokens,
  accessCookieMaxAge,
  ACCESS_TOKEN_MAX_AGE,
  REFRESH_TOKEN_MAX_AGE,
} from "./auth-refresh";

const cookiesMock = vi.hoisted(() => ({
  cookies: vi.fn(),
}));

vi.mock("next/headers", () => cookiesMock);

const originalFetch = global.fetch;

describe("auth-refresh", () => {
  describe("extractRefreshTokenFromSetCookie", () => {
    it("extracts token from single cookie header", () => {
      const header =
        "refresh_token=abc123; Path=/; HttpOnly; Secure; Max-Age=2592000";
      const result = extractRefreshTokenFromSetCookie(header);
      expect(result).toBe("abc123");
    });

    it("extracts token from comma-separated headers", () => {
      const header =
        "session=xyz; Path=/, refresh_token=def456; Path=/; HttpOnly, other=val";
      const result = extractRefreshTokenFromSetCookie(header);
      expect(result).toBe("def456");
    });

    it("extracts token with multiple cookies in header", () => {
      const header =
        "__host-session=abc; Path=/; SameSite=Lax, refresh_token=multi123; Path=/; HttpOnly; Secure, __secure-session=def; Path=/; Secure; SameSite=None";
      const result = extractRefreshTokenFromSetCookie(header);
      expect(result).toBe("multi123");
    });

    it("extracts token when refresh_token is first in header", () => {
      const header =
        "refresh_token=first123; Path=/, session=xyz; Path=/; HttpOnly";
      const result = extractRefreshTokenFromSetCookie(header);
      expect(result).toBe("first123");
    });

    it("returns empty string when token not found", () => {
      const header = "session=xyz; Path=/; HttpOnly";
      const result = extractRefreshTokenFromSetCookie(header);
      expect(result).toBe("");
    });

    it("returns empty string for empty input", () => {
      const result = extractRefreshTokenFromSetCookie("");
      expect(result).toBe("");
    });

    it("handles token with special characters", () => {
      const header =
        "refresh_token=eyJhbGciOiJIUzI1NiJ9.abc-def_ghi; Path=/; HttpOnly";
      const result = extractRefreshTokenFromSetCookie(header);
      expect(result).toBe("eyJhbGciOiJIUzI1NiJ9.abc-def_ghi");
    });

    it("returns empty string for malformed Set-Cookie header", () => {
      expect(extractRefreshTokenFromSetCookie("not-a-valid-cookie")).toBe("");
      expect(extractRefreshTokenFromSetCookie("refresh_token")).toBe("");
      expect(
        extractRefreshTokenFromSetCookie(
          "refresh_token = spaced_value; Path=/",
        ),
      ).toBe("");
    });

    it("returns empty string when cookie has no value", () => {
      expect(extractRefreshTokenFromSetCookie("refresh_token=; Path=/")).toBe(
        "",
      );
      expect(
        extractRefreshTokenFromSetCookie(
          "session=abc, refresh_token=; Path=/; HttpOnly",
        ),
      ).toBe("");
    });
  });

  describe("refreshAccessTokenPair", () => {
    beforeEach(() => {
      vi.restoreAllMocks();
    });

    afterEach(() => {
      vi.resetModules();
      global.fetch = originalFetch;
    });

    it("returns null when no refresh token in cookies", async () => {
      cookiesMock.cookies.mockResolvedValue({
        get: vi.fn().mockReturnValue(undefined),
      });

      const result = await refreshAccessTokenPair();
      expect(result).toBeNull();
    });

    it("returns new tokens on successful refresh", async () => {
      cookiesMock.cookies.mockResolvedValue({
        get: vi
          .fn()
          .mockReturnValue({ name: "refresh_token", value: "old-refresh" }),
      });

      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ accessToken: "new-access-token" }),
        headers: {
          get: vi
            .fn()
            .mockReturnValue("refresh_token=new-refresh; Path=/; HttpOnly"),
        },
      });

      const result = await refreshAccessTokenPair();
      expect(result).toEqual({
        accessToken: "new-access-token",
        refreshToken: "new-refresh",
      });
    });

    it("deduplicates concurrent refresh calls", async () => {
      cookiesMock.cookies.mockResolvedValue({
        get: vi
          .fn()
          .mockReturnValue({ name: "refresh_token", value: "old-refresh" }),
      });

      global.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ accessToken: "new-access-token" }),
        headers: {
          get: vi
            .fn()
            .mockReturnValue("refresh_token=new-refresh; Path=/; HttpOnly"),
        },
      });

      const [result1, result2] = await Promise.all([
        refreshAccessTokenPair(),
        refreshAccessTokenPair(),
      ]);

      expect(result1).toEqual({
        accessToken: "new-access-token",
        refreshToken: "new-refresh",
      });
      expect(result2).toEqual({
        accessToken: "new-access-token",
        refreshToken: "new-refresh",
      });
      expect(global.fetch).toHaveBeenCalledTimes(1);
    });
  });

  describe("constants", () => {
    it("ACCESS_TOKEN_MAX_AGE is 1 hour in seconds", () => {
      expect(ACCESS_TOKEN_MAX_AGE).toBe(3600);
    });

    it("REFRESH_TOKEN_MAX_AGE is 30 days in seconds", () => {
      expect(REFRESH_TOKEN_MAX_AGE).toBe(2592000);
    });
  });
});

describe("refreshTokens", () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("returns tokens on successful refresh", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        accessToken: "new-access",
        expiresAt: "2026-06-20T14:00:00Z",
      }),
      headers: {
        get: vi
          .fn()
          .mockReturnValue("refresh_token=new-refresh; Path=/; HttpOnly"),
      },
    });

    const result = await refreshTokens("old-refresh-token");
    expect(result).toEqual({
      accessToken: "new-access",
      refreshToken: "new-refresh",
      expiresAt: "2026-06-20T14:00:00Z",
    });
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/auth/refresh"),
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ refreshToken: "old-refresh-token" }),
      }),
    );
  });

  it("returns null when response is not ok", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
    });

    expect(await refreshTokens("bad-token")).toBeNull();
  });

  it("returns null when accessToken is missing from body", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ expiresAt: "2026-06-20T14:00:00Z" }),
      headers: { get: vi.fn().mockReturnValue("refresh_token=r; Path=/") },
    });

    expect(await refreshTokens("rt")).toBeNull();
  });

  it("returns null when expiresAt is missing from body", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ accessToken: "tok" }),
      headers: { get: vi.fn().mockReturnValue("refresh_token=r; Path=/") },
    });

    expect(await refreshTokens("rt")).toBeNull();
  });

  it("returns null when no rotated refresh_token in set-cookie", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        accessToken: "tok",
        expiresAt: "2026-06-20T14:00:00Z",
      }),
      headers: { get: vi.fn().mockReturnValue("session=abc; Path=/") },
    });

    expect(await refreshTokens("rt")).toBeNull();
  });

  it("returns null on network error", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("network error"));

    expect(await refreshTokens("rt")).toBeNull();
  });
});

describe("accessCookieMaxAge", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns correct seconds for a future expiresAt", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T12:00:00Z"));

    const expiresAt = "2026-06-20T12:02:00Z";
    expect(accessCookieMaxAge(expiresAt)).toBe(120);
  });

  it("returns 0 for an already-past expiresAt", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T12:00:00Z"));

    const expiresAt = "2026-06-20T11:59:00Z";
    expect(accessCookieMaxAge(expiresAt)).toBe(0);
  });

  it("floors fractional seconds", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T12:00:00.500Z"));

    const expiresAt = "2026-06-20T12:00:01Z"; // 0.5s remaining
    expect(accessCookieMaxAge(expiresAt)).toBe(0);
  });
});
