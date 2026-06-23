import { describe, it, expect, vi, beforeEach } from "vitest";

import { DELETE, GET, POST } from "./route";

const createRequest = (body = {}) =>
  ({
    json: () => Promise.resolve(body),
  }) as unknown as Parameters<typeof POST>[0];

vi.mock("next/server", () => {
  const NextResponseFn = function NextResponse(
    body: unknown,
    init: ResponseInit = {},
  ) {
    return new Response(body === null ? null : (body as BodyInit), init);
  };
  return {
    NextResponse: Object.assign(NextResponseFn, {
      json: (body: unknown, init: ResponseInit = {}) => {
        return new Response(JSON.stringify(body), {
          status: init.status ?? 200,
          headers: init.headers,
        });
      },
    }),
  };
});

describe("push-subscriptions API route", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("GET", () => {
    it("returns the VAPID public key", async () => {
      const original = process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY;
      process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY = "test_public_key";
      const response = await GET();
      process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY = original;
      const data = await response.json();
      expect(response.status).toBe(200);
      expect(data.publicKey).toBe("test_public_key");
    });

    it("returns empty string when key is not set", async () => {
      const original = process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY;
      delete process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY;
      const response = await GET();
      process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY = original;
      const data = await response.json();
      expect(response.status).toBe(200);
      expect(data.publicKey).toBe("");
    });
  });

  describe("POST", () => {
    it("proxies subscription to backend", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        status: 201,
        text: () =>
          Promise.resolve(
            JSON.stringify({ id: 1, endpoint: "https://push.example/sub1" }),
          ),
        headers: {
          get: (key: string) =>
            key === "content-type" ? "application/json" : "",
        },
      });

      const response = await POST(
        createRequest({
          endpoint: "https://push.example/sub1",
          keys: { p256dh: "abc", auth: "xyz" },
        }),
      );
      const data = await response.json();
      expect(response.status).toBe(201);
      expect(data.id).toBe(1);
    });

    it("passes through 204 response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        status: 204,
      });

      const response = await POST(createRequest({ endpoint: "x", keys: {} }));
      expect(response.status).toBe(204);
    });

    it("returns 500 on network error", async () => {
      global.fetch = async () => {
        throw new Error("connection refused");
      };

      const response = await POST(createRequest({ endpoint: "x", keys: {} }));
      expect(response.status).toBe(500);
    });

    it("returns 400 when body fails to parse", async () => {
      global.fetch = vi.fn();
      const badReq = { json: () => Promise.reject(new Error("invalid json")) };
      const response = await POST(badReq as any);
      expect(response.status).toBe(400);
    });

    it("passes through non-JSON response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        status: 200,
        text: () => Promise.resolve("plain text response"),
        headers: { get: () => "text/plain" },
      });

      const response = await POST(createRequest({ endpoint: "x" }));
      expect(response.status).toBe(200);
      const text = await response.text();
      expect(text).toBe("plain text response");
    });

    it("passes through empty response with status", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        status: 418,
        text: () => Promise.resolve(""),
        headers: { get: () => "" },
      });

      const response = await POST(createRequest({ endpoint: "x" }));
      expect(response.status).toBe(418);
    });
  });

  describe("DELETE", () => {
    it("proxies unsubscribe to backend", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        status: 200,
        text: () =>
          Promise.resolve(JSON.stringify({ message: "unsubscribed" })),
        headers: {
          get: (key: string) =>
            key === "content-type" ? "application/json" : "",
        },
      });

      const response = await DELETE(
        createRequest({ endpoint: "https://push.example/sub1" }),
      );
      const data = await response.json();
      expect(response.status).toBe(200);
      expect(data.message).toBe("unsubscribed");
    });

    it("passes through 204 response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        status: 204,
      });

      const response = await DELETE(createRequest({ endpoint: "x" }));
      expect(response.status).toBe(204);
    });

    it("returns 500 on network error", async () => {
      global.fetch = async () => {
        throw new Error("ECONNREFUSED");
      };

      const response = await DELETE(createRequest({ endpoint: "x" }));
      expect(response.status).toBe(500);
    });

    it("returns 400 when body fails to parse", async () => {
      global.fetch = vi.fn();
      const badReq = { json: () => Promise.reject(new Error("invalid json")) };
      const response = await DELETE(badReq as any);
      expect(response.status).toBe(400);
    });

    it("passes through non-JSON response", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        status: 200,
        text: () => Promise.resolve("plain text response"),
        headers: { get: () => "text/plain" },
      });

      const response = await DELETE(createRequest({ endpoint: "x" }));
      expect(response.status).toBe(200);
      const text = await response.text();
      expect(text).toBe("plain text response");
    });

    it("passes through empty response with status", async () => {
      global.fetch = vi.fn().mockResolvedValue({
        status: 503,
        text: () => Promise.resolve(""),
        headers: { get: () => "" },
      });

      const response = await DELETE(createRequest({ endpoint: "x" }));
      expect(response.status).toBe(503);
    });
  });
});
