import { customRender, screen, waitFor, fireEvent, act } from "@/test-utils";
import { describe, it, expect, vi, beforeEach } from "vitest";

import FirstRunWizard from "./FirstRunWizard";

vi.mock("@/lib/api", () => ({
  apiPost: vi.fn(() => Promise.resolve({})),
  apiGet: vi.fn(() => Promise.resolve([])),
  apiGetSingle: vi.fn(() => Promise.resolve({})),
  apiPatch: vi.fn(() => Promise.resolve({})),
  apiDelete: vi.fn(() => Promise.resolve({})),
}));

vi.mock("@/lib/push", () => ({
  isPushEnabled: vi.fn(() => false),
  subscribeToPush: vi.fn(),
}));

vi.mock("@/components/VehicleSelector", () => ({
  default: function MockVehicleSelector({
    label,
    selectedVehicleId,
    onSelectVehicle,
  }: {
    label: string;
    selectedVehicleId: string | null;
    onSelectVehicle: (id: string) => void;
  }) {
    return (
      <div>
        <label>{label}</label>
        <select
          data-testid="vehicle-select"
          value={selectedVehicleId ?? ""}
          onChange={(e) => onSelectVehicle(e.target.value)}
        >
          <option value="" disabled>
            Select a vehicle…
          </option>
          <option value="v1">Existing Vehicle</option>
          <option value="model:m1">New Model</option>
        </select>
      </div>
    );
  },
  parseVehicleSelectorValue: vi.fn((v) => {
    if (!v) return undefined;
    if (v.startsWith("model:")) return { type: "model", modelId: v.slice(6) };
    return { type: "vehicle", vehicleId: v };
  }),
}));

vi.mock("@/components/ConsoleCommandsBlock", () => ({
  default: function MockConsoleCommandsBlock({
    commands,
  }: {
    commands: string;
  }) {
    return <pre data-testid="console-commands">{commands}</pre>;
  },
}));

const { apiPost, apiGet, apiPatch, apiDelete, apiGetSingle } =
  await import("@/lib/api");
const { isPushEnabled, subscribeToPush } = await import("@/lib/push");
const { parseVehicleSelectorValue } =
  await import("@/components/VehicleSelector");

describe("FirstRunWizard", () => {
  const mockOnComplete = vi.fn();

  beforeEach(() => {
    vi.resetAllMocks();
    mockOnComplete.mockResolvedValue(undefined);
    (apiPost as ReturnType<typeof vi.fn>).mockResolvedValue({});
    (apiGet as ReturnType<typeof vi.fn>).mockResolvedValue([]);
    (apiPatch as ReturnType<typeof vi.fn>).mockResolvedValue({});
    (apiDelete as ReturnType<typeof vi.fn>).mockResolvedValue({});
    (apiGetSingle as ReturnType<typeof vi.fn>).mockResolvedValue({});
    (isPushEnabled as ReturnType<typeof vi.fn>).mockReturnValue(false);
    (subscribeToPush as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    (parseVehicleSelectorValue as ReturnType<typeof vi.fn>).mockImplementation(
      (v) => {
        if (!v) return undefined;
        if (v.startsWith("model:"))
          return { type: "model", modelId: v.slice(6) };
        return { type: "vehicle", vehicleId: v };
      },
    );
  });

  describe("wifi-setup step", () => {
    it("renders the initial wifi setup step", () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      expect(
        screen.getByText("Connect your plug to Wi-Fi"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Follow these steps to join your home network."),
      ).toBeInTheDocument();
      expect(screen.getByText("Power on the plug.")).toBeInTheDocument();
    });

    it("shows all 4 wifi setup instructions", () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      expect(screen.getByText("Power on the plug.")).toBeInTheDocument();
      expect(
        screen.getByText(
          /On your phone, connect to the Tasmota Wi-Fi hotspot/i,
        ),
      ).toBeInTheDocument();
      expect(
        screen.getByText(/Open a browser and go to 192\.168\.4\.1/i),
      ).toBeInTheDocument();
      expect(
        screen.getByText(/Save and reconnect your phone to your home Wi-Fi/i),
      ).toBeInTheDocument();
    });

    it("navigates to path-select on next button click", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      expect(
        screen.getByText("How do you want to configure MQTT?"),
      ).toBeInTheDocument();
    });

    it("does not show back button on first step", () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      expect(screen.queryByText("← Back")).not.toBeInTheDocument();
    });
  });

  describe("path-select step", () => {
    it("shows auto-configure and manual MQTT options", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      expect(screen.getByText("Auto-configure")).toBeInTheDocument();
      expect(screen.getByText("Manual MQTT")).toBeInTheDocument();
    });

    it("navigates to auto-config on auto button click", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Auto-configure"));
      });
      expect(screen.getByText("Auto-configure")).toBeInTheDocument();
      expect(
        screen.getByText(/We'll push MQTT settings directly to the plug/i),
      ).toBeInTheDocument();
    });

    it("navigates to manual-mqtt on manual button click", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Manual MQTT"));
      });
      expect(screen.getByText("Manual MQTT setup")).toBeInTheDocument();
    });

    it("navigates back to wifi-setup on back button", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("← Back"));
      });
      expect(
        screen.getByText("Connect your plug to Wi-Fi"),
      ).toBeInTheDocument();
    });
  });

  describe("auto-config step", () => {
    beforeEach(() => {
      (apiGet as ReturnType<typeof vi.fn>).mockResolvedValue([]);
    });

    it("shows form fields for plug name, IP, and vehicle", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Auto-configure"));
      });
      expect(screen.getByText("Plug name *")).toBeInTheDocument();
      expect(screen.getByText("Plug IP address *")).toBeInTheDocument();
      expect(screen.getByText("Vehicle *")).toBeInTheDocument();
    });

    it("disables configure button when fields are empty", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Auto-configure"));
      });
      const button = screen.getByText("Configure →");
      expect(button).toBeDisabled();
    });

    it("enables configure button when required fields are filled", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Auto-configure"));
      });
      await act(async () => {
        fireEvent.change(screen.getByPlaceholderText("Driveway"), {
          target: { value: "My Plug" },
        });
        fireEvent.change(screen.getByPlaceholderText("192.168.1.50"), {
          target: { value: "192.168.1.50" },
        });
        fireEvent.change(screen.getByTestId("vehicle-select"), {
          target: { value: "v1" },
        });
      });
      const button = screen.getByText("Configure →");
      expect(button).toBeEnabled();
    });

    it("shows loading state during configuration", async () => {
      (apiPost as ReturnType<typeof vi.fn>).mockImplementation(
        () => new Promise(() => {}),
      );
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Auto-configure"));
      });
      await act(async () => {
        fireEvent.change(screen.getByPlaceholderText("Driveway"), {
          target: { value: "My Plug" },
        });
        fireEvent.change(screen.getByPlaceholderText("192.168.1.50"), {
          target: { value: "192.168.1.50" },
        });
        fireEvent.change(screen.getByTestId("vehicle-select"), {
          target: { value: "v1" },
        });
        fireEvent.click(screen.getByText("Configure →"));
      });
      await waitFor(() => {
        expect(screen.getByText("Configuring…")).toBeInTheDocument();
      });
    });

    it("shows error message when configuration fails", async () => {
      (apiPost as ReturnType<typeof vi.fn>).mockRejectedValue(
        new Error("Connection refused"),
      );
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Auto-configure"));
      });
      await act(async () => {
        fireEvent.change(screen.getByPlaceholderText("Driveway"), {
          target: { value: "My Plug" },
        });
        fireEvent.change(screen.getByPlaceholderText("192.168.1.50"), {
          target: { value: "192.168.1.50" },
        });
        fireEvent.change(screen.getByTestId("vehicle-select"), {
          target: { value: "v1" },
        });
        fireEvent.click(screen.getByText("Configure →"));
      });
      await waitFor(() => {
        expect(screen.getByText("Connection refused")).toBeInTheDocument();
      });
    });

    it("successfully configures and navigates to waiting step", async () => {
      (apiPost as ReturnType<typeof vi.fn>).mockResolvedValue({
        plug: { id: "plug-1" },
      });
      (apiGetSingle as ReturnType<typeof vi.fn>).mockResolvedValue({
        id: "plug-1",
        online: false,
      });
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Auto-configure"));
      });
      await act(async () => {
        fireEvent.change(screen.getByPlaceholderText("Driveway"), {
          target: { value: "My Plug" },
        });
        fireEvent.change(screen.getByPlaceholderText("192.168.1.50"), {
          target: { value: "192.168.1.50" },
        });
        fireEvent.change(screen.getByTestId("vehicle-select"), {
          target: { value: "v1" },
        });
        fireEvent.click(screen.getByText("Configure →"));
      });
      await waitFor(() => {
        expect(
          screen.getByText(/Waiting for plug to connect/i),
        ).toBeInTheDocument();
      });
    });

    it("navigates back to path-select on back button", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Auto-configure"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("← Back"));
      });
      expect(
        screen.getByText("How do you want to configure MQTT?"),
      ).toBeInTheDocument();
    });
  });

  describe("manual-mqtt step", () => {
    beforeEach(() => {
      (apiGet as ReturnType<typeof vi.fn>).mockResolvedValue([]);
    });

    it("shows form fields for plug name and vehicle", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Manual MQTT"));
      });
      expect(screen.getByText("Plug name *")).toBeInTheDocument();
      expect(screen.getByText("Vehicle *")).toBeInTheDocument();
    });

    it("disables generate button when fields are empty", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Manual MQTT"));
      });
      const button = screen.getByText("Generate credentials →");
      expect(button).toBeDisabled();
    });

    it("generates credentials and shows console commands", async () => {
      (apiPost as ReturnType<typeof vi.fn>).mockImplementation((url) => {
        if (url === "/api/plugs") {
          return Promise.resolve({ plug: { id: "plug-1" } });
        }
        return Promise.resolve({ consoleCommands: "MqttIp 1.2.3.4" });
      });
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Manual MQTT"));
      });
      await act(async () => {
        fireEvent.change(screen.getByPlaceholderText("Driveway"), {
          target: { value: "My Plug" },
        });
        fireEvent.change(screen.getByTestId("vehicle-select"), {
          target: { value: "v1" },
        });
        fireEvent.click(screen.getByText("Generate credentials →"));
      });
      await waitFor(() => {
        expect(screen.getByTestId("console-commands")).toBeInTheDocument();
        expect(
          screen.getByText("Paste into Tasmota console"),
        ).toBeInTheDocument();
      });
    });

    it("shows error message when credential generation fails", async () => {
      (apiPost as ReturnType<typeof vi.fn>).mockRejectedValue(
        new Error("Server error"),
      );
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Manual MQTT"));
      });
      await act(async () => {
        fireEvent.change(screen.getByPlaceholderText("Driveway"), {
          target: { value: "My Plug" },
        });
        fireEvent.change(screen.getByTestId("vehicle-select"), {
          target: { value: "v1" },
        });
        fireEvent.click(screen.getByText("Generate credentials →"));
      });
      await waitFor(() => {
        expect(screen.getByText("Server error")).toBeInTheDocument();
      });
    });

    it("navigates to waiting step after pasting confirmation", async () => {
      (apiPost as ReturnType<typeof vi.fn>).mockImplementation((url) => {
        if (url === "/api/plugs") {
          return Promise.resolve({ plug: { id: "plug-1" } });
        }
        return Promise.resolve({ consoleCommands: "MqttIp 1.2.3.4" });
      });
      (apiGetSingle as ReturnType<typeof vi.fn>).mockResolvedValue({
        id: "plug-1",
        online: true,
      });
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Manual MQTT"));
      });
      await act(async () => {
        fireEvent.change(screen.getByPlaceholderText("Driveway"), {
          target: { value: "My Plug" },
        });
        fireEvent.change(screen.getByTestId("vehicle-select"), {
          target: { value: "v1" },
        });
        fireEvent.click(screen.getByText("Generate credentials →"));
      });
      await waitFor(() => {
        expect(screen.getByTestId("console-commands")).toBeInTheDocument();
      });
      await act(async () => {
        fireEvent.click(screen.getByText("I've pasted the command →"));
      });
      await waitFor(() => {
        expect(
          screen.getByText(/Waiting for plug to connect/i),
        ).toBeInTheDocument();
      });
    });
  });

  describe("waiting step", () => {
    beforeEach(() => {
      (apiGet as ReturnType<typeof vi.fn>).mockResolvedValue([]);
      (apiPost as ReturnType<typeof vi.fn>).mockResolvedValue({
        plug: { id: "plug-1" },
      });
    });

    it("shows spinner and waiting message", async () => {
      (apiGetSingle as ReturnType<typeof vi.fn>).mockResolvedValue({
        id: "plug-1",
        online: false,
      });
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Auto-configure"));
      });
      await act(async () => {
        fireEvent.change(screen.getByPlaceholderText("Driveway"), {
          target: { value: "My Plug" },
        });
        fireEvent.change(screen.getByPlaceholderText("192.168.1.50"), {
          target: { value: "192.168.1.50" },
        });
        fireEvent.change(screen.getByTestId("vehicle-select"), {
          target: { value: "v1" },
        });
        fireEvent.click(screen.getByText("Configure →"));
      });
      await waitFor(() => {
        expect(
          screen.getByText(/Waiting for plug to connect/i),
        ).toBeInTheDocument();
      });
      // Spinner is a div with animate-spin class
      expect(document.querySelector(".animate-spin")).toBeInTheDocument();
    });

    it("navigates to 12v-offer step when plug comes online", async () => {
      (apiGetSingle as ReturnType<typeof vi.fn>).mockResolvedValue({
        id: "plug-1",
        online: true,
      });
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await act(async () => {
        fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Auto-configure"));
      });
      await act(async () => {
        fireEvent.change(screen.getByPlaceholderText("Driveway"), {
          target: { value: "My Plug" },
        });
        fireEvent.change(screen.getByPlaceholderText("192.168.1.50"), {
          target: { value: "192.168.1.50" },
        });
        fireEvent.change(screen.getByTestId("vehicle-select"), {
          target: { value: "v1" },
        });
        fireEvent.click(screen.getByText("Configure →"));
      });
      // After waiting step detects online, it now goes to 12v-offer (not directly to schedule).
      await waitFor(
        () => {
          expect(
            screen.getByText("Add a 12V maintenance charger?"),
          ).toBeInTheDocument();
        },
        { timeout: 10000 },
      );
    });
  });

  // Helper: navigate from wifi-setup through auto-config waiting to reach the 12v-offer step.
  async function navigateTo12vOffer() {
    await act(async () => {
      fireEvent.click(screen.getByText("My plug is on Wi-Fi →"));
    });
    await act(async () => {
      fireEvent.click(screen.getByText("Auto-configure"));
    });
    await act(async () => {
      fireEvent.change(screen.getByPlaceholderText("Driveway"), {
        target: { value: "My Plug" },
      });
      fireEvent.change(screen.getByPlaceholderText("192.168.1.50"), {
        target: { value: "192.168.1.50" },
      });
      fireEvent.change(screen.getByTestId("vehicle-select"), {
        target: { value: "v1" },
      });
      fireEvent.click(screen.getByText("Configure →"));
    });
    await waitFor(() => {
      expect(
        screen.getByText("Add a 12V maintenance charger?"),
      ).toBeInTheDocument();
    });
  }

  // Helper: navigate from wifi-setup all the way to the schedule step (skip 12V).
  async function navigateToScheduleStep() {
    await navigateTo12vOffer();
    await act(async () => {
      fireEvent.click(screen.getByText("Skip →"));
    });
    await waitFor(() => {
      expect(screen.getByText("Set a charging schedule")).toBeInTheDocument();
    });
  }

  describe("12v-offer step", () => {
    beforeEach(() => {
      (apiGet as ReturnType<typeof vi.fn>).mockResolvedValue([]);
      (apiPost as ReturnType<typeof vi.fn>).mockResolvedValue({
        plug: { id: "plug-1" },
      });
      (apiGetSingle as ReturnType<typeof vi.fn>).mockResolvedValue({
        id: "plug-1",
        online: true,
      });
    });

    it("shows offer after waiting step completes", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await navigateTo12vOffer();
      expect(
        screen.getByText("Add a 12V maintenance charger?"),
      ).toBeInTheDocument();
    });

    it("shows skip button that proceeds to schedule", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await navigateTo12vOffer();
      await act(async () => {
        fireEvent.click(screen.getByText("Skip →"));
      });
      await waitFor(() => {
        expect(screen.getByText("Set a charging schedule")).toBeInTheDocument();
      });
    });

    it("shows auto-configure option for 12V charger", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await navigateTo12vOffer();
      expect(screen.getByText("Auto-configure")).toBeInTheDocument();
      expect(screen.getByText("Manual MQTT")).toBeInTheDocument();
    });
  });

  describe("schedule step", () => {
    beforeEach(() => {
      (apiGet as ReturnType<typeof vi.fn>).mockResolvedValue([]);
      (apiPost as ReturnType<typeof vi.fn>).mockResolvedValue({
        plug: { id: "plug-1" },
      });
      (apiGetSingle as ReturnType<typeof vi.fn>).mockResolvedValue({
        id: "plug-1",
        online: true,
      });
      (apiPatch as ReturnType<typeof vi.fn>).mockResolvedValue({});
    });

    it("shows schedule toggle and time input", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await navigateToScheduleStep();
      // ScheduleForm shows "Enabled" toggle and type switcher
      expect(screen.getByText("Enabled")).toBeInTheDocument();
      expect(
        screen.getByRole("switch", { name: "Enabled" }),
      ).toBeInTheDocument();
    });

    it("toggles schedule on and off", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await navigateToScheduleStep();
      const toggle = screen.getByRole("switch", { name: "Enabled" });
      expect(toggle).toHaveAttribute("aria-checked", "false");
      await act(async () => {
        fireEvent.click(toggle);
      });
      await waitFor(() => {
        expect(toggle).toHaveAttribute("aria-checked", "true");
      });
    });

    it("navigates to notifications when skipping schedule", async () => {
      (isPushEnabled as ReturnType<typeof vi.fn>).mockReturnValue(false);
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await navigateToScheduleStep();
      // "Skip" button in ScheduleForm bypasses saving and proceeds to notifications.
      await act(async () => {
        fireEvent.click(screen.getByText("Skip"));
      });
      await waitFor(() => {
        expect(
          screen.getByText(/Push notifications aren't available/i),
        ).toBeInTheDocument();
      });
    });
  });

  describe("notifications step", () => {
    beforeEach(() => {
      (apiGet as ReturnType<typeof vi.fn>).mockResolvedValue([]);
      (apiPost as ReturnType<typeof vi.fn>).mockResolvedValue({
        plug: { id: "plug-1" },
      });
      (apiGetSingle as ReturnType<typeof vi.fn>).mockResolvedValue({
        id: "plug-1",
        online: true,
      });
      (apiPatch as ReturnType<typeof vi.fn>).mockResolvedValue({});
    });

    it("shows push not available message when isPushEnabled returns false", async () => {
      (isPushEnabled as ReturnType<typeof vi.fn>).mockReturnValue(false);
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await navigateToScheduleStep();
      await act(async () => {
        fireEvent.click(screen.getByText("Skip"));
      });
      await waitFor(() => {
        expect(
          screen.getByText(/Push notifications aren't available/i),
        ).toBeInTheDocument();
      });
    });

    it("shows enable notifications button when push is enabled", async () => {
      (isPushEnabled as ReturnType<typeof vi.fn>).mockReturnValue(true);
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await navigateToScheduleStep();
      await act(async () => {
        fireEvent.click(screen.getByText("Skip"));
      });
      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: "Enable notifications" }),
        ).toBeInTheDocument();
      });
    });

    it("navigates to done step on continue", async () => {
      (isPushEnabled as ReturnType<typeof vi.fn>).mockReturnValue(false);
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await navigateToScheduleStep();
      await act(async () => {
        fireEvent.click(screen.getByText("Skip"));
      });
      await waitFor(() => {
        expect(
          screen.getByText(/Push notifications aren't available/i),
        ).toBeInTheDocument();
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Continue →"));
      });
      await waitFor(() => {
        expect(screen.getByText("You're set up!")).toBeInTheDocument();
      });
    });
  });

  describe("done step", () => {
    beforeEach(() => {
      (apiGet as ReturnType<typeof vi.fn>).mockResolvedValue([]);
      (apiPost as ReturnType<typeof vi.fn>).mockResolvedValue({
        plug: { id: "plug-1" },
      });
      (apiGetSingle as ReturnType<typeof vi.fn>).mockResolvedValue({
        id: "plug-1",
        online: true,
      });
      (apiPatch as ReturnType<typeof vi.fn>).mockResolvedValue({});
      (isPushEnabled as ReturnType<typeof vi.fn>).mockReturnValue(false);
    });

    it("shows completion message and checkmark", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await navigateToScheduleStep();
      await act(async () => {
        fireEvent.click(screen.getByText("Skip"));
      });
      await waitFor(() => {
        expect(
          screen.getByText(/Push notifications aren't available/i),
        ).toBeInTheDocument();
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Continue →"));
      });
      await waitFor(() => {
        expect(screen.getByText("You're set up!")).toBeInTheDocument();
        expect(
          screen.getByText("The plug is online and ready to charge."),
        ).toBeInTheDocument();
      });
    });

    it("calls onComplete when dashboard button is clicked", async () => {
      customRender(<FirstRunWizard onComplete={mockOnComplete} />);
      await navigateToScheduleStep();
      await act(async () => {
        fireEvent.click(screen.getByText("Skip"));
      });
      await waitFor(() => {
        expect(
          screen.getByText(/Push notifications aren't available/i),
        ).toBeInTheDocument();
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Continue →"));
      });
      await waitFor(() => {
        expect(screen.getByText("You're set up!")).toBeInTheDocument();
      });
      await act(async () => {
        fireEvent.click(screen.getByText("Go to dashboard →"));
      });
      expect(mockOnComplete).toHaveBeenCalled();
    });
  });
});
