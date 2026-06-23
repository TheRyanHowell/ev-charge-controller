import {
  customRender as render,
  fireEvent,
  screen,
  waitFor,
} from "@/test-utils";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import { LoginForm } from "./LoginForm";

// Mock next/navigation
const mockPush = vi.fn();
const mockRefresh = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
    refresh: mockRefresh,
  }),
}));

const mockFetch = vi.fn();

describe("LoginForm", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", mockFetch);
    mockFetch.mockReset();
    mockPush.mockReset();
    mockRefresh.mockReset();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders email and password inputs", () => {
    render(<LoginForm />);

    expect(screen.getByLabelText(/email address/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /sign in/i }),
    ).toBeInTheDocument();
  });

  it("shows enabled button (HTML5 validation handles empty fields)", () => {
    render(<LoginForm />);

    const button = screen.getByRole("button", { name: /sign in/i });
    // Button is always enabled; HTML5 `required` on inputs prevents
    // submission when fields are empty
    expect(button).toBeEnabled();
  });

  it("submits form with correct credentials", async () => {
    mockFetch.mockImplementation(() => Promise.resolve({ ok: true }));

    render(<LoginForm />);

    // Fill fields using fireEvent (sets DOM value)
    fireEvent.change(screen.getByLabelText(/email address/i), {
      target: { value: "user@example.com" },
    });
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: "password123" },
    });

    // Submit the form
    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    // Verify the API was called with correct credentials (read from refs)
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email: "user@example.com",
          password: "password123",
        }),
      });
    });
  });

  it("submits successfully and redirects", async () => {
    mockFetch.mockImplementation(() => Promise.resolve({ ok: true }));

    render(<LoginForm />);

    fireEvent.change(screen.getByLabelText(/email address/i), {
      target: { value: "user@example.com" },
    });
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: "password123" },
    });

    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email: "user@example.com",
          password: "password123",
        }),
      });
    });

    expect(mockPush).toHaveBeenCalledWith("/dashboard");
    expect(mockRefresh).toHaveBeenCalled();
  });

  it("shows error on failed login", async () => {
    mockFetch.mockImplementation(() =>
      Promise.resolve({
        ok: false,
        status: 401,
        json: async () => ({ detail: "Invalid credentials" }),
      }),
    );

    render(<LoginForm />);

    fireEvent.change(screen.getByLabelText(/email address/i), {
      target: { value: "user@example.com" },
    });
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: "wrong" },
    });

    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Invalid credentials",
      );
    });
  });

  it("shows generic error when response has no detail", async () => {
    mockFetch.mockImplementation(() =>
      Promise.resolve({
        ok: false,
        status: 401,
        json: async () => ({}),
      }),
    );

    render(<LoginForm />);

    fireEvent.change(screen.getByLabelText(/email address/i), {
      target: { value: "user@example.com" },
    });
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: "wrong" },
    });

    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        /Login failed\. Please check your credentials\./,
      );
    });
  });

  it("shows network error on fetch failure", async () => {
    mockFetch.mockImplementation(() =>
      Promise.reject(new Error("network error")),
    );

    render(<LoginForm />);

    fireEvent.change(screen.getByLabelText(/email address/i), {
      target: { value: "user@example.com" },
    });
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: "password123" },
    });

    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Network error. Please try again.",
      );
    });
  });

  it("shows submitting state during login", async () => {
    const onResolve = vi.fn();
    mockFetch.mockImplementation(
      () =>
        new Promise((resolve) => {
          onResolve.mockImplementation(() => resolve({ ok: true }));
        }),
    );

    render(<LoginForm />);

    fireEvent.change(screen.getByLabelText(/email address/i), {
      target: { value: "user@example.com" },
    });
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: "password123" },
    });

    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /signing in/i }),
      ).toBeInTheDocument();
    });

    onResolve();
  });

  it("disables inputs during submission", async () => {
    const onResolve = vi.fn();
    mockFetch.mockImplementation(
      () =>
        new Promise((resolve) => {
          onResolve.mockImplementation(() => resolve({ ok: true }));
        }),
    );

    render(<LoginForm />);

    fireEvent.change(screen.getByLabelText(/email address/i), {
      target: { value: "user@example.com" },
    });
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: "password123" },
    });

    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => {
      expect(screen.getByLabelText(/email address/i)).toBeDisabled();
      expect(screen.getByLabelText(/password/i)).toBeDisabled();
    });

    onResolve();
  });
});
