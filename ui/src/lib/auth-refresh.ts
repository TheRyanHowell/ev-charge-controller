import { cookies } from "next/headers";

const API_URL = process.env.API_URL || "http://localhost:8080";

/**
 * Determines whether cookies should be marked as secure (HTTPS-only).
 * Controlled by COOKIE_SECURE env var; defaults to NODE_ENV === "production".
 * Override to "false" for HTTP-only environments (e2e tests, dev behind HTTP proxy).
 */
export function isCookieSecure(): boolean {
  const override = process.env.COOKIE_SECURE;
  if (override !== undefined) {
    return override.toLowerCase() === "true";
  }
  return process.env.NODE_ENV === "production";
}

export const ACCESS_TOKEN_MAX_AGE = 60 * 60;
export const REFRESH_TOKEN_MAX_AGE = 30 * 24 * 60 * 60;

interface RefreshedTokens {
  accessToken: string;
  refreshToken: string;
}

interface RefreshedTokensFull {
  accessToken: string;
  refreshToken: string;
  expiresAt: string;
}

/**
 * Pure refresh: no cookie I/O. Calls the backend with the given refresh token
 * and returns new tokens including the access-token expiry from the response body.
 */
export async function refreshTokens(
  refreshToken: string,
): Promise<RefreshedTokensFull | null> {
  try {
    const res = await fetch(`${API_URL}/api/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refreshToken }),
      cache: "no-store",
    });

    if (!res.ok) return null;

    const data = (await res.json()) as {
      accessToken?: string;
      expiresAt?: string;
    };
    const accessToken = data.accessToken;
    const expiresAt = data.expiresAt;
    if (!accessToken || !expiresAt) return null;

    const setCookieHeader = res.headers.get("set-cookie") ?? "";
    const newRefreshToken = extractRefreshTokenFromSetCookie(setCookieHeader);
    if (!newRefreshToken) return null;

    return { accessToken, refreshToken: newRefreshToken, expiresAt };
  } catch {
    return null;
  }
}

/**
 * Computes access-token cookie maxAge in seconds from the ISO 8601 expiresAt string.
 * Clamped to a minimum of 0 so we never set a negative maxAge.
 */
export function accessCookieMaxAge(expiresAt: string): number {
  const exp = new Date(expiresAt).getTime();
  return Math.max(0, Math.floor((exp - Date.now()) / 1000));
}

export function extractRefreshTokenFromSetCookie(
  setCookieHeader: string,
): string {
  const match = setCookieHeader.match(/(?:^|,\s*)refresh_token=([^;,\s]+)/);
  return match?.[1] ?? "";
}

let pendingRefresh: Promise<RefreshedTokens | null> | null = null;

export async function refreshAccessTokenPair(): Promise<RefreshedTokens | null> {
  if (pendingRefresh) {
    return pendingRefresh;
  }

  pendingRefresh = (async () => {
    try {
      const store = await cookies();
      const refreshToken = store.get("refresh_token")?.value;
      if (!refreshToken) return null;

      const res = await fetch(`${API_URL}/api/auth/refresh`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refreshToken }),
        cache: "no-store",
      });

      if (!res.ok) return null;

      const data = (await res.json()) as { accessToken?: string };
      const accessToken = data.accessToken;
      if (!accessToken) return null;

      const setCookieHeader = res.headers.get("set-cookie") ?? "";
      const newRefreshToken = extractRefreshTokenFromSetCookie(setCookieHeader);
      if (!newRefreshToken) return null;

      return { accessToken, refreshToken: newRefreshToken };
    } finally {
      pendingRefresh = null;
    }
  })();

  return pendingRefresh;
}
