import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const mockRedirect = vi.hoisted(() => vi.fn());
const mockCookiesImpl = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({
  redirect: mockRedirect,
}));

vi.mock("next/headers", () => ({
  cookies: () => mockCookiesImpl(),
}));

vi.mock("./LoginForm", () => ({
  LoginForm: () => <div data-testid="login-form">Login Form</div>,
}));

import LoginPage from "./page";

const emptyCookieStore = {
  get: (_name: string) => undefined,
};

const loggedInCookieStore = {
  get: (name: string) =>
    name === "access_token" ? { value: "tok" } : undefined,
};

describe("LoginPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the login form", async () => {
    mockCookiesImpl.mockReturnValue(emptyCookieStore);
    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    render(jsx);
    expect(screen.getByTestId("login-form")).toBeInTheDocument();
  });

  it("does NOT render session-expired banner when reason is absent", async () => {
    mockCookiesImpl.mockReturnValue(emptyCookieStore);
    const jsx = await LoginPage({ searchParams: Promise.resolve({}) });
    render(jsx);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("does NOT render session-expired banner for unrelated reason", async () => {
    mockCookiesImpl.mockReturnValue(emptyCookieStore);
    const jsx = await LoginPage({
      searchParams: Promise.resolve({ reason: "other" }),
    });
    render(jsx);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("renders session-expired banner when reason=session-expired", async () => {
    mockCookiesImpl.mockReturnValue(emptyCookieStore);
    const jsx = await LoginPage({
      searchParams: Promise.resolve({ reason: "session-expired" }),
    });
    render(jsx);
    const banner = screen.getByRole("status");
    expect(banner).toBeInTheDocument();
    expect(banner).toHaveTextContent(/session has expired/i);
  });

  it("redirects to dashboard when access_token cookie is present", async () => {
    mockCookiesImpl.mockReturnValue(loggedInCookieStore);
    await LoginPage({ searchParams: Promise.resolve({}) });
    expect(mockRedirect).toHaveBeenCalledWith("/dashboard");
  });
});
