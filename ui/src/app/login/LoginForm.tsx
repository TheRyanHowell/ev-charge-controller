"use client";

import { useRouter } from "next/navigation";
import { useCallback, useRef, useState } from "react";

interface LoginState {
  email: string;
  password: string;
  error: string | null;
  submitting: boolean;
}

export function LoginForm() {
  const router = useRouter();
  const emailRef = useRef<HTMLInputElement>(null);
  const passwordRef = useRef<HTMLInputElement>(null);
  const [state, setState] = useState<LoginState>({
    email: "",
    password: "",
    error: null,
    submitting: false,
  });

  const handleSubmit = useCallback(
    async (e: React.FormEvent<HTMLFormElement>) => {
      e.preventDefault();
      setState((s) => ({ ...s, error: null, submitting: true }));

      // Read values directly from DOM via refs - immune to React event timing issues
      const email = emailRef.current?.value ?? "";
      const password = passwordRef.current?.value ?? "";

      try {
        const res = await fetch("/api/auth/login", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ email, password }),
        });

        if (res.ok) {
          router.push("/dashboard");
          router.refresh();
          return;
        }

        const data = (await res.json()) as { detail?: string };
        setState((s) => ({
          ...s,
          error: data.detail ?? "Login failed. Please check your credentials.",
          submitting: false,
        }));
      } catch {
        setState((s) => ({
          ...s,
          error: "Network error. Please try again.",
          submitting: false,
        }));
      }
    },
    [router],
  );

  return (
    <form onSubmit={handleSubmit} className="space-y-4" noValidate>
      <div>
        <label
          htmlFor="email"
          className="block text-sm font-medium text-fg mb-1"
        >
          Email address
        </label>
        <input
          ref={emailRef}
          id="email"
          type="email"
          autoComplete="email"
          required
          value={state.email}
          onChange={(e) => setState((s) => ({ ...s, email: e.target.value }))}
          disabled={state.submitting}
          className="w-full rounded-md border border-border bg-page-bg px-3 py-2 text-sm text-fg placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:opacity-50"
          placeholder="you@example.com"
        />
      </div>

      <div>
        <label
          htmlFor="password"
          className="block text-sm font-medium text-fg mb-1"
        >
          Password
        </label>
        <input
          ref={passwordRef}
          id="password"
          type="password"
          autoComplete="current-password"
          required
          value={state.password}
          onChange={(e) =>
            setState((s) => ({ ...s, password: e.target.value }))
          }
          disabled={state.submitting}
          className="w-full rounded-md border border-border bg-page-bg px-3 py-2 text-sm text-fg placeholder-fg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:opacity-50"
          placeholder="••••••••"
        />
      </div>

      {state.error && (
        <p role="alert" className="text-sm text-danger">
          {state.error}
        </p>
      )}

      <button
        type="submit"
        disabled={state.submitting}
        className="w-full rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 active:scale-[0.98] disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        {state.submitting ? "Signing in…" : "Sign in"}
      </button>
    </form>
  );
}
