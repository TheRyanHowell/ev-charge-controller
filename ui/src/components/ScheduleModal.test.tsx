import type { SchedulePayload } from "@/hooks/useSchedule";
import type { Schedule } from "@/lib/schemas";

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

import ScheduleModal from "./ScheduleModal";

// JSDOM does not implement showModal/close on <dialog>. Polyfill both so the
// dialog gets the [open] attribute (which RTL uses to determine visibility).
beforeEach(() => {
  HTMLDialogElement.prototype.showModal = function () {
    this.setAttribute("open", "");
  };
  HTMLDialogElement.prototype.close = function () {
    this.removeAttribute("open");
  };
});

const baseProps = {
  isOpen: true,
  onClose: vi.fn(),
  schedule: null as Schedule | null,
  onSave: vi.fn(),
};

describe("ScheduleModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // - Open / close -

  it("renders modal heading when open", () => {
    render(<ScheduleModal {...baseProps} />);
    expect(screen.getByText("Charge Schedule")).toBeInTheDocument();
  });

  it("does not render when isOpen is false", () => {
    render(<ScheduleModal {...baseProps} isOpen={false} />);
    expect(screen.queryByText("Charge Schedule")).not.toBeInTheDocument();
  });

  it("calls onClose when Skip is clicked", () => {
    const onClose = vi.fn();
    render(<ScheduleModal {...baseProps} onClose={onClose} />);
    // ScheduleModal passes onClose as the onSkip handler to ScheduleForm
    fireEvent.click(screen.getByRole("button", { name: "Skip" }));
    expect(onClose).toHaveBeenCalled();
  });

  it("calls onClose when X button is clicked", () => {
    const onClose = vi.fn();
    render(<ScheduleModal {...baseProps} onClose={onClose} />);
    fireEvent.click(
      screen.getByRole("button", { name: "Close schedule settings" }),
    );
    expect(onClose).toHaveBeenCalled();
  });

  // - Enable toggle -

  it("enable toggle is off by default with null schedule", () => {
    render(<ScheduleModal {...baseProps} />);
    expect(screen.getByRole("switch", { name: "Enabled" })).toHaveAttribute(
      "aria-checked",
      "false",
    );
  });

  it("enable toggle turns on when clicked", () => {
    render(<ScheduleModal {...baseProps} />);
    const toggle = screen.getByRole("switch", { name: "Enabled" });
    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute("aria-checked", "true");
  });

  // - Type switcher -

  it("shows Daily form by default", () => {
    render(<ScheduleModal {...baseProps} />);
    expect(screen.getByLabelText("Start time")).toBeInTheDocument();
    expect(screen.queryByLabelText("Earliest")).not.toBeInTheDocument();
  });

  it("switches to Carbon-aware form when Carbon-aware is clicked", () => {
    render(<ScheduleModal {...baseProps} />);
    fireEvent.click(screen.getByRole("button", { name: "Carbon-aware" }));
    expect(screen.getByLabelText("Earliest")).toBeInTheDocument();
    expect(screen.getByLabelText("Ready by")).toBeInTheDocument();
    expect(screen.queryByLabelText("Start time")).not.toBeInTheDocument();
  });

  it("switches back to Daily form from Carbon-aware", () => {
    render(<ScheduleModal {...baseProps} />);
    fireEvent.click(screen.getByRole("button", { name: "Carbon-aware" }));
    fireEvent.click(screen.getByRole("button", { name: "Daily" }));
    expect(screen.getByLabelText("Start time")).toBeInTheDocument();
    expect(screen.queryByLabelText("Earliest")).not.toBeInTheDocument();
  });

  // - Save payloads -

  it("calls onSave with daily payload", () => {
    const onSave = vi.fn();
    render(<ScheduleModal {...baseProps} onSave={onSave} />);

    fireEvent.click(screen.getByRole("switch", { name: "Enabled" })); // enable
    fireEvent.change(screen.getByLabelText("Start time"), {
      target: { value: "03:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(onSave).toHaveBeenCalledWith<[SchedulePayload]>({
      type: "daily",
      time: "03:00",
      enabled: true,
    });
  });

  it("calls onSave with carbon_aware payload", () => {
    const onSave = vi.fn();
    render(<ScheduleModal {...baseProps} onSave={onSave} />);

    fireEvent.click(screen.getByRole("button", { name: "Carbon-aware" }));
    fireEvent.click(screen.getByRole("switch", { name: "Enabled" })); // enable
    fireEvent.change(screen.getByLabelText("Earliest"), {
      target: { value: "22:00" },
    });
    fireEvent.change(screen.getByLabelText("Ready by"), {
      target: { value: "06:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(onSave).toHaveBeenCalledWith<[SchedulePayload]>({
      type: "carbon_aware",
      windowStart: "22:00",
      windowEnd: "06:00",
      twoStage: false,
      enabled: true,
    });
  });

  // - Validation -

  it("shows error when windowStart equals windowEnd", () => {
    const onSave = vi.fn();
    render(<ScheduleModal {...baseProps} onSave={onSave} />);

    fireEvent.click(screen.getByRole("button", { name: "Carbon-aware" }));
    fireEvent.change(screen.getByLabelText("Earliest"), {
      target: { value: "06:00" },
    });
    fireEvent.change(screen.getByLabelText("Ready by"), {
      target: { value: "06:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Start and ready-by times must differ",
    );
    expect(onSave).not.toHaveBeenCalled();
  });

  it("clears validation error when Earliest input changes", () => {
    render(<ScheduleModal {...baseProps} />);
    fireEvent.click(screen.getByRole("button", { name: "Carbon-aware" }));
    fireEvent.change(screen.getByLabelText("Earliest"), {
      target: { value: "06:00" },
    });
    fireEvent.change(screen.getByLabelText("Ready by"), {
      target: { value: "06:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(screen.getByRole("alert")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Earliest"), {
      target: { value: "22:00" },
    });
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("clears validation error when Ready by input changes", () => {
    render(<ScheduleModal {...baseProps} />);
    fireEvent.click(screen.getByRole("button", { name: "Carbon-aware" }));
    fireEvent.change(screen.getByLabelText("Earliest"), {
      target: { value: "06:00" },
    });
    fireEvent.change(screen.getByLabelText("Ready by"), {
      target: { value: "06:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(screen.getByRole("alert")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Ready by"), {
      target: { value: "07:00" },
    });
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  // - Saving state -

  it("disables Save button and shows Saving… while isSaving", () => {
    render(<ScheduleModal {...baseProps} isSaving={true} />);
    const saveBtn = screen.getByRole("button", { name: "Saving…" });
    expect(saveBtn).toBeDisabled();
    expect(screen.getByText("Saving…")).toBeInTheDocument();
  });

  // - Initial values from schedule prop -

  it("initializes from an existing daily schedule", () => {
    render(
      <ScheduleModal
        {...baseProps}
        schedule={{
          id: "s1",
          type: "daily",
          time: "03:00",
          enabled: true,
        }}
      />,
    );
    expect(screen.getByLabelText("Start time")).toHaveValue("03:00");
    expect(screen.getByRole("switch", { name: "Enabled" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
  });

  it("initializes from an existing carbon_aware schedule", () => {
    render(
      <ScheduleModal
        {...baseProps}
        schedule={{
          id: "s1",
          type: "carbon_aware",
          time: "22:00",
          windowStart: "22:00",
          windowEnd: "06:00",
          enabled: false,
        }}
      />,
    );
    expect(screen.getByLabelText("Earliest")).toHaveValue("22:00");
    expect(screen.getByLabelText("Ready by")).toHaveValue("06:00");
    expect(screen.getByRole("switch", { name: "Enabled" })).toHaveAttribute(
      "aria-checked",
      "false",
    );
  });

  // - External sync -

  it("syncs form state when schedule prop changes to a different object", () => {
    const { rerender } = render(
      <ScheduleModal {...baseProps} schedule={null} />,
    );

    // null schedule → daily, time defaults to 01:00
    expect(screen.getByLabelText("Start time")).toHaveValue("01:00");

    rerender(
      <ScheduleModal
        {...baseProps}
        schedule={{
          id: "s1",
          type: "carbon_aware",
          time: "22:00",
          windowStart: "22:00",
          windowEnd: "06:00",
          enabled: true,
        }}
      />,
    );

    expect(screen.getByLabelText("Earliest")).toHaveValue("22:00");
    expect(screen.getByLabelText("Ready by")).toHaveValue("06:00");
    expect(screen.getByRole("switch", { name: "Enabled" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
  });
});
