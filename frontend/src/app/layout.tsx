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