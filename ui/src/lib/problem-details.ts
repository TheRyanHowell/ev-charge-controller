import { NextResponse } from "next/server";

/**
 * Creates an RFC 7807 Problem Details response.
 * @see https://datatracker.ietf.org/doc/html/rfc7807
 */
export function problemResponse(
  detail: string,
  status: number,
  extra?: Record<string, unknown>,
): Response {
  const slug = detail.replace(/\s+/g, "-").toLowerCase();
  const body = {
    type: `about:blank#${slug}`,
    title: "Problem",
    status,
    detail,
    ...(extra ?? {}),
  };
  return NextResponse.json(body, {
    status,
    headers: { "Content-Type": "application/problem+json" },
  });
}
