import type { Schedule } from "@/lib/schemas";

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

import ScheduleForm from "./ScheduleForm";

const baseSchedule: Schedule = {
  id: "s1",
  type: "daily",
  time: "06:00",
  enabled: true,
};

describe("ScheduleForm", () => {
  it("renders enable toggle", () => {
    render(<ScheduleForm schedule={null} onSave={vi.fn()} />);
    expect(screen.getByRole("switch", { name: "Enabled" })).toBeInTheDocument();
    expect(screen.getByText("Enabled")).toBeInTheDocument();
  });

  it("reflects schedule.enabled in toggle state", () => {
    render(<ScheduleForm schedule={baseSchedule} onSave={vi.fn()} />);
    expect(screen.getByRole("switch", { name: "Enabled" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
  });

  it("shows Daily / Carbon-aware type buttons", () => {
    render(<ScheduleForm schedule={null} onSave={vi.fn()} />);
    expect(screen.getByRole("button", { name: /daily/i })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /carbon-aware/i }),
    ).toBeInTheDocument();
  });

  it("shows time input when type is daily", () => {
    render(<ScheduleForm schedule={baseSchedule} onSave={vi.fn()} />);
    expect(screen.getByLabelText(/start time/i)).toBeInTheDocument();
  });

  it("shows window inputs when type is carbon_aware", () => {
    const caSchedule: Schedule = {
      id: "s2",
      type: "carbon_aware",
      time: "06:00",
      windowStart: "01:00",
      windowEnd: "07:00",
      enabled: true,
    };
    render(<ScheduleForm schedule={caSchedule} onSave={vi.fn()} />);
    expect(screen.getByLabelText(/earliest/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/ready by/i)).toBeInTheDocument();
  });

  it("calls onSave with correct daily payload on save", () => {
    const onSave = vi.fn();
    render(
      <ScheduleForm schedule={baseSchedule} onSave={onSave} saveLabel="Save" />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSave).toHaveBeenCalledWith({
      type: "daily",
      time: "06:00",
      enabled: true,
    });
  });

  it("shows Skip button when onSkip provided", () => {
    const onSkip = vi.fn();
    render(<ScheduleForm schedule={null} onSave={vi.fn()} onSkip={onSkip} />);
    fireEvent.click(screen.getByRole("button", { name: "Skip" }));
    expect(onSkip).toHaveBeenCalled();
  });

  it("does not show Skip button when onSkip not provided", () => {
    render(<ScheduleForm schedule={null} onSave={vi.fn()} />);
    expect(
      screen.queryByRole("button", { name: "Skip" }),
    ).not.toBeInTheDocument();
  });

  it("does not show ready-by input by default for daily schedule", () => {
    render(<ScheduleForm schedule={baseSchedule} onSave={vi.fn()} />);
    expect(screen.queryByLabelText(/ready by/i)).not.toBeInTheDocument();
  });

  it("shows ready-by input when schedule has readyBy set", () => {
    const twoStageSchedule: Schedule = {
      ...baseSchedule,
      readyBy: "10:00",
    };
    render(<ScheduleForm schedule={twoStageSchedule} onSave={vi.fn()} />);
    expect(screen.getByLabelText(/ready by/i)).toHaveValue("10:00");
  });

  it("toggling two-stage charging reveals the ready-by input", () => {
    render(<ScheduleForm schedule={baseSchedule} onSave={vi.fn()} />);
    fireEvent.click(screen.getByRole("switch", { name: "Two-stage charging" }));
    expect(screen.getByLabelText(/ready by/i)).toBeInTheDocument();
  });

  it("calls onSave with readyBy when two-stage charging is enabled", () => {
    const onSave = vi.fn();
    const twoStageSchedule: Schedule = {
      ...baseSchedule,
      readyBy: "10:00",
    };
    render(
      <ScheduleForm
        schedule={twoStageSchedule}
        onSave={onSave}
        saveLabel="Save"
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSave).toHaveBeenCalledWith({
      type: "daily",
      time: "06:00",
      readyBy: "10:00",
      enabled: true,
    });
  });

  it("omits readyBy from payload when two-stage charging is disabled", () => {
    const onSave = vi.fn();
    render(
      <ScheduleForm schedule={baseSchedule} onSave={onSave} saveLabel="Save" />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSave).toHaveBeenCalledWith({
      type: "daily",
      time: "06:00",
      enabled: true,
    });
  });

  it("validates that daily readyBy differs from start time", () => {
    const sameTimeSchedule: Schedule = {
      ...baseSchedule,
      readyBy: "06:00",
    };
    render(
      <ScheduleForm
        schedule={sameTimeSchedule}
        onSave={vi.fn()}
        saveLabel="Save"
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(
      screen.getByText("Ready by must differ from start time."),
    ).toBeInTheDocument();
  });

  it("validates that carbon_aware window start and end differ", () => {
    // Use a carbon_aware schedule with identical start/end to trigger the error
    const sameWindowSchedule: Schedule = {
      id: "s3",
      type: "carbon_aware",
      time: "01:00",
      windowStart: "06:00",
      windowEnd: "06:00",
      enabled: true,
    };
    render(
      <ScheduleForm
        schedule={sameWindowSchedule}
        onSave={vi.fn()}
        saveLabel="Save"
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(
      screen.getByText("Start and ready-by times must differ."),
    ).toBeInTheDocument();
  });
});
