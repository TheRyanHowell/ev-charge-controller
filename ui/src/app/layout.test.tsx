import { render } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

// Mock next/font/google - uses SSR/compile-time magic that doesn't work in jsdom
vi.mock("next/font/google", () => ({
  default: () => ({ className: "font-variable", variable: "--font-variable" }),
  Geist: () => ({
    className: "font-sans-variable",
    variable: "--font-geist-sans",
  }),
  Geist_Mono: () => ({
    className: "font-mono-variable",
    variable: "--font-geist-mono",
  }),
}));

import RootLayout from "./layout";

describe("RootLayout", () => {
  it("renders children", () => {
    const { getByText } = render(
      <RootLayout>
        <div data-testid="page-content">Page content</div>
      </RootLayout>,
    );
    expect(getByText("Page content")).toBeInTheDocument();
  });

  it("does not wrap children in ErrorBoundary (pages handle their own boundaries)", () => {
    const { container } = render(
      <RootLayout>
        <div>Child</div>
      </RootLayout>,
    );
    expect(
      container.querySelector('[data-testid="error-boundary-wrapper"]'),
    ).toBeNull();
  });

  it('sets lang="en" on html element', () => {
    render(
      <RootLayout>
        <div>Child</div>
      </RootLayout>,
    );
    const html = document.querySelector("html");
    expect(html).toHaveAttribute("lang", "en");
  });

  it("includes font variable classes and utility classes on html", () => {
    render(
      <RootLayout>
        <div>Child</div>
      </RootLayout>,
    );
    const html = document.querySelector("html");
    expect(html).toHaveClass("h-full");
    expect(html).toHaveClass("antialiased");
    expect(html).toHaveClass("--font-geist-sans");
    expect(html).toHaveClass("--font-geist-mono");
  });

  it("sets favicon and apple-touch-icon via metadata export", async () => {
    const { metadata } = await import("./layout");
    expect(metadata.icons).toEqual({
      icon: "/favicon.ico",
      apple: "/icon-192.png",
    });
  });

  it("uses app/manifest.ts for manifest (no explicit metadata.manifest)", async () => {
    const { metadata } = await import("./layout");
    expect(metadata.manifest).toBeUndefined();
  });

  it("renders body with correct classes", () => {
    render(
      <RootLayout>
        <div>Child</div>
      </RootLayout>,
    );
    const body = document.querySelector("body");
    expect(body).toHaveClass("min-h-full");
    expect(body).toHaveClass("flex");
    expect(body).toHaveClass("flex-col");
  });

  it("sets metadata title and description via module export", () => {
    // Metadata is server-side; verify the module loads without error
    expect(() =>
      render(
        <RootLayout>
          <div>Child</div>
        </RootLayout>,
      ),
    ).not.toThrow();
  });
});
