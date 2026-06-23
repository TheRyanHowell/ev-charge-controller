import { isCookieSecure } from "@/lib/auth-refresh";
import { NextRequest, NextResponse } from "next/server";

const API_URL = process.env.API_URL || "http://localhost:8080";

export async function POST(req: NextRequest) {
  const refreshToken = req.cookies.get("refresh_token")?.value;

  if (refreshToken) {
    try {
      await fetch(`${API_URL}/api/auth/logout`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refreshToken }),
      });
    } catch {
      // Best-effort revocation; always clear cookies
    }
  }

  const response = new NextResponse(null, { status: 204 });
  response.cookies.set("access_token", "", {
    httpOnly: true,
    secure: isCookieSecure(),
    sameSite: "strict",
    maxAge: -1,
    path: "/",
  });
  response.cookies.set("refresh_token", "", {
    httpOnly: true,
    secure: isCookieSecure(),
    sameSite: "strict",
    maxAge: -1,
    path: "/",
  });
  return response;
}
