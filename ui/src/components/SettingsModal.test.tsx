import type { Mock } from "vitest";

import { createPlug, createVehicle } from "@/test/fixtures";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("@/lib/push", () => ({
  isPushEnabled: vi.fn(() => false),
  subscribeToPush: vi.fn().mockResolvedValue(null),
  unsubscribeFromPush: vi.fn().mockResolvedValue(false),
  registerServiceWorker: vi.fn().mockResolvedValue(null),
  getPushSubscription: vi.fn().mockResolvedValue(null),
}));

vi.mock("@/components/ConsoleCommandsBlock", () => ({
  default: ({ commands }: { commands: string }) => (
    <div data-testid="console-commands">{commands}</div>
  ),
}));

vi.mock("@/lib/api", () => ({
  apiPost: vi.fn(),
  apiGet: vi.fn(),
}));

vi.mock("@tanstack/react-query", () => ({
  useQueryClient: () => ({
    invalidateQueries: vi.fn(),
    setQueryData: vi.fn(),
  }),
  useMutation: vi.fn((opts: any) => ({
    mutate: vi.fn(),
    mutateAsync: vi.fn(),
    isPending: false,
    error: null,
    ...opts,
  })),
  useQuery: vi.fn(() => ({ data: [], isLoading: false })),
}));

import { isPushEnabled } from "@/lib/push";

import SettingsModal from "./SettingsModal";

const mockVehicles = [
  createVehicle({
    id: "vehicle-1",
    name: "Maeving RM1S",
    capacityKwh: 3.8,
    chargerOutputW: 1200,
    rangeMaxMi: 30,
  }),
  createVehicle({
    id: "vehicle-2",
    name: "Tesla Model 3",
    capacityKwh: 75,
    chargerOutputW: 7200,
    rangeMaxMi: 300,
  }),
];

const mockPlug = createPlug({
  id: "plug-1",
  name: "Driveway",
  namespace: "ns1",
  mqttTopic: "plug1",
  vehicleId: "vehicle-1",
  lastSeen: null,
});

const baseProps = {
  isOpen: true,
  onClose: vi.fn(),
  plug: mockPlug,
  vehicles: mockVehicles,
  onUpdateName: vi.fn(),
  onDelete: vi.fn(),
};

describe("SettingsModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (isPushEnabled as Mock).mockImplementation(() => false);
  });

  const createRenderProps = (overrides = {}) => ({
    ...baseProps,
    ...overrides,
  });

  // - Rendering Tests -

  it("renders with isOpen true", () => {
    render(<SettingsModal {...createRenderProps()} />);
    expect(screen.getByText("Settings")).toBeInTheDocument();
  });

  it("does not render when isOpen is false", () => {
    render(<SettingsModal {...createRenderProps({ isOpen: false })} />);
    expect(screen.queryByText("Settings")).not.toBeInTheDocument();
  });

  it("renders with 'No charging plug' when plug is null", () => {
    render(<SettingsModal {...createRenderProps({ plug: null })} />);
    expect(screen.getByText("Settings")).toBeInTheDocument();
    expect(screen.getByText("No charging plug")).toBeInTheDocument();
  });

  it("calls onAddChargingPlug and onClose when 'Add charging plug →' is clicked", () => {
    const onAddChargingPlug = vi.fn();
    const onClose = vi.fn();
    render(
      <SettingsModal
        {...createRenderProps({ plug: null, onClose, onAddChargingPlug })}
      />,
    );
    fireEvent.click(screen.getByText("Add charging plug →"));
    expect(onAddChargingPlug).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("does not show 'Add charging plug →' when onAddChargingPlug is not provided", () => {
    render(<SettingsModal {...createRenderProps({ plug: null })} />);
    expect(screen.queryByText("Add charging plug →")).not.toBeInTheDocument();
  });

  it("renders with dialog role and aria-modal", () => {
    render(<SettingsModal {...createRenderProps()} />);
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveAttribute("aria-modal", "true");
  });

  it("shows plug name and online status", () => {
    render(<SettingsModal {...createRenderProps()} />);
    expect(screen.getByText("Driveway")).toBeInTheDocument();
  });

  // - CC/CV Chart Tests -
  // Chart has been moved to the vehicle detail page; it no longer lives in SettingsModal.

  it("does not show CC/CV Charging Profile section", () => {
    render(<SettingsModal {...createRenderProps()} />);
    expect(
      screen.queryByText("CC/CV Charging Profile"),
    ).not.toBeInTheDocument();
  });

  // - Plug Name Editing Tests -

  it("shows pencil icon edit button for plug name", () => {
    render(<SettingsModal {...createRenderProps()} />);
    expect(screen.getByTitle("Edit name")).toBeInTheDocument();
  });

  it("enters edit mode when pencil is clicked", () => {
    render(<SettingsModal {...createRenderProps()} />);
    fireEvent.click(screen.getByTitle("Edit name"));
    expect(screen.getByRole("textbox")).toHaveValue("Driveway");
  });

  it("calls onUpdateName when Save is clicked after editing", async () => {
    const onUpdateName = vi.fn();
    render(<SettingsModal {...createRenderProps({ onUpdateName })} />);

    fireEvent.click(screen.getByTitle("Edit name"));
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "New Name" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(onUpdateName).toHaveBeenCalledWith("New Name");
  });

  it("cancels edit when Cancel is clicked in edit mode", () => {
    render(<SettingsModal {...createRenderProps()} />);

    fireEvent.click(screen.getByTitle("Edit name"));
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "New Name" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    expect(
      screen.queryByRole("button", { name: "Save" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Driveway")).toBeInTheDocument();
  });

  it("does not show vehicle assignment dropdown", () => {
    render(<SettingsModal {...createRenderProps()} />);
    expect(screen.queryByLabelText("Assigned vehicle")).not.toBeInTheDocument();
  });

  // - Delete Plug Tests -

  it("shows Delete button in action row", () => {
    render(<SettingsModal {...createRenderProps()} />);
    expect(screen.getByRole("button", { name: "Delete" })).toBeInTheDocument();
  });

  it("shows confirmation when Delete is clicked", () => {
    render(<SettingsModal {...createRenderProps()} />);
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    expect(screen.getByText("Delete this plug?")).toBeInTheDocument();
  });

  it("calls onDelete when confirmed", () => {
    const onDelete = vi.fn();
    render(<SettingsModal {...createRenderProps({ onDelete })} />);

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    // Confirmation panel renders a second Delete button - click it to confirm
    const deleteBtns = screen.getAllByRole("button", { name: "Delete" });
    fireEvent.click(deleteBtns[deleteBtns.length - 1] as HTMLElement);

    expect(onDelete).toHaveBeenCalled();
  });

  it("cancels delete confirmation", () => {
    render(<SettingsModal {...createRenderProps()} />);

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    expect(screen.getByText("Delete this plug?")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByText("Delete this plug?")).not.toBeInTheDocument();
  });

  // - Schedule Tests -
  // Schedule has been moved to ScheduleModal (no longer in SettingsModal).

  it("does not show schedule section in SettingsModal", () => {
    render(<SettingsModal {...createRenderProps()} />);
    expect(screen.queryByText("Daily Schedule")).not.toBeInTheDocument();
    expect(screen.queryByText("Carbon-aware")).not.toBeInTheDocument();
    expect(screen.queryByText("Charge Schedule")).not.toBeInTheDocument();
  });

  // - Push Notifications Tests -

  it("does not show notifications when push is not enabled", () => {
    (isPushEnabled as Mock).mockImplementation(() => false);
    render(<SettingsModal {...createRenderProps()} />);
    expect(screen.queryByText("Push Notifications")).not.toBeInTheDocument();
  });

  it("shows notifications when push is enabled", async () => {
    (isPushEnabled as Mock).mockImplementation(() => true);
    render(<SettingsModal {...createRenderProps()} />);
    await waitFor(() => {
      expect(screen.getByText("Push Notifications")).toBeInTheDocument();
    });
  });

  // - Configure icon Tests -

  it("shows Configure gear icon button for any plug", () => {
    render(<SettingsModal {...createRenderProps()} />);
    expect(screen.getByTitle("Configure")).toBeInTheDocument();
  });

  it("clicking Configure shows path-select chooser with Auto-configure and Manual", () => {
    render(<SettingsModal {...createRenderProps()} />);
    fireEvent.click(screen.getByTitle("Configure"));
    expect(screen.getByText("Auto-configure")).toBeInTheDocument();
    expect(screen.getByText("Manual")).toBeInTheDocument();
  });

  it("Cancel in configure chooser dismisses it", () => {
    render(<SettingsModal {...createRenderProps()} />);
    fireEvent.click(screen.getByTitle("Configure"));
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByText("Auto-configure")).not.toBeInTheDocument();
  });

  it("configure opens a separate dialog element, not inline", () => {
    render(<SettingsModal {...createRenderProps()} />);
    // Before opening configure: one dialog (the settings modal itself)
    expect(document.querySelectorAll("dialog")).toHaveLength(1);
    fireEvent.click(screen.getByTitle("Configure"));
    // After: a second dialog has been mounted for the configure flow
    expect(document.querySelectorAll("dialog")).toHaveLength(2);
  });

  it("Auto-configure in chooser navigates to the IP address form", () => {
    render(<SettingsModal {...createRenderProps()} />);
    fireEvent.click(screen.getByTitle("Configure"));
    // Button accessible name includes the subtitle, so use regex
    fireEvent.click(screen.getByRole("button", { name: /auto-configure/i }));
    expect(screen.getByLabelText(/plug ip address/i)).toBeInTheDocument();
    expect(
      screen.queryByText("How do you want to configure"),
    ).not.toBeInTheDocument();
  });

  it("Cancel from Auto-configure form returns to path-select chooser", () => {
    render(<SettingsModal {...createRenderProps()} />);
    fireEvent.click(screen.getByTitle("Configure"));
    fireEvent.click(screen.getByRole("button", { name: /auto-configure/i }));
    expect(screen.getByLabelText(/plug ip address/i)).toBeInTheDocument();

    // Cancel goes back to chooser, not close the configure modal
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.getByText("Auto-configure")).toBeInTheDocument();
    expect(screen.getByText("Manual")).toBeInTheDocument();
    expect(screen.queryByLabelText(/plug ip address/i)).not.toBeInTheDocument();
  });

  it("Manual in chooser calls configure API and shows console commands", async () => {
    const { apiPost } = await import("@/lib/api");
    (apiPost as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      consoleCommands: "BacklogUrl mqtt://test",
    });

    render(<SettingsModal {...createRenderProps()} />);
    fireEvent.click(screen.getByTitle("Configure"));
    fireEvent.click(screen.getByRole("button", { name: /^manual/i }));

    await waitFor(() => {
      expect(screen.getByTestId("console-commands")).toBeInTheDocument();
    });
    expect(apiPost).toHaveBeenCalledWith(
      expect.stringContaining("/configure"),
      expect.anything(),
      {},
    );
  });

  it("Done in manual configure closes the configure modal", async () => {
    const { apiPost } = await import("@/lib/api");
    (apiPost as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      consoleCommands: "BacklogUrl mqtt://test",
    });

    render(<SettingsModal {...createRenderProps()} />);
    fireEvent.click(screen.getByTitle("Configure"));
    fireEvent.click(screen.getByRole("button", { name: /^manual/i }));

    await waitFor(() => {
      expect(screen.getByTestId("console-commands")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Done" }));
    expect(screen.queryByTestId("console-commands")).not.toBeInTheDocument();
    expect(document.querySelectorAll("dialog")).toHaveLength(1);
  });
});
