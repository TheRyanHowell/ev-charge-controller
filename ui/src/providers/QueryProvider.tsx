"use client";

import { handleQueryError } from "@/lib/query-error-handler";
import {
  MutationCache,
  QueryCache,
  QueryClient,
  QueryClientProvider,
} from "@tanstack/react-query";
import dynamic from "next/dynamic";
import { useEffect, useState, type ReactNode } from "react";

const GC_TIME_MS = 1000 * 60 * 30;

declare global {
  interface Window {
    __queryClient__?: QueryClient;
  }
}

const ReactQueryDevtools =
  process.env.NODE_ENV === "development"
    ? dynamic(
        () =>
          import("@tanstack/react-query-devtools").then((m) => ({
            default: m.ReactQueryDevtools,
          })),
        { ssr: false },
      )
    : null;

export function QueryProvider({ children }: { children: ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        queryCache: new QueryCache({ onError: handleQueryError }),
        mutationCache: new MutationCache({ onError: handleQueryError }),
        defaultOptions: {
          queries: {
            staleTime: 1000 * 5,
            gcTime: GC_TIME_MS,
            retry: 1,
          },
        },
      }),
  );

  useEffect(() => {
    if (process.env.NODE_ENV === "development") {
      window.__queryClient__ = queryClient;
    }
  }, [queryClient]);

  return (
    <QueryClientProvider client={queryClient}>
      {children}
      {ReactQueryDevtools ? <ReactQueryDevtools initialIsOpen={false} /> : null}
    </QueryClientProvider>
  );
}
