import { customRender, screen, waitFor, fireEvent } from "@/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";

import TariffSettingsSection from "./TariffSettingsSection";

const tariff = {
  baseRatePence: 24.83,
  offPeakWindows: [{ start: "00:30", end: "04:30", ratePence: 7 }],
};

function jsonResponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
    text: async () => JSON.stringify(body),
  };
}

describe("TariffSettingsSection", () => {
  const mockFetch = vi.fn();

  beforeEach(() => {
    vi.stubGlobal("fetch", mockFetch);
    mockFetch.mockReset();
    mockFetch.mockResolvedValue(jsonResponse(tariff));
  });

  it("renders the loaded base rate and off-peak window", async () => {
    customRender(<TariffSettingsSection />);

    await waitFor(() => {
      expect(screen.getByLabelText(/base rate/i)).toHaveValue(24.83);
    });
    expect(screen.getByLabelText("Off-peak window 1 start")).toHaveValue(
      "00:30",
    );
    expect(screen.getByLabelText("Off-peak window 1 rate")).toHaveValue(7);
  });

  it("adds a window (saves immediately) then updates rate on blur to save again", async () => {
    customRender(<TariffSettingsSection />);
    await waitFor(() => {
      expect(screen.getByLabelText(/base rate/i)).toHaveValue(24.83);
    });

    // Adding a window triggers an immediate auto-save with the default rate (= base rate).
    mockFetch.mockResolvedValue(jsonResponse(tariff));
    fireEvent.click(
      screen.getByRole("button", { name: /add off-peak window/i }),
    );

    await waitFor(() => {
      const putCall = mockFetch.mock.calls.find(
        ([, init]) => init?.method === "PUT",
      );
      if (!putCall) throw new Error("no PUT on add");
      const body = JSON.parse((putCall[1] as RequestInit).body as string);
      expect(body.offPeakWindows).toHaveLength(2);
      // New window defaults to base rate
      expect(body.offPeakWindows[1].ratePence).toBe(24.83);
    });

    // Changing + blurring the rate field triggers another auto-save.
    mockFetch.mockReset();
    mockFetch.mockResolvedValue(jsonResponse(tariff));
    fireEvent.change(screen.getByLabelText("Off-peak window 2 rate"), {
      target: { value: "9" },
    });
    fireEvent.blur(screen.getByLabelText("Off-peak window 2 rate"));

    await waitFor(() => {
      const putCall = mockFetch.mock.calls.find(
        ([, init]) => init?.method === "PUT",
      );
      if (!putCall) throw new Error("no PUT on blur");
      const body = JSON.parse((putCall[1] as RequestInit).body as string);
      expect(body.offPeakWindows[1].ratePence).toBe(9);
    });
  });

  it("blocks saving when a window rate is negative (validated on blur)", async () => {
    customRender(<TariffSettingsSection />);
    await waitFor(() => {
      expect(screen.getByLabelText(/base rate/i)).toHaveValue(24.83);
    });

    fireEvent.change(screen.getByLabelText("Off-peak window 1 rate"), {
      target: { value: "-3" },
    });
    fireEvent.blur(screen.getByLabelText("Off-peak window 1 rate"));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/rate must be/i);
    });
    expect(
      mockFetch.mock.calls.some(([, init]) => init?.method === "PUT"),
    ).toBe(false);
  });
});
