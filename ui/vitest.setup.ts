import "@testing-library/jest-dom";
import { cleanup } from "@testing-library/react";
import { afterAll, afterEach, beforeAll } from "vitest";
import failOnConsole from "vitest-fail-on-console";
import "vitest-canvas-mock";

// Polyfill PointerEvent for JSDOM (needed for pointer-based drag tests)
if (typeof globalThis.PointerEvent === "undefined") {
  class PointerEventPolyfill extends MouseEvent {
    public pointerId: number;
    public pointerType: string;
    public isPrimary: boolean;
    public pressure: number;
    public tiltX: number;
    public tiltY: number;
    public twist: number;
    public width: number;
    public height: number;

    public constructor(
      type: string,
      init: PointerEventInit = {},
    ) {
      super(type, init);
      this.pointerId = init.pointerId ?? 1;
      this.pointerType = init.pointerType ?? "mouse";
      this.isPrimary = init.isPrimary ?? true;
      this.pressure = init.pressure ?? 0;
      this.tiltX = init.tiltX ?? 0;
      this.tiltY = init.tiltY ?? 0;
      this.twist = init.twist ?? 0;
      this.width = init.width ?? 1;
      this.height = init.height ?? 1;
    }
  }
  globalThis.PointerEvent = PointerEventPolyfill as unknown as typeof PointerEvent;
}

// Polyfill setPointerCapture for JSDOM (needed for pointer-based drag tests)
if (!Element.prototype.setPointerCapture) {
  Element.prototype.setPointerCapture = function () {
    // No-op in JSDOM
  };
}

// Catch unexpected console.warn during tests; console.error is allowed
// since the app uses it for intentional error logging in catch blocks.
failOnConsole({
  shouldFailOnWarn: true,
  shouldFailOnError: false,
});

// Cleanup after each test
afterEach(() => {
  cleanup();
});

// Mock window.matchMedia
beforeAll(() => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  });
});

// Mock ResizeObserver
class ResizeObserver {
  observe = () => {};
  unobserve = () => {};
  disconnect = () => {};
}
global.ResizeObserver = ResizeObserver;

// Polyfill <dialog> element methods for JSDOM
beforeAll(() => {
  if (!HTMLDialogElement.prototype.showModal) {
    HTMLDialogElement.prototype.showModal = function () {
      this.setAttribute("open", "");
      this.setAttribute("aria-modal", "true");
      this.style.display = "";
      document.body.style.overflow = "hidden";
    };
    HTMLDialogElement.prototype.close = function () {
      this.removeAttribute("open");
      this.removeAttribute("aria-modal");
      this.style.display = "none";
      document.body.style.overflow = "";
      this.dispatchEvent(new Event("close", { bubbles: false, cancelable: false }));
    };
    Object.defineProperty(HTMLDialogElement.prototype, "open", {
      get() {
        return this.hasAttribute("open");
      },
    });
  }

  // Handle Escape key for <dialog> (native behavior)
  const originalKeyDown = document.dispatchEvent.bind(document);
  document.addEventListener(
    "keydown",
    (e) => {
      if (e.key === "Escape") {
        const openDialog = document.querySelector("dialog[open]");
        if (openDialog) {
          (openDialog as HTMLDialogElement).close();
        }
      }
    },
    true,
  );
  void originalKeyDown;

  // Handle focus trap for <dialog> (native behavior)
  document.addEventListener("keydown", (e) => {
    if (e.key !== "Tab") return;
    const openDialog = document.querySelector("dialog[open]");
    if (!openDialog) return;

    const focusable = openDialog.querySelectorAll<HTMLElement>(
      'button:not([tabindex="-1"]), [href]:not([tabindex="-1"]), input:not([tabindex="-1"]), select:not([tabindex="-1"]), textarea:not([tabindex="-1"]), [tabindex]:not([tabindex="-1"])',
    );
    if (!focusable.length) return;

    const first = focusable[0] as HTMLElement | undefined;
    const last = focusable[focusable.length - 1] as HTMLElement | undefined;

    if (e.shiftKey) {
      if (document.activeElement === first) {
        e.preventDefault();
        last?.focus();
      }
    } else {
      if (document.activeElement === last) {
        e.preventDefault();
        first?.focus();
      }
    }
  });
});

// Mock canvas getBoundingClientRect to return proper dimensions
beforeAll(() => {
  Object.defineProperty(HTMLCanvasElement.prototype, 'width', {
    writable: true,
    value: 320,
  });
  Object.defineProperty(HTMLCanvasElement.prototype, 'height', {
    writable: true,
    value: 320,
  });

  Object.defineProperty(HTMLCanvasElement.prototype, 'getBoundingClientRect', {
    value: function() {
      return {
        left: 0,
        top: 0,
        width: this.width || 320,
        height: this.height || 320,
        x: 0,
        y: 0,
        getClientRects: () => [],
        toJSON: () => {},
      };
    },
    writable: true,
    configurable: true,
  });
});
