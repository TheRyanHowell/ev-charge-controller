import type { ReactNode } from "react";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  render,
  renderHook,
  type RenderHookOptions,
  type RenderOptions,
} from "@testing-library/react";
import { useMemo } from "react";

export function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: 5 * 60 * 1000,
      },
      mutations: {
        retry: false,
      },
    },
  });
}

function TestWrapper({ children }: { children: ReactNode }) {
  const queryClient = useMemo(() => makeQueryClient(), []);
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

export function customRender(ui: ReactNode, options?: RenderOptions) {
  return render(ui, { wrapper: TestWrapper, ...options });
}

export function customRenderHook<P, T>(
  hook: (props: P) => T,
  options?: RenderHookOptions<P>,
) {
  return renderHook(hook, { wrapper: TestWrapper, ...options });
}

export {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
