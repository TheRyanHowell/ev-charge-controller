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

describe("POST /api/auth/register", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("registers user and returns data on success", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      json: () =>
        Promise.resolve({
          id: "user1",
          email: "new@example.com",
        }),
    });

    const req = makeRequest({
      email: "new@example.com",
      password: "secure123",
    });
    const response = await POST(req);
    const data = await response.json();

    expect(response.status).toBe(201);
    expect(data.email).toBe("new@example.com");
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/auth/register"),
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          email: "new@example.com",
          password: "secure123",
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
      makeRequest({ email: "new@example.com", password: "pass" }),
    );
    expect(response.status).toBe(500);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
  });

  it("passes through backend error responses", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      json: () =>
        Promise.resolve({
          type: "about:blank",
          title: "Conflict",
          status: 409,
          detail: "email already registered",
        }),
    });

    const response = await POST(
      makeRequest({ email: "existing@example.com", password: "pass" }),
    );
    expect(response.status).toBe(409);
    const data = await response.json();
    expect(data.detail).toBe("email already registered");
  });
});
