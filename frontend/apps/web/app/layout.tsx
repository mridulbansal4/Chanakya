import type { Metadata, Viewport } from "next"
import { Inter, Source_Serif_4, JetBrains_Mono } from "next/font/google"

import "@workspace/ui/globals.css"
import { ThemeProvider } from "@/components/theme-provider"
import { Providers } from "@/components/providers"
import { AsOfProvider } from "@/components/as-of-provider"
import { AppShell } from "@/components/app-shell"
import { cn } from "@workspace/ui/lib/utils"

/**
 * Three families, three jobs. See the TYPE SCALE block in globals.css for
 * the rules on which one is allowed where.
 */
const fontSans = Inter({
  subsets: ["latin"],
  variable: "--font-sans",
  display: "swap",
})

const fontSerif = Source_Serif_4({
  subsets: ["latin"],
  variable: "--font-serif",
  // 700 is loaded because existing headings pair `font-display` with
  // `font-bold`; without it the browser synthesises a fake bold, which on a
  // serif smears the stroke contrast that makes the face worth using.
  weight: ["400", "600", "700"],
  display: "swap",
})

const fontMono = JetBrains_Mono({
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  variable: "--font-mono",
  display: "swap",
})

export const metadata: Metadata = {
  title: "CHANAKYA",
  description:
    "Agentic compliance for the Indian securities market: from regulatory text to operational action.",
  icons: {
    /*
      Tab chrome follows the OS appearance, not the app's own theme toggle,
      so the mark is switched on prefers-color-scheme rather than on the
      .dark class the header uses.

      The light-scheme entry is declared last deliberately: a browser that
      ignores `media` on rel=icon falls back to the final one, and a dark
      mark on the light tab bar most browsers ship is the safer thing to be
      wrong about than a white mark that disappears into it.
    */
    icon: [
      {
        url: "/logo-dark.png",
        type: "image/png",
        media: "(prefers-color-scheme: dark)",
      },
      {
        url: "/logo-light.png",
        type: "image/png",
        media: "(prefers-color-scheme: light)",
      },
    ],
  },
}

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  // No maximumScale / userScalable:false - pinch zoom must never be disabled.
  // Dark is the product default, but the theme toggle can switch to light, so
  // the document must advertise both - pinning this to "dark" leaves the UA
  // painting dark form controls and scrollbars over a light page.
  colorScheme: "dark light",
  themeColor: [
    { media: "(prefers-color-scheme: dark)", color: "#0b0c0e" },
    { media: "(prefers-color-scheme: light)", color: "#f7f8fa" },
  ],
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
        "font-sans",
        fontSans.variable,
        fontSerif.variable,
        fontMono.variable,
      )}
    >
      <body>
        <a
          href="#main-content"
          /* z-50 clears the app header (z-30). */
          className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-50 focus:rounded-md focus:bg-accent-solid focus:px-4 focus:py-2 focus:text-label-lg focus:text-accent-on"
        >
          Skip to content
        </a>
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
