"use client"

import * as React from "react"

// The "as-of date" is a first-class control in CHANAKYA: every data view is a
// bi-temporal reconstruction as-of some world-time date. It lives in one
// context so the same date drives the overview graph, the register, and (later)
// the audit lineage simultaneously.

interface AsOfContextValue {
  /** as-of date, "YYYY-MM-DD". */
  asOf: string
  setAsOf: (d: string) => void
  /** today, "YYYY-MM-DD". */
  today: string
}

const AsOfContext = React.createContext<AsOfContextValue | null>(null)

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

export function AsOfProvider({ children }: { children: React.ReactNode }) {
  const today = todayISO()
  const [asOf, setAsOf] = React.useState(today)
  return (
    <AsOfContext.Provider value={{ asOf, setAsOf, today }}>
      {children}
    </AsOfContext.Provider>
  )
}

export function useAsOf(): AsOfContextValue {
  const ctx = React.useContext(AsOfContext)
  if (!ctx) throw new Error("useAsOf must be used within an AsOfProvider")
  return ctx
}
