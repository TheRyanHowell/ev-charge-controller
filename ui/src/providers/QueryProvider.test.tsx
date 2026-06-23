import { ApiError } from "@/lib/api";
import { handleQueryError } from "@/lib/query-error-handler";
import { render } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import { QueryProvider } from "./QueryProvider";

describe("QueryProvider", () => {
  it("renders children", () => {
    const { container } = render(
      <QueryProvider>
        <div data-testid="child">test</div>
      </QueryProvider>,
    );
    expect(
      container.querySelector('[data-testid="child"]'),
    ).toBeInTheDocument();
  });
});

describe("handleQueryError", () => {
  beforeEach(() => {
    vi.stubGlobal("location", { href: "" });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("redirects to /login?reason=session-expired on 401 ApiError", () => {
    handleQueryError(new ApiError("Unauthorized", 401));
    expect(window.location.href).toBe("/login?reason=session-expired");
  });

  it("does not redirect for a 404 ApiError", () => {
    handleQueryError(new ApiError("Not Found", 404));
    expect(window.location.href).toBe("");
  });

  it("does not redirect for a 500 ApiError", () => {
    handleQueryError(new ApiError("Server Error", 500));
    expect(window.location.href).toBe("");
  });

  it("does not redirect for a generic Error", () => {
    handleQueryError(new Error("network error"));
    expect(window.location.href).toBe("");
  });

  it("does not redirect for null error", () => {
    handleQueryError(null);
    expect(window.location.href).toBe("");
  });
});
