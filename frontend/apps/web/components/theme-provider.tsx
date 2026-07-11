"use client"

import * as React from "react"
import { ThemeProvider as NextThemesProvider } from "next-themes"

/**
 * CHANAKYA "Editorial" is a single locked light palette (warm cream + ink). We
 * force one theme so there's no dark/neon variant to drift out of brand.
 */
function ThemeProvider({
  children,
  ...props
}: React.ComponentProps<typeof NextThemesProvider>) {
  return (
    <NextThemesProvider
      attribute="class"
      forcedTheme="light"
      enableSystem={false}
      disableTransitionOnChange
      {...props}
    >
      {children}
    </NextThemesProvider>
  )
}

export { ThemeProvider }
