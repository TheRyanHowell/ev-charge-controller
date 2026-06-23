import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock next/headers cookies
const mockCookies = vi.fn();
vi.mock("next/headers", () => ({
  cookies: () => mockCookies(),
}));

// Mock global fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Mock auth-refresh
const mockRefreshAccessTokenPair = vi.fn();
vi.mock("@/lib/auth-refresh", () => ({
  refreshAccessTokenPair: () => mockRefreshAccessTokenPair(),
  isCookieSecure: () => false,
  ACCESS_TOKEN_MAX_AGE: 3600,
  REFRESH_TOKEN_MAX_AGE: 604800,
}));

// Mock problemResponse
const mockProblemResponse = vi.fn((detail: string, status: number) =>
  Response.json({ title: "Error", detail, status }, { status }),
);
vi.mock("@/lib/problem-details", () => ({
  problemResponse: (detail: string, status: number) =>
    mockProblemResponse(detail, status),
}));

import { proxyRequest, proxyGet } from "./api-proxy";

const mockCookieStore = {
  get: vi.fn(),
};

describe("proxyRequest", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCookies.mockReturnValue(mockCookieStore);
    mockCookieStore.get.mockReturnValue(undefined);
    mockRefreshAccessTokenPair.mockResolvedValue(null);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("proxies GET request without auth when no cookie", async () => {
    const mockResponse = {
      ok: true,
      status: 200,
      text: () => Promise.resolve(JSON.stringify({ data: "ok" })),
      headers: { get: () => "application/json" },
    };
    mockFetch.mockResolvedValue(mockResponse);

    const res = await proxyRequest({ path: "/api/vehicles" });

    expect(res.status).toBe(200);
    const body = await res.json();
    expect(body).toEqual({ data: "ok" });
  });

  it("proxies GET request with auth when cookie exists", async () => {
    mockCookieStore.get.mockImplementation((name) => {
      if (name === "access_token") return { value: "my-token" };
      return undefined;
    });

    const mockResponse = {
      ok: true,
      status: 200,
      text: () => Promise.resolve(JSON.stringify({ data: "ok" })),
      headers: { get: () => "application/json" },
    };
    mockFetch.mockResolvedValue(mockResponse);

    await proxyRequest({ path: "/api/vehicles" });

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/vehicles"),
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer my-token",
        }),
      }),
    );
  });

  it("handles 204 response", async () => {
    mockFetch.mockResolvedValue({ ok: false, status: 204 });

    const res = await proxyRequest({ path: "/api/vehicles" });

    expect(res.status).toBe(204);
  });

  it("handles 401 with retry when refresh succeeds", async () => {
    mockCookieStore.get.mockImplementation((name) => {
      if (name === "access_token") return { value: "expired-token" };
      return undefined;
    });

    mockRefreshAccessTokenPair.mockResolvedValue({
      accessToken: "refreshed-token",
      refreshToken: "new-refresh",
    });

    mockFetch
      .mockResolvedValueOnce({ ok: false, status: 401 })
      .mockResolvedValue({
        ok: true,
        status: 200,
        text: () => Promise.resolve(JSON.stringify({ data: "ok" })),
        headers: { get: () => "application/json" },
      });

    const res = await proxyRequest({ path: "/api/vehicles" });

    expect(res.status).toBe(200);
    expect(mockRefreshAccessTokenPair).toHaveBeenCalled();
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      expect.stringContaining("/api/vehicles"),
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer refreshed-token",
        }),
      }),
    );
  });

  it("returns 401 when refresh fails on 401 retry", async () => {
    mockCookieStore.get.mockImplementation((name) => {
      if (name === "access_token") return { value: "expired-token" };
      return undefined;
    });

    mockRefreshAccessTokenPair.mockResolvedValue(null);
    mockFetch.mockResolvedValue({ ok: false, status: 401 });

    const res = await proxyRequest({ path: "/api/vehicles" });

    expect(res.status).toBe(401);
  });

  it("returns 500 problem response on fetch error", async () => {
    mockFetch.mockRejectedValue(new Error("network error"));

    const res = await proxyRequest({ path: "/api/vehicles" });

    expect(res.status).toBe(500);
  });

  it("handles incomingAuthHeader catch when cookies() throws", async () => {
    mockCookies.mockRejectedValue(new Error("cookies error"));

    const mockResponse = {
      ok: true,
      status: 200,
      text: () => Promise.resolve(JSON.stringify({ data: "ok" })),
      headers: { get: () => "application/json" },
    };
    mockFetch.mockResolvedValue(mockResponse);

    const res = await proxyRequest({ path: "/api/vehicles" });

    expect(res.status).toBe(200);
    // Should still work - auth headers are empty when cookies() throws
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/vehicles"),
      expect.objectContaining({
        method: "GET",
        cache: "no-store",
      }),
    );
  });

  it("proxies POST request with body", async () => {
    mockCookieStore.get.mockImplementation((name) => {
      if (name === "access_token") return { value: "my-token" };
      return undefined;
    });

    const mockResponse = {
      ok: true,
      status: 201,
      text: () => Promise.resolve(JSON.stringify({ id: "created" })),
      headers: { get: () => "application/json" },
    };
    mockFetch.mockResolvedValue(mockResponse);

    await proxyRequest({
      path: "/api/plugs",
      method: "POST",
      body: { name: "New Plug" },
    });

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/plugs"),
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          "Content-Type": "application/json",
          Authorization: "Bearer my-token",
        }),
        body: JSON.stringify({ name: "New Plug" }),
      }),
    );
  });

  it("proxies non-JSON response", async () => {
    const mockResponse = {
      ok: true,
      status: 200,
      text: () => Promise.resolve("plain text"),
      headers: { get: () => "text/plain" },
    };
    mockFetch.mockResolvedValue(mockResponse);

    const res = await proxyRequest({ path: "/api/health" });

    expect(res.status).toBe(200);
    const text = await res.text();
    expect(text).toBe("plain text");
  });
});

describe("proxyGet", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCookies.mockReturnValue(mockCookieStore);
    mockCookieStore.get.mockReturnValue(undefined);
    mockRefreshAccessTokenPair.mockResolvedValue(null);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("proxies GET request and returns JSON", async () => {
    const mockResponse = {
      ok: true,
      status: 200,
      json: () => Promise.resolve({ vehicles: [] }),
    };
    mockFetch.mockResolvedValue(mockResponse);

    const res = await proxyGet({ path: "/api/vehicles" });

    expect(res.status).toBe(200);
    const body = await res.json();
    expect(body).toEqual({ vehicles: [] });
  });

  it("passes search params to URL", async () => {
    const mockResponse = {
      ok: true,
      status: 200,
      json: () => Promise.resolve({ vehicles: [] }),
    };
    mockFetch.mockResolvedValue(mockResponse);

    const params = new URLSearchParams({ limit: "10" });
    await proxyGet({ path: "/api/vehicles", searchParams: params });

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("?limit=10"),
      expect.anything(),
    );
  });

  it("applies validate function to search params", async () => {
    const mockResponse = {
      ok: true,
      status: 200,
      json: () => Promise.resolve({ vehicles: [] }),
    };
    mockFetch.mockResolvedValue(mockResponse);

    const params = new URLSearchParams({ limit: "10", page: "1" });
    const validate = (p: URLSearchParams) => {
      p.set("limit", "5");
      p.delete("page");
      return p;
    };

    await proxyGet({
      path: "/api/vehicles",
      searchParams: params,
      validate,
    });

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("?limit=5"),
      expect.anything(),
    );
    expect(mockFetch).not.toHaveBeenCalledWith(
      expect.stringContaining("page="),
      expect.anything(),
    );
  });

  it("handles 204 response", async () => {
    mockFetch.mockResolvedValue({ ok: false, status: 204 });

    const res = await proxyGet({ path: "/api/vehicles" });

    expect(res.status).toBe(204);
  });

  it("handles 401 with retry when refresh succeeds", async () => {
    mockCookieStore.get.mockImplementation((name) => {
      if (name === "access_token") return { value: "expired-token" };
      return undefined;
    });

    mockRefreshAccessTokenPair.mockResolvedValue({
      accessToken: "refreshed-token",
      refreshToken: "new-refresh",
    });

    mockFetch
      .mockResolvedValueOnce({ ok: false, status: 401 })
      .mockResolvedValue({
        ok: true,
        status: 200,
        text: () => Promise.resolve(JSON.stringify({ data: "ok" })),
        headers: { get: () => "application/json" },
      });

    const res = await proxyGet({ path: "/api/vehicles" });

    expect(res.status).toBe(200);
    expect(mockRefreshAccessTokenPair).toHaveBeenCalled();
  });

  it("returns problem response on non-ok status", async () => {
    mockFetch.mockResolvedValue({ ok: false, status: 500 });

    const res = await proxyGet({ path: "/api/vehicles" });

    expect(res.status).toBe(500);
  });

  it("uses custom detail in problem response", async () => {
    mockFetch.mockResolvedValue({ ok: false, status: 404 });

    const res = await proxyGet({
      path: "/api/vehicles",
      detail: "Vehicles not found",
    });

    expect(res.status).toBe(404);
  });

  it("returns 500 on fetch error", async () => {
    mockFetch.mockRejectedValue(new Error("network error"));

    const res = await proxyGet({ path: "/api/vehicles" });

    expect(res.status).toBe(500);
  });

  it("handles validate returning empty params", async () => {
    const mockResponse = {
      ok: true,
      status: 200,
      json: () => Promise.resolve({ vehicles: [] }),
    };
    mockFetch.mockResolvedValue(mockResponse);

    const params = new URLSearchParams({ limit: "10" });
    const validate = () => new URLSearchParams();

    await proxyGet({
      path: "/api/vehicles",
      searchParams: params,
      validate,
    });

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/vehicles"),
      expect.anything(),
    );
    // Should NOT have search params since validate returned empty
    expect(mockFetch).not.toHaveBeenCalledWith(
      expect.stringContaining("?"),
      expect.anything(),
    );
  });
});
