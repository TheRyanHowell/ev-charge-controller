import { render, screen } from "@testing-library/react";
import React from "react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import ErrorBoundary from "./ErrorBoundary";

// Suppress expected React error boundary logging - these tests intentionally
// trigger errors to verify boundary behavior. The "calls console.error" test
// installs its own spy to assert on the ErrorBoundary's componentDidCatch call.
const originalConsoleError = console.error;
beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation((...args: unknown[]) => {
    const msg = args.map(String).join(" ");
    if (
      msg.includes("Test error") ||
      msg.includes("The above error occurred in the") ||
      msg.includes("ErrorBoundary caught an error") ||
      msg.includes("React will try to recreate this component tree")
    ) {
      return;
    }
    originalConsoleError(...args);
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

// Create a component that throws an error in constructor
class ErrorThrowingComponent extends React.Component<Record<string, never>> {
  constructor(props: Record<string, never>) {
    super(props);
    throw new Error("Test error");
  }

  render() {
    return <div>Should not see this</div>;
  }
}

describe("ErrorBoundary", () => {
  it("renders children when no error", () => {
    render(
      <ErrorBoundary>
        <div data-testid="child-content">Child content</div>
      </ErrorBoundary>,
    );
    expect(screen.getByTestId("child-content")).toBeInTheDocument();
    expect(screen.getByText("Child content")).toBeInTheDocument();
  });

  it("renders default error UI when error occurs", () => {
    render(
      <ErrorBoundary>
        <ErrorThrowingComponent />
      </ErrorBoundary>,
    );

    // Verify error UI is shown
    expect(screen.getByText(/Something went wrong/i)).toBeInTheDocument();
    expect(screen.getByText(/Reload Page/i)).toBeInTheDocument();
  });

  it("renders custom fallback when error occurs", () => {
    const customFallback = (
      <div data-testid="custom-fallback">Custom Error UI</div>
    );

    render(
      <ErrorBoundary fallback={customFallback}>
        <ErrorThrowingComponent />
      </ErrorBoundary>,
    );

    expect(screen.getByTestId("custom-fallback")).toBeInTheDocument();
    expect(screen.getByText("Custom Error UI")).toBeInTheDocument();
  });

  it("calls console.error on componentDidCatch", () => {
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    render(
      <ErrorBoundary>
        <ErrorThrowingComponent />
      </ErrorBoundary>,
    );

    expect(consoleSpy).toHaveBeenCalled();
    consoleSpy.mockRestore();
  });

  it("reload button reloads the page", () => {
    const locationReload = vi.fn();
    Object.defineProperty(window, "location", {
      value: { reload: locationReload },
      writable: true,
    });

    render(
      <ErrorBoundary>
        <ErrorThrowingComponent />
      </ErrorBoundary>,
    );

    const reloadButton = screen.getByText(/Reload Page/i);
    expect(reloadButton).toBeInTheDocument();
    reloadButton.click();
    expect(locationReload).toHaveBeenCalled();

    // Restore location
    const w = window as unknown as Record<string, unknown>;
    delete w.location;
  });

  it("multiple boundaries isolate errors independently", () => {
    render(
      <div>
        <ErrorBoundary
          fallback={<div data-testid="gauge-error">Gauge error</div>}
        >
          <ErrorThrowingComponent />
        </ErrorBoundary>
        <ErrorBoundary
          fallback={<div data-testid="chart-error">Chart error</div>}
        >
          <div data-testid="chart-ok">Chart content</div>
        </ErrorBoundary>
      </div>,
    );

    expect(screen.getByTestId("gauge-error")).toBeInTheDocument();
    expect(screen.getByTestId("chart-ok")).toBeInTheDocument();
  });
});
