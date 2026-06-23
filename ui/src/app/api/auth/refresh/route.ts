import {
  REFRESH_TOKEN_MAX_AGE,
  accessCookieMaxAge,
  extractRefreshTokenFromSetCookie,
  isCookieSecure,
} from "@/lib/auth-refresh";
import { NextRequest, NextResponse } from "next/server";

const API_URL = process.env.API_URL || "http://localhost:8080";

export async function POST(req: NextRequest) {
  const refreshToken = req.cookies.get("refresh_token")?.value;
  if (!refreshToken) {
    return NextResponse.json(
      {
        type: "about:blank",
        title: "Unauthorized",
        status: 401,
        detail: "refresh token required",
      },
      { status: 401, headers: { "Content-Type": "application/problem+json" } },
    );
  }

  let res: Response;
  try {
    res = await fetch(`${API_URL}/api/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refreshToken }),
    });
  } catch {
    return NextResponse.json(
      { type: "about:blank", title: "Internal Server Error", status: 500 },
      { status: 500, headers: { "Content-Type": "application/problem+json" } },
    );
  }

  const data = (await res.json()) as {
    accessToken?: string;
    expiresAt?: string;
    [key: string]: unknown;
  };

  if (!res.ok) {
    return NextResponse.json(data, { status: res.status });
  }

  const setCookieHeader = res.headers.get("set-cookie") ?? "";
  const newRefreshToken = extractRefreshTokenFromSetCookie(setCookieHeader);

  const response = NextResponse.json(data, { status: 200 });

  if (data.accessToken) {
    response.cookies.set("access_token", data.accessToken, {
      httpOnly: true,
      secure: isCookieSecure(),
      sameSite: "strict",
      maxAge: data.expiresAt ? accessCookieMaxAge(data.expiresAt) : 0,
      path: "/",
    });
  }

  if (newRefreshToken) {
    response.cookies.set("refresh_token", newRefreshToken, {
      httpOnly: true,
      secure: isCookieSecure(),
      sameSite: "strict",
      maxAge: REFRESH_TOKEN_MAX_AGE,
      path: "/",
    });
  }

  return response;
}
