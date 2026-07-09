import { render, screen, fireEvent } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import Dialog from "./Dialog";

describe("Dialog", () => {
  afterEach(() => {
    document.body.style.overflow = "";
  });

  it("renders content when open", () => {
    render(
      <Dialog isOpen onClose={vi.fn()} aria-labelledby="test-title">
        <div id="test-title">Test Dialog</div>
      </Dialog>,
    );

    expect(screen.getByText("Test Dialog")).toBeInTheDocument();
  });

  it("dialog is not open when isOpen is false", () => {
    const { container } = render(
      <Dialog isOpen={false} onClose={vi.fn()} aria-labelledby="test-title">
        <div id="test-title">Test Dialog</div>
      </Dialog>,
    );

    const dialog = container.querySelector("dialog");
    expect(dialog).not.toBeNull();
    expect(dialog?.open).toBe(false);
  });

  it("calls onClose when Escape is pressed", () => {
    const onClose = vi.fn();
    render(
      <Dialog isOpen onClose={onClose} aria-labelledby="test-title">
        <div id="test-title">Test Dialog</div>
        <button>OK</button>
      </Dialog>,
    );

    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalled();
  });

  it("calls onClose when backdrop is clicked", () => {
    const onClose = vi.fn();
    render(
      <Dialog isOpen onClose={onClose} aria-labelledby="test-title">
        <div className="bg-surface-raised rounded-xl p-6 max-w-sm w-full mx-4">
          <h2 id="test-title">Test Dialog</h2>
          <button>OK</button>
        </div>
      </Dialog>,
    );

    // Click on the dialog element (backdrop area)
    const dialog = document.querySelector("dialog") as HTMLDialogElement | null;
    if (dialog) {
      fireEvent.click(dialog);
    }
    expect(onClose).toHaveBeenCalled();
  });

  it("does not call onClose when dialog content is clicked", () => {
    const onClose = vi.fn();
    render(
      <Dialog isOpen onClose={onClose} aria-labelledby="test-title">
        <div className="bg-surface-raised rounded-xl p-6 max-w-sm w-full mx-4">
          <h2 id="test-title">Test Dialog</h2>
          <button>OK</button>
        </div>
      </Dialog>,
    );

    const content = screen.getByText("Test Dialog");
    fireEvent.click(content);
    expect(onClose).not.toHaveBeenCalled();
  });
});
