import { type Metadata } from 'next'
import { ClerkProvider } from '@clerk/nextjs'
// import { dark, neobrutalism } from '@clerk/themes'
import { Geist, Geist_Mono } from 'next/font/google'
import { Toaster } from "@/components/ui/sonner"
import './globals.css'

const geistSans = Geist({
  variable: '--font-geist-sans',
  subsets: ['latin'],
})

const geistMono = Geist_Mono({
  variable: '--font-geist-mono',
  subsets: ['latin'],
})

export const metadata: Metadata = {
  title: 'Nimbus',
  description: 'Nimbus',
}

// Every page depends on Clerk auth at render time (auth() in page.tsx,
// ClerkProvider in this layout), so nothing here can be statically
// prerendered. Marking the layout dynamic keeps `next build` from
// requiring live Clerk keys; running the app still does.
export const dynamic = 'force-dynamic'

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <ClerkProvider>
      <html lang="en">
        <body className={`${geistSans.variable} ${geistMono.variable} antialiased`}>
          {children}
          <Toaster />
        </body>
      </html>
    </ClerkProvider>
  );
}