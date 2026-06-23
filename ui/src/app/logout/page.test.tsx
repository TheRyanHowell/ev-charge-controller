import { customRender, screen, waitFor } from "@/test-utils";
import { describe, it, expect, vi, beforeEach, Mock } from "vitest";

const mockPush = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
}));

import LogoutPage from "./page";

describe("Logout Page", () => {
  let mockFetch: Mock;

  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch = vi.fn();
    global.fetch = mockFetch;
  });

  it("renders logging out message", async () => {
    mockFetch.mockResolvedValue({ ok: true });
    customRender(<LogoutPage />);
    await waitFor(() =>
      expect(screen.getByText("Logging out…")).toBeInTheDocument(),
    );
  });

  it("calls logout API on mount", async () => {
    mockFetch.mockResolvedValue({ ok: true });
    customRender(<LogoutPage />);
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith("/api/auth/logout", {
        method: "POST",
      });
    });
  });

  it("redirects to login after logout", async () => {
    mockFetch.mockResolvedValue({ ok: true });
    customRender(<LogoutPage />);
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/login");
    });
  });

  it("redirects to login even on API failure", async () => {
    mockFetch.mockRejectedValue(new Error("network error"));
    customRender(<LogoutPage />);
    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/login");
    });
  });
});
