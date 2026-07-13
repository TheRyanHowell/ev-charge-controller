import { describe, it, expect, vi } from "vitest";

import { DELETE } from "./route";

function makeParams(id: string) {
  return {
    params: Promise.resolve({ id }),
  };
}

// Regression: this proxy route was missing entirely, so deleting a session
// from the history page 404'd inside Next.js before ever reaching the Go API.
describe("DELETE /api/charge-sessions/:id", () => {
  it("proxies DELETE to the backend", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 204,
    });

    const ctx = makeParams("14630c22-cba5-4362-b91c-1e51471fe04d");
    const response = await DELETE({} as never, ctx);

    expect(response.status).toBe(204);
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining(
        "/api/charge-sessions/14630c22-cba5-4362-b91c-1e51471fe04d",
      ),
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("rejects an invalid session ID without calling the backend", async () => {
    global.fetch = vi.fn();

    const ctx = makeParams("../evil");
    const response = await DELETE({} as never, ctx);

    expect(response.status).toBe(400);
    expect(global.fetch).not.toHaveBeenCalled();
  });

  it("passes through backend errors (e.g. active session guard)", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      status: 409,
      ok: false,
      text: async () =>
        JSON.stringify({ title: "Conflict", detail: "session is active" }),
      headers: { get: () => "application/problem+json" },
    });

    const ctx = makeParams("14630c22-cba5-4362-b91c-1e51471fe04d");
    const response = await DELETE({} as never, ctx);

    expect(response.status).toBe(409);
  });
});
