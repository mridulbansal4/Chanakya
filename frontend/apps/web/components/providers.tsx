"use client"

import * as React from "react"
import { QueryClient } from "@tanstack/react-query"
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

  // The persister is created unconditionally so the provider tree is identical
  // on the server and the client. Passing `storage: undefined` during SSR makes
  // it a no-op; branching on `typeof window` here instead would swap the
  // provider component between render passes and break hydration.
  const [persister] = React.useState(() =>
    createSyncStoragePersister({
      storage: typeof window !== "undefined" ? window.localStorage : undefined,
    }),
  )

  return (
    <PersistQueryClientProvider client={client} persistOptions={{ persister }}>
      {children}
    </PersistQueryClientProvider>
  )
}
