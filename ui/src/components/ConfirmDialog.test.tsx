import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import ConfirmDialog from "./ConfirmDialog";

describe("ConfirmDialog", () => {
  const defaultProps = {
    title: "Confirm Action",
    message: "Are you sure?",
    onConfirm: vi.fn(),
    onCancel: vi.fn(),
  };

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("renders title and message", () => {
    render(<ConfirmDialog {...defaultProps} />);

    expect(screen.getByText("Confirm Action")).toBeInTheDocument();
    expect(screen.getByText("Are you sure?")).toBeInTheDocument();
  });

  it("renders default confirm and cancel buttons", () => {
    render(<ConfirmDialog {...defaultProps} />);

    expect(screen.getByText("Confirm")).toBeInTheDocument();
    expect(screen.getByText("Cancel")).toBeInTheDocument();
  });

  it("renders custom confirm and cancel labels", () => {
    render(
      <ConfirmDialog
        {...defaultProps}
        confirmLabel="Delete"
        cancelLabel="Keep"
      />,
    );

    expect(screen.getByText("Delete")).toBeInTheDocument();
    expect(screen.getByText("Keep")).toBeInTheDocument();
  });

  it("calls onCancel when cancel button is clicked", () => {
    render(<ConfirmDialog {...defaultProps} />);

    fireEvent.click(screen.getByText("Cancel"));

    expect(defaultProps.onCancel).toHaveBeenCalledTimes(1);
  });

  it("calls onConfirm when confirm button is clicked", () => {
    render(<ConfirmDialog {...defaultProps} />);

    fireEvent.click(screen.getByText("Confirm"));

    expect(defaultProps.onConfirm).toHaveBeenCalledTimes(1);
  });

  it("calls onConfirm when Enter is pressed on the confirm button", () => {
    render(<ConfirmDialog {...defaultProps} />);

    fireEvent.keyDown(screen.getByText("Confirm"), { key: "Enter" });

    expect(defaultProps.onConfirm).toHaveBeenCalledTimes(1);
  });

  it("calls onConfirm when Enter is pressed on the confirm div", () => {
    render(<ConfirmDialog {...defaultProps} />);

    const confirmEl = screen.getByText("Confirm");
    const confirmDiv = confirmEl.closest("div");
    if (!confirmDiv) return;

    fireEvent.keyDown(confirmDiv, { key: "Enter" });

    expect(defaultProps.onConfirm).toHaveBeenCalledTimes(1);
  });

  it("does not call onConfirm for other keys", () => {
    render(<ConfirmDialog {...defaultProps} />);

    fireEvent.keyDown(screen.getByText("Confirm"), { key: "Escape" });

    expect(defaultProps.onConfirm).not.toHaveBeenCalled();
  });

  it("applies danger variant classes to confirm button", () => {
    render(<ConfirmDialog {...defaultProps} variant="danger" />);

    const confirmBtn = screen.getByText("Confirm");

    expect(confirmBtn).toHaveClass("bg-red-600");
    expect(confirmBtn).toHaveClass("hover:bg-red-500");
    expect(confirmBtn).toHaveClass("focus-visible:ring-red-500");
  });

  it("applies info variant classes to confirm button", () => {
    render(<ConfirmDialog {...defaultProps} variant="info" />);

    const confirmBtn = screen.getByText("Confirm");

    expect(confirmBtn).toHaveClass("bg-blue-600");
    expect(confirmBtn).toHaveClass("hover:bg-blue-500");
    expect(confirmBtn).toHaveClass("focus-visible:ring-blue-500");
  });

  it("focuses the confirm button on mount", async () => {
    const focusSpy = vi.spyOn(HTMLButtonElement.prototype, "focus");

    render(<ConfirmDialog {...defaultProps} />);

    await waitFor(() => {
      expect(focusSpy).toHaveBeenCalled();
    });

    focusSpy.mockRestore();
  });
});
