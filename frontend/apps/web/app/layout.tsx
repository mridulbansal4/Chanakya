import type { Metadata } from "next"
import { Inter, Plus_Jakarta_Sans, JetBrains_Mono } from "next/font/google"

import "@workspace/ui/globals.css"
import { ThemeProvider } from "@/components/theme-provider"
import { Providers } from "@/components/providers"
import { AsOfProvider } from "@/components/as-of-provider"
import { AppShell } from "@/components/app-shell"
import { cn } from "@workspace/ui/lib/utils"

const fontSans = Inter({ subsets: ["latin"], variable: "--font-sans" })

const fontDisplay = Plus_Jakarta_Sans({
  subsets: ["latin"],
  variable: "--font-display",
  weight: ["500", "600", "700", "800"],
})

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
