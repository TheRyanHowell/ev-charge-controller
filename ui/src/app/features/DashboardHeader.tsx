"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";

interface DashboardHeaderProps {
  onOpenSettings: () => void;
}

export default function DashboardHeader({
  onOpenSettings,
}: DashboardHeaderProps) {
  const router = useRouter();

  async function handleLogout() {
    await fetch("/api/auth/logout", { method: "POST" });
    router.push("/login");
  }

  return (
    <header className="flex items-center justify-between px-0 py-4 mb-3">
      <h1 className="text-lg font-semibold tracking-tight text-white">
        EV Charge Controller
      </h1>
      <div className="flex items-center gap-1">
        <Link
          href="/history"
          className="text-gray-500 hover:text-gray-200 transition-colors rounded-lg p-2 hover:bg-surface-raised"
          title="History"
          aria-label="View charge history"
        >
          <i className="fas fa-clock text-sm" aria-hidden="true"></i>
        </Link>
        <Link
          href="/vehicles"
          className="text-gray-500 hover:text-gray-200 transition-colors rounded-lg p-2 hover:bg-surface-raised"
          title="Vehicles"
          aria-label="View vehicles"
        >
          <i className="fas fa-charging-station text-sm" aria-hidden="true"></i>
        </Link>
        <button
          onClick={onOpenSettings}
          className="text-gray-500 hover:text-gray-200 transition-colors rounded-lg p-2 hover:bg-surface-raised"
          title="Settings"
          aria-label="Open settings"
        >
          <i className="fas fa-gear text-sm" aria-hidden="true"></i>
        </button>
        <button
          onClick={handleLogout}
          className="text-gray-500 hover:text-gray-200 transition-colors rounded-lg p-2 hover:bg-surface-raised"
          title="Logout"
          aria-label="Log out"
        >
          <i
            className="fas fa-arrow-right-from-bracket text-sm"
            aria-hidden="true"
          ></i>
        </button>
      </div>
    </header>
  );
}
