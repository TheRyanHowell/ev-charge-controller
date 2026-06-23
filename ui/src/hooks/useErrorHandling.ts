import { ERROR_AUTO_DISMISS_MS } from "@/lib/constants";
import { useCallback, useEffect, useState } from "react";

interface ProblemDetails {
  type: string;
  title: string;
  status: number;
  detail?: string;
}

const FALLBACK_MESSAGES: Record<number, string> = {
  400: "Bad request. Please check your input.",
  404: "Resource not found.",
  409: "Conflict. Please try again.",
  500: "Internal server error. Please try again later.",
  502: "Bad gateway. Please try again later.",
  503: "Service unavailable. Please try again later.",
};

export const GENERIC_FALLBACK = "Something went wrong. Please try again.";

function isProblemDetails(parsed: unknown): parsed is ProblemDetails {
  if (parsed == null || typeof parsed !== "object") return false;
  const obj = parsed as Record<string, unknown>;
  return (
    typeof obj.type === "string" &&
    typeof obj.title === "string" &&
    typeof obj.status === "number"
  );
}

function parseProblem(text: string): ProblemDetails | null {
  try {
    const parsed = JSON.parse(text);
    if (isProblemDetails(parsed)) return parsed;
  } catch {
    // Not JSON
  }
  return null;
}

export async function handleError(response: Response): Promise<string> {
  // NOTE: Consumes response.body via text(). Only call on error responses
  // where the body won't be read again (e.g., after !res.ok check).
  const text = typeof response.text === "function" ? await response.text() : "";
  const problem = parseProblem(text);

  if (problem?.detail) return problem.detail;

  if (problem?.status && FALLBACK_MESSAGES[problem.status]) {
    return FALLBACK_MESSAGES[problem.status] ?? GENERIC_FALLBACK;
  }

  if (problem?.title) return problem.title;

  return GENERIC_FALLBACK;
}

export function useErrorHandling() {
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  useEffect(() => {
    if (errorMessage) {
      const timer = setTimeout(() => {
        setErrorMessage(null);
      }, ERROR_AUTO_DISMISS_MS);
      return () => clearTimeout(timer);
    }
  }, [errorMessage]);

  const onError = useCallback((msg: string) => {
    setErrorMessage(msg);
  }, []);

  const clearError = useCallback(() => {
    setErrorMessage(null);
  }, []);

  return { errorMessage, onError, clearError };
}
