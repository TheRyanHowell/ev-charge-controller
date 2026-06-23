const JWT_SKEW_SECONDS = 30;

export function decodeJwtExp(token: string): number | null {
  const parts = token.split(".");
  if (parts.length !== 3) return null;

  const payload = parts[1];
  if (!payload) return null;

  try {
    const base64 = payload.replace(/-/g, "+").replace(/_/g, "/");
    const padded = base64 + "=".repeat((4 - (base64.length % 4)) % 4);
    const json = atob(padded);
    const parsed = JSON.parse(json) as Record<string, unknown>;
    if (typeof parsed.exp !== "number") return null;
    return parsed.exp;
  } catch {
    return null;
  }
}

export function isTokenExpiringSoon(
  token: string | undefined | null,
  skewSeconds = JWT_SKEW_SECONDS,
): boolean {
  if (!token) return true;
  const exp = decodeJwtExp(token);
  if (exp === null) return true;
  return exp <= Math.floor(Date.now() / 1000) + skewSeconds;
}
