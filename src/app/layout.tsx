// Constants
import { APP_DESCRIPTION, APP_NAME } from "@/lib/constants"

// Styles
import "@/styles/globals.css"
import localFont from "next/font/local"
import { Inter, Roboto_Mono } from "next/font/google"

// Types
import { type Metadata } from "next"

// Components
import { TRPCReactProvider } from "@/trpc/react"

export const metadata: Metadata = {
  title: APP_NAME,
  description: APP_DESCRIPTION
}

const inter = Inter({
  variable: "--font-inter",
  subsets: ["latin"]
})

const robotoMono = Roboto_Mono({
  variable: "--font-roboto-mono",
  subsets: ["latin"]
})

const mondwest = localFont({
  src: "../public/fonts/PPMondwest-Regular.otf",
  variable: "--font-mondwest"
})

export default function RootLayout({
  children
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html
      lang="en"
      className={`${inter.variable} ${robotoMono.variable} ${mondwest.variable}`}
    >
      <body>
        <TRPCReactProvider>{children}</TRPCReactProvider>
      </body>
    </html>
  )
}
