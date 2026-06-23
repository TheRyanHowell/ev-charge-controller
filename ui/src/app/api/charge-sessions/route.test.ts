import { describe, it, expect, vi } from "vitest";

import { GET, POST, PATCH, DELETE } from "./route";

const createRequest = (
  body: Record<string, unknown> = {},
  searchParams: Record<string, string> = {},
) =>
  ({
    json: () => Promise.resolve(body),
    nextUrl: {
      searchParams: {
        get: (key: string) => searchParams[key] ?? null,
      },
    },
  }) as any;

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

describe("GET /api/charge-sessions", () => {
  it("forwards to backend and returns response", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 200,
      text: () =>
        Promise.resolve(JSON.stringify({ id: "sess1", status: "active" })),
      headers: {
        get: (key: string) =>
          key === "content-type" ? "application/json" : "",
      },
    });

    const response = await GET(createRequest());
    const data = await response.json();
    expect(response.status).toBe(200);
    expect(data.id).toBe("sess1");
  });

  it("passes through 204 response", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 204,
    });

    const response = await GET(createRequest());
    expect(response.status).toBe(204);
  });

  it("returns text for non-JSON response", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 200,
      text: () => Promise.resolve("plain text"),
      headers: {
        get: (key: string) => (key === "content-type" ? "text/plain" : ""),
      },
    });

    const response = await GET(createRequest());
    expect(response.status).toBe(200);
    expect(await response.text()).toBe("plain text");
  });

  it("returns 500 on network error", async () => {
    global.fetch = async () => {
      throw new Error("Network error");
    };

    const response = await GET(createRequest());
    expect(response.status).toBe(500);
  });

  it("handles empty body response (non-204 with empty text)", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 200,
      text: () => Promise.resolve(""),
      headers: { get: (_key: string) => "" },
    });

    const response = await GET(createRequest());
    expect(response.status).toBe(200);
    const body = await response.text();
    expect(body).toBe("");
  });
});

describe("POST /api/charge-sessions", () => {
  it("forwards body and returns response", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 201,
      text: () =>
        Promise.resolve(JSON.stringify({ id: "sess2", status: "active" })),
      headers: {
        get: (key: string) =>
          key === "content-type" ? "application/json" : "",
      },
    });

    const response = await POST(
      createRequest({ vehicleId: "rm1", startPercent: 20, targetPercent: 80 }),
    );
    const data = await response.json();
    expect(response.status).toBe(201);
    expect(data.id).toBe("sess2");
  });

  it("passes through 204 response", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 204,
    });

    const response = await POST(createRequest({ vehicleId: "rm1" }));
    expect(response.status).toBe(204);
  });

  it("returns 500 on network error", async () => {
    global.fetch = async () => {
      throw new Error("Network error");
    };

    const response = await POST(createRequest({ vehicleId: "rm1" }));
    expect(response.status).toBe(500);
  });
});

describe("PATCH /api/charge-sessions", () => {
  it("forwards body and returns response", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 200,
      text: () =>
        Promise.resolve(
          JSON.stringify({ status: "active", targetPercent: 90 }),
        ),
      headers: {
        get: (key: string) =>
          key === "content-type" ? "application/json" : "",
      },
    });

    const response = await PATCH(createRequest({ targetPercent: 90 }));
    const data = await response.json();
    expect(response.status).toBe(200);
    expect(data.targetPercent).toBe(90);
  });

  it("returns 500 on network error", async () => {
    global.fetch = async () => {
      throw new Error("Network error");
    };

    const response = await PATCH(createRequest({ targetPercent: 90 }));
    expect(response.status).toBe(500);
  });
});

describe("DELETE /api/charge-sessions", () => {
  it("deletes specific session by id query param", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 200,
      text: () => Promise.resolve(JSON.stringify({ deleted: true })),
      headers: {
        get: (key: string) =>
          key === "content-type" ? "application/json" : "",
      },
    });

    const req = {
      nextUrl: {
        searchParams: {
          get: (key: string) => (key === "id" ? "session-123" : null),
        },
      },
    } as any;

    const response = await DELETE(req);
    const data = await response.json();
    expect(response.status).toBe(200);
    expect(data.deleted).toBe(true);
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/charge-sessions/session-123"),
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("returns 400 when id param is missing", async () => {
    const req = {
      nextUrl: {
        searchParams: {
          get: () => null,
        },
      },
    } as any;

    const response = await DELETE(req);
    expect(response.status).toBe(400);
  });
});

describe("POST /api/charge-sessions JSON parse error", () => {
  it("returns 400 on invalid JSON body", async () => {
    const req = {
      json: async () => {
        throw new Error("invalid json");
      },
      nextUrl: {
        searchParams: {
          get: () => null,
        },
      },
    } as any;

    const response = await POST(req);
    expect(response.status).toBe(400);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
    const data = await response.json();
    expect(data.detail).toBe("Invalid request body");
  });
});

describe("PATCH /api/charge-sessions JSON parse error", () => {
  it("returns 400 on invalid JSON body", async () => {
    const req = {
      json: async () => {
        throw new Error("invalid json");
      },
      nextUrl: {
        searchParams: {
          get: () => null,
        },
      },
    } as any;

    const response = await PATCH(req);
    expect(response.status).toBe(400);
    expect(response.headers.get("content-type")).toContain(
      "application/problem+json",
    );
    const data = await response.json();
    expect(data.detail).toBe("Invalid request body");
  });
});
