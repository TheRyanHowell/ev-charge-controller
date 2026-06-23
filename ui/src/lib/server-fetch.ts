import { cookies } from "next/headers";

const API_URL = process.env.API_URL || "http://localhost:8080";

interface ServerFetchOptions {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
  /**
   * Pre-resolved access token. When provided, skips cookie read.
   */
  accessToken?: string;
}

/**
 * Fetches from the Go API with the current access token attached.
 * For server components. Middleware is responsible for refreshing expired tokens
 * before the page renders, so this function just attaches the token and forwards
 * the response - including 401 - to the caller.
 */
export async function serverFetch(
  path: string,
  opts: ServerFetchOptions = {},
): Promise<Response> {
  const url = new URL(path, API_URL);
  const method = opts.method ?? "GET";

  let accessToken: string | undefined;

  if (opts.accessToken) {
    accessToken = opts.accessToken;
  } else {
    const store = await cookies();
    accessToken = store.get("access_token")?.value;
  }

  if (!accessToken) {
    throw new Error("Authentication required");
  }

  const headers: Record<string, string> = {
    Authorization: `Bearer ${accessToken}`,
    ...opts.headers,
  };

  const init: RequestInit = {
    method,
    headers,
    cache: "no-store",
  };

  if (opts.body !== undefined && method !== "GET") {
    headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(opts.body);
  }

  return fetch(url.toString(), init);
}
