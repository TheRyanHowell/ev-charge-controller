import { render } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

vi.mock("@axe-core/react", () => ({ default: vi.fn() }));

import axe from "@axe-core/react";

import AxeAuditor from "./AxeAuditor";

describe("AxeAuditor", () => {
  it("renders nothing", () => {
    const { container } = render(<AxeAuditor />);
    expect(container).toBeEmptyDOMElement();
  });

  it("runs axe against the DOM in non-production builds", async () => {
    render(<AxeAuditor />);
    await vi.waitFor(() => expect(axe).toHaveBeenCalled());
    expect(axe).toHaveBeenCalledWith(
      expect.anything(),
      expect.anything(),
      1000,
    );
  });
});
