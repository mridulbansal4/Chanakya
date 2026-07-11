"use client"

import * as React from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"

/**
 * Providers holds app-wide client context. Currently TanStack Query; later
 * phases add more here. The QueryClient is created once per browser session.
 */
export function Providers({ children }: { children: React.ReactNode }) {
  const [client] = React.useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 30_000,
            retry: 1,
            refetchOnWindowFocus: false,
          },
        },
      }),
  )
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>
}
