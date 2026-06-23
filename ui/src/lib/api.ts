import { GENERIC_FALLBACK, handleError } from "@/hooks/useErrorHandling";
import { z } from "zod";

const JSON_HEADERS = { "Content-Type": "application/json" };

interface ApiOptions {
  signal?: AbortSignal;
}

export class ApiError extends Error {
  public readonly status: number;

  public constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

async function throwApiError(res: Response): Promise<never> {
  throw new ApiError(await handleError(res), res.status);
}

function throwNetworkError(): never {
  throw new Error(GENERIC_FALLBACK);
}

async function fetchWithOptions(
  url: string,
  init: RequestInit,
  options?: ApiOptions,
): Promise<Response> {
  try {
    const headers = (init.headers as Record<string, string>) ?? {};
    const hasHeaders = Object.keys(headers).length > 0;
    const hasSignal = !!options?.signal;
    const hasInit = Object.keys(init).length > 0;

    if (!hasHeaders && !hasSignal && !hasInit) {
      return await fetch(url, { cache: "no-store" });
    }

    return await fetch(url, {
      ...init,
      cache: "no-store",
      ...(hasHeaders && { headers }),
      ...(hasSignal && { signal: options.signal }),
    });
  } catch {
    throwNetworkError();
  }
}

async function parseJsonResponse<S extends z.ZodType>(
  res: Response,
  schema: S,
): Promise<z.infer<S>> {
  if (!res.ok) await throwApiError(res);
  const raw = await res.json();
  return schema.parse(raw);
}

async function parseJsonResponseArray<S extends z.ZodType>(
  res: Response,
  schema: S,
): Promise<z.infer<S>[]> {
  if (!res.ok) await throwApiError(res);
  const raw = await res.json();
  return schema.array().parse(raw);
}

async function parseJsonResponseOrNull<S extends z.ZodType>(
  res: Response,
  schema: S,
): Promise<z.infer<S> | null> {
  if (res.status === 204) return null;
  if (!res.ok) await throwApiError(res);
  const raw = await res.json();
  return schema.parse(raw);
}

export async function apiGet<S extends z.ZodType>(
  url: string,
  schema: S,
  options?: ApiOptions,
): Promise<z.infer<S>[]> {
  const res = await fetchWithOptions(url, {}, options);
  if (res.status === 204) return [];
  return parseJsonResponseArray(res, schema);
}

export async function apiGetSingle<S extends z.ZodType>(
  url: string,
  schema: S,
  options?: ApiOptions,
): Promise<z.infer<S> | null> {
  const res = await fetchWithOptions(url, {}, options);
  return parseJsonResponseOrNull(res, schema);
}

export async function apiPost<S extends z.ZodType>(
  url: string,
  schema: S,
  body: Record<string, unknown>,
  options?: ApiOptions,
): Promise<z.infer<S>> {
  const res = await fetchWithOptions(
    url,
    {
      method: "POST",
      headers: JSON_HEADERS,
      body: JSON.stringify(body),
    },
    options,
  );
  return parseJsonResponse(res, schema);
}

export async function apiPostNullable<S extends z.ZodType>(
  url: string,
  schema: S,
  body: Record<string, unknown>,
  options?: ApiOptions,
): Promise<z.infer<S> | null> {
  const res = await fetchWithOptions(
    url,
    {
      method: "POST",
      headers: JSON_HEADERS,
      body: JSON.stringify(body),
    },
    options,
  );
  return parseJsonResponseOrNull(res, schema);
}

export async function apiPatch<S extends z.ZodType>(
  url: string,
  schema: S,
  body: Record<string, unknown>,
  options?: ApiOptions,
): Promise<z.infer<S>> {
  const res = await fetchWithOptions(
    url,
    {
      method: "PATCH",
      headers: JSON_HEADERS,
      body: JSON.stringify(body),
    },
    options,
  );
  return parseJsonResponse(res, schema);
}

export async function apiPut<S extends z.ZodType>(
  url: string,
  schema: S,
  body: Record<string, unknown>,
  options?: ApiOptions,
): Promise<z.infer<S>> {
  const res = await fetchWithOptions(
    url,
    {
      method: "PUT",
      headers: JSON_HEADERS,
      body: JSON.stringify(body),
    },
    options,
  );
  return parseJsonResponse(res, schema);
}

export async function apiPatchRaw(
  url: string,
  body: Record<string, unknown>,
  options?: ApiOptions,
): Promise<boolean> {
  try {
    const res = await fetchWithOptions(
      url,
      {
        method: "PATCH",
        headers: JSON_HEADERS,
        body: JSON.stringify(body),
      },
      options,
    );
    return res.ok;
  } catch {
    return false;
  }
}

export async function apiPatchNoContent(
  url: string,
  body: Record<string, unknown>,
  options?: ApiOptions,
): Promise<void> {
  const res = await fetchWithOptions(
    url,
    {
      method: "PATCH",
      headers: JSON_HEADERS,
      body: JSON.stringify(body),
    },
    options,
  );
  if (!res.ok) await throwApiError(res);
}

export async function apiDelete(
  url: string,
  options?: ApiOptions,
): Promise<void> {
  const res = await fetchWithOptions(url, { method: "DELETE" }, options);
  if (!res.ok) await throwApiError(res);
}

export async function apiOk(
  url: string,
  options?: ApiOptions,
): Promise<boolean> {
  try {
    const res = await fetchWithOptions(url, {}, options);
    return res.ok;
  } catch {
    return false;
  }
}
