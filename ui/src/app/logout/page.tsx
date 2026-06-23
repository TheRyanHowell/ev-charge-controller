"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";

export default function LogoutPage() {
  const router = useRouter();

  useEffect(() => {
    fetch("/api/auth/logout", { method: "POST" })
      .catch(() => {})
      .finally(() => {
        router.push("/login");
      });
  }, [router]);

  return (
    <main className="min-h-screen flex items-center justify-center bg-background px-4">
      <p className="text-muted-foreground">Logging out…</p>
    </main>
  );
}
