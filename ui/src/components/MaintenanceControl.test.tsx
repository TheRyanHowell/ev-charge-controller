import { createPlug } from "@/test/fixtures";
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

import MaintenanceControl from "./MaintenanceControl";

const onlinePlug = createPlug({
  id: "m1",
  name: "12V Charger",
  type: "maintenance",
  online: true,
  powerOn: false,
});

describe("MaintenanceControl", () => {
  it("shows the plug name and online status", () => {
    render(<MaintenanceControl plug={onlinePlug} onToggle={vi.fn()} />);
    expect(screen.getByText("12V Charger")).toBeInTheDocument();
    expect(screen.getByText("Online")).toBeInTheDocument();
  });

  it("toggles power via the switch", () => {
    const onToggle = vi.fn();
    render(<MaintenanceControl plug={onlinePlug} onToggle={onToggle} />);
    fireEvent.click(
      screen.getByRole("switch", { name: /12V charger off - tap to turn on/i }),
    );
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it("disables the switch while a toggle is pending", () => {
    render(
      <MaintenanceControl plug={onlinePlug} onToggle={vi.fn()} isPending />,
    );
    expect(screen.getByRole("switch")).toBeDisabled();
  });

  it("shows offline state when the plug is offline", () => {
    render(
      <MaintenanceControl
        plug={createPlug({ ...onlinePlug, online: false })}
        onToggle={vi.fn()}
      />,
    );
    expect(screen.getByText("Offline")).toBeInTheDocument();
    expect(
      screen.getByRole("switch", { name: /12V charger offline/i }),
    ).toBeInTheDocument();
  });

  it("offers to add a 12V charger when none is configured", () => {
    const onAdd12V = vi.fn();
    render(<MaintenanceControl plug={null} onAdd12V={onAdd12V} />);
    expect(
      screen.getByText(/No 12V maintenance charger configured/i),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Add 12V charger/i }));
    expect(onAdd12V).toHaveBeenCalledTimes(1);
  });
});
