import { describe, it, expect, vi } from "vitest";

import { problemResponse } from "./problem-details";

vi.mock("next/server", () => ({
  NextResponse: {
    json: (body: unknown, init: ResponseInit) => {
      return new Response(JSON.stringify(body), {
        status: init.status ?? 200,
        ...(init.headers && { headers: init.headers }),
      });
    },
  },
}));

describe("problemResponse", () => {
  it("creates correct RFC 7807 body", async () => {
    const response = problemResponse("Something went wrong", 500);
    const body = await response.json();
    expect(body.type).toBe("about:blank#something-went-wrong");
    expect(body.title).toBe("Problem");
    expect(body.status).toBe(500);
    expect(body.detail).toBe("Something went wrong");
  });

  it("derives slug from detail string", async () => {
    const response = problemResponse("Failed to fetch vehicles", 404);
    const body = await response.json();
    expect(body.type).toBe("about:blank#failed-to-fetch-vehicles");
  });

  it("merges extra fields into body", async () => {
    const response = problemResponse("Error", 400, {
      code: "NETWORK_DOWN",
      traceId: "abc123",
    });
    const body = await response.json();
    expect(body.code).toBe("NETWORK_DOWN");
    expect(body.traceId).toBe("abc123");
  });

  it("has correct Content-Type header", async () => {
    const response = problemResponse("Error", 500);
    expect(response.headers.get("content-type")).toBe(
      "application/problem+json",
    );
  });

  it("passes status code through", async () => {
    const r400 = problemResponse("Bad request", 400);
    const r404 = problemResponse("Not found", 404);
    const r500 = problemResponse("Internal error", 500);
    expect(r400.status).toBe(400);
    expect(r404.status).toBe(404);
    expect(r500.status).toBe(500);
  });

  it("does not spread extra when omitted", async () => {
    const response = problemResponse("Error", 500);
    const body = await response.json();
    expect(Object.keys(body).sort()).toEqual([
      "detail",
      "status",
      "title",
      "type",
    ]);
  });
});
