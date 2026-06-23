import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const mockCookies = vi.fn();
vi.mock("next/headers", () => ({
  cookies: () => mockCookies(),
}));

const mockFetch = vi.fn();
global.fetch = mockFetch;

import { serverFetch } from "./server-fetch";

const mockCookieStore = {
  get: vi.fn(),
};

describe("serverFetch", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCookies.mockReturnValue(mockCookieStore);
    mockCookieStore.get.mockReturnValue(undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("uses accessToken from options when provided, skipping cookie read", async () => {
    const mockResponse = { ok: true, status: 200 };
    mockFetch.mockResolvedValue(mockResponse);

    const res = await serverFetch("/api/test", {
      accessToken: "provided-token",
    });

    expect(res.ok).toBe(true);
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/test"),
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer provided-token",
        }),
      }),
    );
    expect(mockCookieStore.get).not.toHaveBeenCalled();
  });

  it("reads access token from cookies when not provided in options", async () => {
    mockCookieStore.get.mockImplementation((name: string) => {
      if (name === "access_token") return { value: "cookie-token" };
      return undefined;
    });

    mockFetch.mockResolvedValue({ ok: true, status: 200 });

    await serverFetch("/api/test");

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/test"),
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer cookie-token",
        }),
      }),
    );
  });

  it("throws when no access token in options or cookies", async () => {
    mockCookieStore.get.mockReturnValue(undefined);

    await expect(serverFetch("/api/test")).rejects.toThrow(
      "Authentication required",
    );
  });

  it("returns 401 without retrying when backend returns 401", async () => {
    mockCookieStore.get.mockImplementation((name: string) => {
      if (name === "access_token") return { value: "token" };
      return undefined;
    });

    mockFetch.mockResolvedValue({ ok: false, status: 401 });

    const res = await serverFetch("/api/test");

    expect(res.status).toBe(401);
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it("sends Content-Type header for POST requests with body", async () => {
    mockCookieStore.get.mockImplementation((name: string) => {
      if (name === "access_token") return { value: "cookie-token" };
      return undefined;
    });

    mockFetch.mockResolvedValue({ ok: true, status: 201 });

    await serverFetch("/api/test", {
      method: "POST",
      body: { name: "test" },
    });

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/test"),
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          "Content-Type": "application/json",
          Authorization: "Bearer cookie-token",
        }),
        body: JSON.stringify({ name: "test" }),
      }),
    );
  });

  it("does not send Content-Type for GET requests", async () => {
    mockCookieStore.get.mockImplementation((name: string) => {
      if (name === "access_token") return { value: "cookie-token" };
      return undefined;
    });

    mockFetch.mockResolvedValue({ ok: true, status: 200 });

    await serverFetch("/api/test", { method: "GET" });

    const callArgs = mockFetch.mock.calls[0] as [string, RequestInit];
    const headers = callArgs[1].headers as Record<string, string>;
    expect(headers["Content-Type"]).toBeUndefined();
  });
});
