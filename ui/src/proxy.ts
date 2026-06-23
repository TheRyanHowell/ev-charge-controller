import {
  accessCookieMaxAge,
  isCookieSecure,
  REFRESH_TOKEN_MAX_AGE,
  refreshTokens,
} from "@/lib/auth-refresh";
import { isTokenExpiringSoon } from "@/lib/jwt";
import { NextRequest, NextResponse } from "next/server";

const SKIPPED_PATHS = [
  "/login",
  "/api/health",
  "/api/auth/login",
  "/api/auth/register",
  "/api/auth/refresh",
  "/api/auth/logout",
];

function isSkippedPath(pathname: string): boolean {
  return SKIPPED_PATHS.some(
    (p) => pathname === p || pathname.startsWith(p + "/"),
  );
}

function isApiRoute(pathname: string): boolean {
  return pathname.startsWith("/api/");
}

function unauthorizedApiResponse(): NextResponse {
  return NextResponse.json(
    {
      type: "about:blank",
      title: "Unauthorized",
      status: 401,
      detail: "session expired",
    },
    { status: 401, headers: { "Content-Type": "application/problem+json" } },
  );
}

function clearAccessToken(res: NextResponse): void {
  res.cookies.delete("access_token");
}

export async function proxy(req: NextRequest): Promise<NextResponse> {
  const { pathname } = req.nextUrl;

  if (isSkippedPath(pathname)) {
    return NextResponse.next();
  }

  const accessToken = req.cookies.get("access_token")?.value;
  const refreshToken = req.cookies.get("refresh_token")?.value;

  if (accessToken && !isTokenExpiringSoon(accessToken)) {
    return NextResponse.next();
  }

  if (refreshToken) {
    const tokens = await refreshTokens(refreshToken);

    if (tokens) {
      const requestHeaders = new Headers(req.headers);
      const otherCookies = req.headers
        .get("cookie")
        ?.split(";")
        .map((c) => c.trim())
        .filter(
          (c) =>
            !c.startsWith("access_token=") && !c.startsWith("refresh_token="),
        )
        .join("; ");
      const newCookies = [
        otherCookies,
        `access_token=${tokens.accessToken}`,
        `refresh_token=${tokens.refreshToken}`,
      ]
        .filter(Boolean)
        .join("; ");
      requestHeaders.set("cookie", newCookies);

      const response = NextResponse.next({
        request: { headers: requestHeaders },
      });
      response.cookies.set("access_token", tokens.accessToken, {
        httpOnly: true,
        secure: isCookieSecure(),
        sameSite: "lax",
        maxAge: accessCookieMaxAge(tokens.expiresAt),
        path: "/",
      });
      response.cookies.set("refresh_token", tokens.refreshToken, {
        httpOnly: true,
        secure: isCookieSecure(),
        sameSite: "lax",
        maxAge: REFRESH_TOKEN_MAX_AGE,
        path: "/",
      });

      return response;
    }

    if (isApiRoute(pathname)) {
      const res = unauthorizedApiResponse();
      clearAccessToken(res);
      return res;
    }

    const res = NextResponse.redirect(
      new URL("/login?reason=session-expired", req.url),
    );
    clearAccessToken(res);
    return res;
  }

  if (isApiRoute(pathname)) {
    return NextResponse.json(
      {
        type: "about:blank",
        title: "Unauthorized",
        status: 401,
        detail: "authentication required",
      },
      { status: 401, headers: { "Content-Type": "application/problem+json" } },
    );
  }

  return NextResponse.redirect(new URL("/login", req.url));
}

export const config = {
  matcher: [
    "/((?!_next/static|_next/image|favicon\\.ico|sw\\.js|.*\\.webmanifest$|.*\\.png$|.*\\.svg$|.*\\.ico$|.*\\.webp$).*)",
  ],
};
