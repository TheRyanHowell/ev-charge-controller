import AddPlugModal from "@/components/AddPlugModal";
import {
  customRender as render,
  screen,
  waitFor,
  fireEvent,
} from "@/test-utils";
import { createVehicleModel } from "@/test/fixtures";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";

const mockFetch = vi.fn();
global.fetch = mockFetch;

describe("AddPlugModal", () => {
  const onClose = vi.fn();
  const onPlugCreated = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/vehicle-models")) {
        return Promise.resolve({
          status: 200,
          ok: true,
          json: async () => [
            createVehicleModel({ id: "m1", name: "Maeving RM1" }),
            createVehicleModel({
              id: "m2",
              name: "Maeving RM2",
              capacityKwh: 5.46,
            }),
          ],
        });
      }
      if (url.includes("/api/vehicles")) {
        return Promise.resolve({
          status: 200,
          ok: true,
          json: async () => [],
        });
      }
      return Promise.resolve({ ok: false });
    });
  });

  it("dialog is closed when isOpen is false", async () => {
    render(
      <AddPlugModal
        isOpen={false}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    const dialog = document.querySelector("dialog");
    expect(dialog).not.toBeNull();
    if (dialog) {
      expect(dialog.open).toBe(false);
    }
  });

  it("dialog opens and shows path-select when isOpen is true", async () => {
    render(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Add vehicle")).toBeInTheDocument();
      expect(
        screen.getByText("How do you want to set up this plug?"),
      ).toBeInTheDocument();
    });
  });

  it("shows auto-config form when auto-configure button is clicked", async () => {
    const user = userEvent.setup();
    render(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    await user.click(screen.getByText("Auto-configure"));

    await waitFor(() => {
      expect(screen.getByText("Plug name *")).toBeInTheDocument();
      expect(screen.getByText("Plug IP address *")).toBeInTheDocument();
      expect(screen.getByText("Vehicle *")).toBeInTheDocument();
    });
  });

  it("shows manual form when manual button is clicked", async () => {
    const user = userEvent.setup();
    render(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    await user.click(screen.getByText("Manual"));

    await waitFor(() => {
      expect(screen.getByText("Plug name *")).toBeInTheDocument();
      expect(screen.getByText("Vehicle *")).toBeInTheDocument();
      expect(screen.queryByText("Plug IP address *")).not.toBeInTheDocument();
    });
  });

  it("back button returns to path-select from auto-config", async () => {
    const user = userEvent.setup();
    render(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    await user.click(screen.getByText("Auto-configure"));
    await waitFor(() => {
      expect(screen.getByText("Plug IP address *")).toBeInTheDocument();
    });

    await user.click(screen.getByText("Back"));
    await waitFor(() => {
      expect(
        screen.getByText("How do you want to set up this plug?"),
      ).toBeInTheDocument();
    });
  });

  it("back button returns to path-select from manual", async () => {
    const user = userEvent.setup();
    render(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    await user.click(screen.getByText("Manual"));
    await waitFor(() => {
      expect(screen.getByText(/Generate credentials/)).toBeInTheDocument();
    });

    await user.click(screen.getByText("Back"));
    await waitFor(() => {
      expect(
        screen.getByText("How do you want to set up this plug?"),
      ).toBeInTheDocument();
    });
  });

  it("cancel button closes modal and calls onClose", async () => {
    const user = userEvent.setup();
    render(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    await user.click(screen.getByText("Cancel"));
    expect(onClose).toHaveBeenCalled();
  });

  it("re-opening modal after close resets to path-select", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    await user.click(screen.getByText("Auto-configure"));
    await waitFor(() => {
      expect(screen.getByText("Plug IP address *")).toBeInTheDocument();
    });

    await user.click(screen.getByText("Back"));
    await waitFor(() => {
      expect(
        screen.getByText("How do you want to set up this plug?"),
      ).toBeInTheDocument();
    });

    await user.click(screen.getByText("Cancel"));

    rerender(
      <AddPlugModal
        isOpen={false}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    rerender(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByText("How do you want to set up this plug?"),
      ).toBeInTheDocument();
    });
  });

  it("mode stays at auto-config when isOpen stays true during re-render", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    await user.click(screen.getByText("Auto-configure"));
    await waitFor(() => {
      expect(screen.getByText("Plug IP address *")).toBeInTheDocument();
    });

    rerender(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    expect(screen.getByText("Plug IP address *")).toBeInTheDocument();
    expect(
      screen.queryByText("How do you want to set up this plug?"),
    ).not.toBeInTheDocument();
  });

  it("auto-config form fetches vehicle models", async () => {
    const user = userEvent.setup();
    render(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    await user.click(screen.getByText("Auto-configure"));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/vehicle-models"),
        expect.anything(),
      );
    });
  });

  it("auto-config configure button is disabled until required fields filled", async () => {
    const user = userEvent.setup();
    render(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    await user.click(screen.getByText("Auto-configure"));

    const configureBtn = screen.getByText("Configure →");
    expect(configureBtn).toBeDisabled();
  });

  it("manual generate button is disabled until required fields filled", async () => {
    const user = userEvent.setup();
    render(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    await user.click(screen.getByText("Manual"));
    await waitFor(() => {
      expect(screen.getByText(/Generate credentials/)).toBeInTheDocument();
    });

    const genBtn = screen.getByText(/Generate credentials/);
    expect(genBtn).toBeDisabled();
  });

  it("auto-config success flow creates plug and calls onPlugCreated", async () => {
    const mockPlug = {
      id: "plug-1",
      userId: "user-1",
      name: "Driveway",
      namespace: "tasmota",
      mqttTopic: "tasmota123",
      tls: false,
      online: true,
      createdAt: "2024-01-01T00:00:00Z",
      vehicleId: "veh-1",
    };

    mockFetch.mockImplementation((url: string, init?: RequestInit) =>
      Promise.resolve({
        status:
          (url.includes("/api/vehicles") || url.includes("/api/plugs")) &&
          init?.method === "POST"
            ? 201
            : 200,
        ok: true,
        json: async () => {
          if (url.includes("/api/vehicle-models"))
            return [
              { id: "m1", name: "Maeving RM1", capacityKwh: 2.026 },
              { id: "m2", name: "Maeving RM2", capacityKwh: 5.46 },
            ];
          if (url.includes("/api/vehicles") && !init) return [];
          if (url.includes("/configure")) return { consoleCommands: "" };
          if (url.includes("/api/vehicles") && init?.method === "POST")
            return {
              id: "veh-1",
              modelId: "m1",
              name: "My EV",
              capacityKwh: 2.026,
              chargerOutputW: 1440,
              chargingEfficiency: 0.95,
              rangeMinMi: 0,
              rangeMaxMi: 10,
            };
          if (url.includes("/api/plugs") && init?.method === "POST")
            return { plug: mockPlug };
          return {};
        },
      }),
    );

    const user = userEvent.setup();
    render(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    await user.click(screen.getByText("Auto-configure"));
    await waitFor(() =>
      expect(screen.getByText("Plug IP address *")).toBeInTheDocument(),
    );

    const nameInput = screen.getByPlaceholderText("Driveway");
    const ipInput = screen.getByPlaceholderText("192.168.1.50");
    const modelSelect = screen.getByLabelText(/Vehicle/);

    Object.defineProperty(nameInput, "value", {
      value: "Driveway",
      writable: true,
    });
    fireEvent.input(nameInput);
    Object.defineProperty(ipInput, "value", {
      value: "192.168.1.50",
      writable: true,
    });
    fireEvent.input(ipInput);
    Object.defineProperty(modelSelect, "value", {
      value: "model:m1",
      writable: true,
    });
    fireEvent.change(modelSelect);

    await waitFor(() =>
      expect(screen.getByText("Configure →")).not.toBeDisabled(),
    );

    await user.click(screen.getByText("Configure →"));

    await waitFor(() => expect(onPlugCreated).toHaveBeenCalled(), {
      timeout: 5000,
    });
  });

  it("auto-config shows error message on API failure", async () => {
    const errorBody = JSON.stringify({
      type: "about:blank",
      title: "Internal Server Error",
      status: 500,
      detail: "Database error",
    });

    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (url.includes("/api/vehicle-models")) {
        return Promise.resolve({
          status: 200,
          ok: true,
          json: async () => [
            { id: "m1", name: "Maeving RM1", capacityKwh: 2.026 },
          ],
        });
      }
      if (url.includes("/api/vehicles") && !init)
        return Promise.resolve({
          status: 200,
          ok: true,
          json: async () => [],
        });
      if (url.includes("/api/vehicles") && init?.method === "POST") {
        return Promise.resolve({
          status: 201,
          ok: true,
          json: async () => ({
            id: "veh-1",
            modelId: "m1",
            name: "My EV",
            capacityKwh: 2.026,
            chargerOutputW: 1440,
            chargingEfficiency: 0.95,
            rangeMinMi: 0,
            rangeMaxMi: 10,
          }),
        });
      }
      if (url.includes("/api/plugs") && init?.method === "POST") {
        return Promise.resolve({
          status: 500,
          ok: false,
          text: async () => errorBody,
          json: async () => JSON.parse(errorBody),
        });
      }
      return Promise.resolve({ ok: false, status: 404 });
    });

    const user = userEvent.setup();
    render(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    await user.click(screen.getByText("Auto-configure"));
    await waitFor(() => {
      expect(screen.getByText("Plug IP address *")).toBeInTheDocument();
    });

    const nameInput = screen.getByPlaceholderText("Driveway");
    const ipInput = screen.getByPlaceholderText("192.168.1.50");
    const modelSelect = screen.getByLabelText(/Vehicle/);

    Object.defineProperty(nameInput, "value", {
      value: "Driveway",
      writable: true,
    });
    fireEvent.input(nameInput);
    Object.defineProperty(ipInput, "value", {
      value: "192.168.1.50",
      writable: true,
    });
    fireEvent.input(ipInput);
    Object.defineProperty(modelSelect, "value", {
      value: "model:m1",
      writable: true,
    });
    fireEvent.change(modelSelect);

    await user.click(screen.getByText("Configure →"));
    await waitFor(() => {
      expect(screen.getByText("Database error")).toBeInTheDocument();
    });
  });

  it("manual success flow creates plug and shows console commands", async () => {
    const mockPlug = {
      id: "plug-2",
      userId: "user-1",
      name: "Garage",
      namespace: "tasmota",
      mqttTopic: "tasmota456",
      tls: false,
      online: true,
      createdAt: "2024-01-01T00:00:00Z",
      vehicleId: "veh-2",
    };

    mockFetch.mockImplementation((url: string, init?: RequestInit) =>
      Promise.resolve({
        status:
          (url.includes("/api/vehicles") || url.includes("/api/plugs")) &&
          init?.method === "POST"
            ? 201
            : 200,
        ok: true,
        json: async () => {
          if (url.includes("/api/vehicle-models"))
            return [{ id: "m1", name: "Maeving RM1", capacityKwh: 2.026 }];
          if (url.includes("/api/vehicles") && !init) return [];
          if (url.includes("/configure"))
            return { consoleCommands: "Backlog MqttHost 1.2.3.4" };
          if (url.includes("/api/vehicles") && init?.method === "POST")
            return {
              id: "veh-2",
              modelId: "m1",
              name: "My EV",
              capacityKwh: 2.026,
              chargerOutputW: 1440,
              chargingEfficiency: 0.95,
              rangeMinMi: 0,
              rangeMaxMi: 10,
            };
          if (url.includes("/api/plugs") && init?.method === "POST")
            return { plug: mockPlug };
          return {};
        },
      }),
    );

    const user = userEvent.setup();
    render(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    await user.click(screen.getByText("Manual"));
    await waitFor(() =>
      expect(screen.getByText(/Generate credentials/)).toBeInTheDocument(),
    );

    const nameInput = screen.getByPlaceholderText("Driveway");
    const modelSelect = screen.getByLabelText(/Vehicle/);

    Object.defineProperty(nameInput, "value", {
      value: "Garage",
      writable: true,
    });
    fireEvent.input(nameInput);
    Object.defineProperty(modelSelect, "value", {
      value: "model:m1",
      writable: true,
    });
    fireEvent.change(modelSelect);

    await waitFor(() =>
      expect(screen.getByText(/Generate credentials/)).not.toBeDisabled(),
    );

    await user.click(screen.getByText(/Generate credentials/));

    await waitFor(
      () =>
        expect(
          screen.getByText(/Open the Tasmota console/),
        ).toBeInTheDocument(),
      {
        timeout: 5000,
      },
    );

    await user.click(screen.getByText("Done ✓"));

    await waitFor(() => expect(onPlugCreated).toHaveBeenCalled(), {
      timeout: 5000,
    });
  });

  it("manual shows error message on API failure", async () => {
    const errorBody = JSON.stringify({
      type: "about:blank",
      title: "Internal Server Error",
      status: 500,
      detail: "Failed to create vehicle",
    });

    mockFetch.mockImplementation((url: string, init?: RequestInit) => {
      if (url.includes("/api/vehicle-models")) {
        return Promise.resolve({
          status: 200,
          ok: true,
          json: async () => [
            { id: "m1", name: "Maeving RM1", capacityKwh: 2.026 },
          ],
        });
      }
      if (url.includes("/api/vehicles") && !init)
        return Promise.resolve({
          status: 200,
          ok: true,
          json: async () => [],
        });
      if (url.includes("/api/vehicles") && init?.method === "POST") {
        return Promise.resolve({
          status: 500,
          ok: false,
          text: async () => errorBody,
          json: async () => JSON.parse(errorBody),
        });
      }
      return Promise.resolve({ ok: false, status: 404 });
    });

    const user = userEvent.setup();
    render(
      <AddPlugModal
        isOpen={true}
        onClose={onClose}
        onPlugCreated={onPlugCreated}
      />,
    );

    await user.click(screen.getByText("Manual"));
    await waitFor(() => {
      expect(screen.getByText(/Generate credentials/)).toBeInTheDocument();
    });

    const nameInput = screen.getByPlaceholderText("Driveway");
    const modelSelect = screen.getByLabelText(/Vehicle/);

    Object.defineProperty(nameInput, "value", {
      value: "Garage",
      writable: true,
    });
    fireEvent.input(nameInput);
    Object.defineProperty(modelSelect, "value", {
      value: "model:m1",
      writable: true,
    });
    fireEvent.change(modelSelect);

    await waitFor(() => {
      const btn = screen.getByText(/Generate credentials/);
      expect(btn).not.toBeDisabled();
    });

    await user.click(screen.getByText(/Generate credentials/));

    await waitFor(
      () => {
        expect(
          screen.getByText("Failed to create vehicle"),
        ).toBeInTheDocument();
      },
      { timeout: 5000 },
    );
  });

  describe("12V mode (existingVehicleId set)", () => {
    it("shows 'Cancel' instead of 'Skip' in 12V offer when opened directly from Settings", async () => {
      render(
        <AddPlugModal
          isOpen={true}
          onClose={onClose}
          onPlugCreated={onPlugCreated}
          existingVehicleId="vehicle-123"
        />,
      );

      await waitFor(() => {
        expect(
          screen.getByRole("heading", { name: /add 12V maintenance charger/i }),
        ).toBeInTheDocument();
      });

      expect(
        screen.getByRole("button", { name: /cancel/i }),
        "Direct 12V add flow must show Cancel, not Skip",
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: /^skip$/i }),
      ).not.toBeInTheDocument();
    });

    it("skips directly to 12V offer and does not show vehicle selector", async () => {
      render(
        <AddPlugModal
          isOpen={true}
          onClose={onClose}
          onPlugCreated={onPlugCreated}
          existingVehicleId="vehicle-123"
        />,
      );

      await waitFor(() => {
        expect(
          screen.getByRole("heading", { name: /add 12V maintenance charger/i }),
        ).toBeInTheDocument();
      });

      // Vehicle selector must not appear - vehicle is already locked by existingVehicleId
      expect(screen.queryByLabelText(/vehicle/i)).not.toBeInTheDocument();
      // Auto-configure and Manual path options must be visible
      expect(
        screen.getByRole("button", { name: /auto-configure/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /manual/i }),
      ).toBeInTheDocument();
    });
  });
});
