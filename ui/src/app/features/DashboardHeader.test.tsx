import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

import DashboardHeader from "./DashboardHeader";

const mockPush = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
}));

global.fetch = vi.fn();

beforeEach(() => {
  vi.clearAllMocks();
  (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({ ok: true });
});

describe("DashboardHeader", () => {
  it("renders page title", () => {
    render(<DashboardHeader onOpenSettings={vi.fn()} />);
    expect(screen.getByText("EV Charge Controller")).toBeInTheDocument();
  });

  it("renders history link with correct href and label", () => {
    render(<DashboardHeader onOpenSettings={vi.fn()} />);
    const link = screen.getByRole("link", { name: /View charge history/i });
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute("href", "/history");
  });

  it("renders settings button with correct label", () => {
    render(<DashboardHeader onOpenSettings={vi.fn()} />);
    const button = screen.getByRole("button", { name: /Open settings/i });
    expect(button).toBeInTheDocument();
  });

  it("calls onOpenSettings when settings button is clicked", () => {
    const onOpenSettings = vi.fn();
    render(<DashboardHeader onOpenSettings={onOpenSettings} />);
    const button = screen.getByRole("button", { name: /Open settings/i });
    fireEvent.click(button);
    expect(onOpenSettings).toHaveBeenCalledTimes(1);
  });

  it("renders logout button", () => {
    render(<DashboardHeader onOpenSettings={vi.fn()} />);
    expect(
      screen.getByRole("button", { name: /Log out/i }),
    ).toBeInTheDocument();
  });

  it("calls logout endpoint and redirects on logout click", async () => {
    render(<DashboardHeader onOpenSettings={vi.fn()} />);
    const button = screen.getByRole("button", { name: /Log out/i });
    fireEvent.click(button);
    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith("/api/auth/logout", {
        method: "POST",
      });
      expect(mockPush).toHaveBeenCalledWith("/login");
    });
  });
});
