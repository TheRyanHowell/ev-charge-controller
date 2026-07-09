import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

import Toggle from "./Toggle";

describe("Toggle", () => {
  it("renders with role switch", () => {
    render(<Toggle checked={false} onChange={vi.fn()} />);
    expect(screen.getByRole("switch")).toBeInTheDocument();
  });

  it("reflects checked state via aria-checked", () => {
    render(<Toggle checked onChange={vi.fn()} />);
    expect(screen.getByRole("switch")).toHaveAttribute("aria-checked", "true");
  });

  it("reflects unchecked state via aria-checked", () => {
    render(<Toggle checked={false} onChange={vi.fn()} />);
    expect(screen.getByRole("switch")).toHaveAttribute("aria-checked", "false");
  });

  it("calls onChange with the inverted value when clicked", () => {
    const onChange = vi.fn();
    render(<Toggle checked={false} onChange={onChange} />);
    fireEvent.click(screen.getByRole("switch"));
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it("is disabled when the disabled prop is set", () => {
    render(<Toggle checked={false} onChange={vi.fn()} disabled />);
    expect(screen.getByRole("switch")).toBeDisabled();
  });

  it("does not call onChange when disabled and clicked", () => {
    const onChange = vi.fn();
    render(<Toggle checked={false} onChange={onChange} disabled />);
    fireEvent.click(screen.getByRole("switch"));
    expect(onChange).not.toHaveBeenCalled();
  });

  it("sets aria-label when a label is provided", () => {
    render(<Toggle checked={false} onChange={vi.fn()} label="Dark mode" />);
    expect(screen.getByRole("switch")).toHaveAttribute(
      "aria-label",
      "Dark mode",
    );
  });

  it("centers the thumb in the track so it isn't flush against the edges", () => {
    render(<Toggle checked={false} onChange={vi.fn()} />);
    const track = screen.getByRole("switch");
    expect(track).toHaveClass("items-center");
    expect(track).toHaveClass("p-0.5");
  });
});
