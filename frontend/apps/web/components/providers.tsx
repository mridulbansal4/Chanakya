"use client"

import * as React from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { PersistQueryClientProvider } from "@tanstack/react-query-persist-client"
import { createSyncStoragePersister } from "@tanstack/query-sync-storage-persister"

export function Providers({ children }: { children: React.ReactNode }) {
  const [client] = React.useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            gcTime: 1000 * 60 * 60 * 24, // 24 hours
            staleTime: 30_000,
            retry: 1,
            refetchOnWindowFocus: false,
          },
        },
      }),
  )

  const [persister] = React.useState(() => {
    if (typeof window !== "undefined") {
      return createSyncStoragePersister({
        storage: window.localStorage,
      })
    }
    return undefined;
  });

  if (!persister) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>
  }

  return (
    <PersistQueryClientProvider client={client} persistOptions={{ persister }}>
      {children}
    </PersistQueryClientProvider>
  )
}
