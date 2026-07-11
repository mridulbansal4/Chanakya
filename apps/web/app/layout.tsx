import type { Metadata } from "next"
import { Fraunces, Inter, JetBrains_Mono } from "next/font/google"

import "@workspace/ui/globals.css"
import { ThemeProvider } from "@/components/theme-provider"
import { Providers } from "@/components/providers"
import { AsOfProvider } from "@/components/as-of-provider"
import { AppShell } from "@/components/app-shell"
import { cn } from "@workspace/ui/lib/utils"

// UI / body / tables / buttons — Inter (the clean base font).
const fontSans = Inter({ subsets: ["latin"], variable: "--font-sans" })

// Page titles + CHANAKYA wordmark — Fraunces (high-contrast editorial serif).
const fontDisplay = Fraunces({
  subsets: ["latin"],
  variable: "--font-display",
  weight: ["400", "500", "600", "700"],
})

// Machine values only — IDs, hashes, clause numbers, Rego, JSON.
const fontMono = JetBrains_Mono({
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  variable: "--font-mono",
})

export const metadata: Metadata = {
  title: "CHANAKYA — Regulatory Operating System",
  description:
    "Agentic compliance for the Indian securities market: from regulatory text to operational action.",
}

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html
      lang="en"
      suppressHydrationWarning
      className={cn(
        "antialiased",
        "font-sans",
        fontSans.variable,
        fontDisplay.variable,
        fontMono.variable,
      )}
    >
      <body>
        <ThemeProvider>
          <Providers>
            <AsOfProvider>
              <AppShell>{children}</AppShell>
            </AsOfProvider>
          </Providers>
        </ThemeProvider>
      </body>
    </html>
  )
}
