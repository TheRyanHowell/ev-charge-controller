import {
  ACCESS_TOKEN_MAX_AGE,
  REFRESH_TOKEN_MAX_AGE,
  isCookieSecure,
  refreshAccessTokenPair,
} from "@/lib/auth-refresh";
import { problemResponse } from "@/lib/problem-details";
import { cookies } from "next/headers";
import { NextResponse } from "next/server";

const API_URL = process.env.API_URL || "http://localhost:8080";

async function incomingAuthHeader(): Promise<Record<string, string>> {
  try {
    const store = await cookies();
    const token = store.get("access_token")?.value;
    return token ? { Authorization: `Bearer ${token}` } : {};
  } catch {
    return {};
  }
}

interface ProxyOptions {
  path: string;
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
  handleSearchParams?: (searchParams: URLSearchParams) => URLSearchParams;
  detail?: string;
}

function buildFetchInit(
  method: string,
  authHeaders: Record<string, string>,
  opts: ProxyOptions,
): RequestInit {
  const init: RequestInit = { method, cache: "no-store" };
  if (opts.body !== undefined && method !== "GET") {
    init.headers = {
      "Content-Type": "application/json",
      ...authHeaders,
      ...opts.headers,
    };
    init.body = JSON.stringify(opts.body);
  } else if (opts.headers || Object.keys(authHeaders).length > 0) {
    init.headers = { ...authHeaders, ...opts.headers };
  }
  return init;
}

function toNextResponse(res: Response, text: string): NextResponse {
  if (!text) return new NextResponse(null, { status: res.status });
  const ct = res.headers.get("content-type") || "";
  if (ct.includes("application/json") || ct.includes("application/problem")) {
    return NextResponse.json(JSON.parse(text) as unknown, {
      status: res.status,
    });
  }
  return new NextResponse(text, { status: res.status });
}

async function handle401Retry(
  url: string,
  method: string,
  opts: ProxyOptions,
): Promise<NextResponse> {
  const tokens = await refreshAccessTokenPair();
  if (!tokens) {
    return NextResponse.json({ status: 401 }, { status: 401 });
  }

  const retryInit = buildFetchInit(
    method,
    { Authorization: `Bearer ${tokens.accessToken}` },
    opts,
  );
  const retryRes = await fetch(url, retryInit);
  const retryText = retryRes.status === 204 ? "" : await retryRes.text();
  const response = toNextResponse(retryRes, retryText);
  response.cookies.set("access_token", tokens.accessToken, {
    httpOnly: true,
    secure: isCookieSecure(),
    sameSite: "lax",
    maxAge: ACCESS_TOKEN_MAX_AGE,
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

/**
 * Proxies a request to the Go API backend.
 * On 401, attempts a silent token refresh and retries once.
 * If the retry response has a new access token, sets it as an httpOnly cookie.
 */
export async function proxyRequest(opts: ProxyOptions): Promise<Response> {
  const url = new URL(opts.path, API_URL);
  const method = opts.method ?? "GET";

  const authHeaders = await incomingAuthHeader();
  const init = buildFetchInit(method, authHeaders, opts);

  try {
    const res = await fetch(url.toString(), init);

    if (res.status === 204) return new NextResponse(null, { status: 204 });

    if (res.status === 401) {
      const retryRes = await handle401Retry(url.toString(), method, opts);
      return retryRes;
    }

    const text = await res.text();
    return toNextResponse(res, text);
  } catch {
    return problemResponse("Internal server error", 500);
  }
}

/**
 * Proxies a GET request with validated search params.
 * On 401, attempts a silent token refresh and retries once.
 * Returns a problem response on non-ok status.
 */
export async function proxyGet(opts: {
  path: string;
  searchParams?: URLSearchParams;
  validate?: (params: URLSearchParams) => URLSearchParams;
  detail?: string;
}): Promise<Response> {
  const url = new URL(opts.path, API_URL);

  if (opts.searchParams) {
    const validated = opts.validate
      ? opts.validate(opts.searchParams)
      : opts.searchParams;
    if (validated.toString()) {
      url.search = validated.toString();
    }
  }

  const defaultDetail = `Failed to fetch data`;
  const detail = opts.detail || defaultDetail;

  const authHeaders = await incomingAuthHeader();

  try {
    const res = await fetch(url.toString(), {
      cache: "no-store",
      ...(Object.keys(authHeaders).length > 0 && { headers: authHeaders }),
    });

    if (res.status === 204) {
      return new Response(null, { status: 204 });
    }

    if (res.status === 401) {
      return handle401Retry(url.toString(), "GET", opts);
    }

    if (!res.ok) {
      return problemResponse(detail, res.status);
    }
    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch {
    return problemResponse(detail, 500);
  }
}
