import { describe, it, expect, vi, afterEach } from "vitest";

import { decodeJwtExp, isTokenExpiringSoon } from "./jwt";

function makeTestJwt(payload: Record<string, unknown>): string {
  const header = btoa(JSON.stringify({ alg: "HS256", typ: "JWT" }))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=/g, "");
  const body = btoa(JSON.stringify(payload))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=/g, "");
  return `${header}.${body}.fakesignature`;
}

describe("decodeJwtExp", () => {
  it("returns exp for a valid JWT with future exp", () => {
    const exp = 9999999999;
    const token = makeTestJwt({ sub: "user1", exp });
    expect(decodeJwtExp(token)).toBe(exp);
  });

  it("returns exp for a valid JWT with past exp", () => {
    const exp = 1000000000;
    const token = makeTestJwt({ sub: "user1", exp });
    expect(decodeJwtExp(token)).toBe(exp);
  });

  it("handles base64url chars (- and _) correctly", () => {
    // Payload with chars that produce base64url special chars
    const exp = 9999999999;
    const token = makeTestJwt({
      sub: "1234567890",
      name: "John Doe",
      iat: 1516239022,
      exp,
    });
    expect(decodeJwtExp(token)).toBe(exp);
  });

  it("returns null for a token without 3 parts", () => {
    expect(decodeJwtExp("onlyone")).toBeNull();
    expect(decodeJwtExp("two.parts")).toBeNull();
    expect(decodeJwtExp("four.parts.is.too.many")).toBeNull();
  });

  it("returns null for a token with invalid base64 payload", () => {
    expect(decodeJwtExp("header.!!!invalid!!!.sig")).toBeNull();
  });

  it("returns null when payload JSON has no exp field", () => {
    const token = makeTestJwt({ sub: "user1", iat: 1234567890 });
    expect(decodeJwtExp(token)).toBeNull();
  });

  it("returns null when exp is not a number", () => {
    const token = makeTestJwt({ exp: "not-a-number" });
    expect(decodeJwtExp(token)).toBeNull();
  });

  it("returns null for empty string", () => {
    expect(decodeJwtExp("")).toBeNull();
  });

  it("returns null when payload is valid base64 but not JSON", () => {
    const header = "header";
    const payload = btoa("not json at all");
    expect(decodeJwtExp(`${header}.${payload}.sig`)).toBeNull();
  });
});

describe("isTokenExpiringSoon", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns true for null", () => {
    expect(isTokenExpiringSoon(null)).toBe(true);
  });

  it("returns true for undefined", () => {
    expect(isTokenExpiringSoon(undefined)).toBe(true);
  });

  it("returns true for empty string", () => {
    expect(isTokenExpiringSoon("")).toBe(true);
  });

  it("returns true for an undecodable token", () => {
    expect(isTokenExpiringSoon("bad.token")).toBe(true);
  });

  it("returns false for a token expiring far in the future", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T12:00:00Z"));

    const exp = Math.floor(Date.now() / 1000) + 3600;
    const token = makeTestJwt({ exp });
    expect(isTokenExpiringSoon(token)).toBe(false);
  });

  it("returns true for a token already expired", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T12:00:00Z"));

    const exp = Math.floor(Date.now() / 1000) - 60;
    const token = makeTestJwt({ exp });
    expect(isTokenExpiringSoon(token)).toBe(true);
  });

  it("returns true when exp is exactly now + skew (30s)", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T12:00:00Z"));

    const now = Math.floor(Date.now() / 1000);
    const token = makeTestJwt({ exp: now + 30 });
    expect(isTokenExpiringSoon(token)).toBe(true);
  });

  it("returns false when exp is now + skew + 1s", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T12:00:00Z"));

    const now = Math.floor(Date.now() / 1000);
    const token = makeTestJwt({ exp: now + 31 });
    expect(isTokenExpiringSoon(token)).toBe(false);
  });

  it("respects custom skew parameter", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T12:00:00Z"));

    const now = Math.floor(Date.now() / 1000);
    const token = makeTestJwt({ exp: now + 60 });

    expect(isTokenExpiringSoon(token, 0)).toBe(false);
    expect(isTokenExpiringSoon(token, 60)).toBe(true);
    expect(isTokenExpiringSoon(token, 59)).toBe(false);
  });
});
