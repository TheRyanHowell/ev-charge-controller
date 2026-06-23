import type { Metadata } from "next";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { LoginForm } from "./LoginForm";

export const metadata: Metadata = {
  title: "Sign in - EV Charge Controller",
};

interface LoginPageProps {
  searchParams: Promise<{ reason?: string }>;
}

export default async function LoginPage({ searchParams }: LoginPageProps) {
  const cookieStore = await cookies();
  if (cookieStore.get("access_token")?.value) {
    redirect("/dashboard");
  }

  const { reason } = await searchParams;
  const sessionExpired = reason === "session-expired";

  return (
    <main className="min-h-screen flex items-center justify-center bg-background px-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center">
          <h1 className="text-2xl font-semibold text-foreground">
            EV Charge Controller
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Sign in to your account
          </p>
        </div>
        {sessionExpired && (
          <div
            role="status"
            className="mb-4 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-200"
          >
            Your session has expired. Please sign in again.
          </div>
        )}
        <div className="rounded-lg border border-border bg-card p-6 shadow-sm">
          <LoginForm />
        </div>
      </div>
    </main>
  );
}
