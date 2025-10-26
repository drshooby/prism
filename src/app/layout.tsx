// Constants
import { APP_DESCRIPTION, APP_NAME } from "@/lib/constants"

// Styles
import "@/styles/globals.css"
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

export default function RootLayout({
  children
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${inter.variable} ${robotoMono.variable}`}>
      <body>
        <TRPCReactProvider>{children}</TRPCReactProvider>
      </body>
    </html>
  )
}
