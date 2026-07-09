import { render } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

const runMock = vi.fn(
  (
    _context: unknown,
    callback: (err: Error | null, results: { violations: unknown[] }) => void,
  ) => {
    callback(null, { violations: [] });
  },
);

vi.mock("axe-core", () => ({ default: { run: runMock } }));

import AxeAuditor from "./AxeAuditor";

describe("AxeAuditor", () => {
  it("renders nothing", () => {
    const { container } = render(<AxeAuditor />);
    expect(container).toBeEmptyDOMElement();
  });

  it("runs axe-core against the DOM in non-production builds", async () => {
    render(<AxeAuditor />);
    await vi.waitFor(() => expect(runMock).toHaveBeenCalled(), {
      timeout: 2000,
    });
    expect(runMock).toHaveBeenCalledWith(document, expect.any(Function));
  });
});
