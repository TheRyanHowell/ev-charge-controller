import {
  REFRESH_TOKEN_MAX_AGE,
  accessCookieMaxAge,
  extractRefreshTokenFromSetCookie,
  isCookieSecure,
} from "@/lib/auth-refresh";
import { NextRequest, NextResponse } from "next/server";

const API_URL = process.env.API_URL || "http://localhost:8080";

export async function POST(req: NextRequest) {
  let body: unknown;
  try {
    body = await req.json();
  } catch {
    return NextResponse.json(
      {
        type: "about:blank",
        title: "Bad Request",
        status: 400,
        detail: "invalid JSON",
      },
      { status: 400, headers: { "Content-Type": "application/problem+json" } },
    );
  }

  let res: Response;
  try {
    res = await fetch(`${API_URL}/api/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
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
  const refreshToken = extractRefreshTokenFromSetCookie(setCookieHeader);

  const response = NextResponse.json(data, { status: 200 });

  if (data.accessToken) {
    response.cookies.set("access_token", data.accessToken, {
      httpOnly: true,
      secure: isCookieSecure(),
      sameSite: "lax",
      maxAge: data.expiresAt ? accessCookieMaxAge(data.expiresAt) : 0,
      path: "/",
    });
  }

  if (refreshToken) {
    response.cookies.set("refresh_token", refreshToken, {
      httpOnly: true,
      secure: isCookieSecure(),
      sameSite: "lax",
      maxAge: REFRESH_TOKEN_MAX_AGE,
      path: "/",
    });
  }

  return response;
}
