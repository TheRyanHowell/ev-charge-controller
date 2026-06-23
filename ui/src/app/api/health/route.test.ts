import { describe, it, expect, vi } from "vitest";

import { GET } from "./route";

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

describe("GET /api/health", () => {
  it("returns { status: 'ok' } with 200", async () => {
    const response = await GET();
    const data = await response.json();

    expect(response.status).toBe(200);
    expect(data).toEqual({ status: "ok" });
  });
});
