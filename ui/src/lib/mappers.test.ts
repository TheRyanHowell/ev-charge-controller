import { describe, it, expect } from "vitest";

import { mapBackendStatus } from "./mappers";

describe("mapBackendStatus", () => {
  it("maps 'active' to 'charging'", () => {
    expect(mapBackendStatus("active")).toBe("charging");
  });

  it("maps 'pending' to 'pending'", () => {
    expect(mapBackendStatus("pending")).toBe("pending");
  });

  it("maps 'holding' to 'holding'", () => {
    expect(mapBackendStatus("holding")).toBe("holding");
  });

  it("maps 'cancelled' to 'error'", () => {
    expect(mapBackendStatus("cancelled")).toBe("error");
  });

  it("maps 'completed' to 'idle'", () => {
    expect(mapBackendStatus("completed")).toBe("idle");
  });

  it("maps 'inactive' to 'idle'", () => {
    expect(mapBackendStatus("inactive")).toBe("idle");
  });

  it("throws on unknown status", () => {
    expect(() => mapBackendStatus("unknown")).toThrow(
      "Unknown backend status: unknown",
    );
  });

  it("throws on empty string", () => {
    expect(() => mapBackendStatus("")).toThrow("Unknown backend status: ");
  });

  it("throws on fault status", () => {
    expect(() => mapBackendStatus("fault")).toThrow(
      "Unknown backend status: fault",
    );
  });
});
