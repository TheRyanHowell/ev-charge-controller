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
    expect(screen.getByRole("switch")).toBeInTheDocument();
    expect(screen.getByText("Enabled")).toBeInTheDocument();
  });

  it("reflects schedule.enabled in toggle state", () => {
    render(<ScheduleForm schedule={baseSchedule} onSave={vi.fn()} />);
    expect(screen.getByRole("switch")).toHaveAttribute("aria-checked", "true");
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
