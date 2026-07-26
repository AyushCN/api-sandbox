import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import Link from "next/link";
import { Box } from "lucide-react";
import { Toaster } from "react-hot-toast";
import { AuthProvider } from "@/lib/auth";
import { LogoutButton } from "@/components/LogoutButton";

const inter = Inter({ subsets: ["latin"] });

export const metadata: Metadata = {
  title: "API Sandbox",
  description: "Deploy backend APIs instantly.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="dark">
      <body className={`${inter.className} min-h-screen flex flex-col`}>
        <AuthProvider>
          <nav className="sticky top-0 z-50 w-full border-b border-white/10 bg-background/50 backdrop-blur-xl">
            <div className="container mx-auto px-4 h-16 flex items-center justify-between">
              <Link href="/" className="flex items-center gap-2 text-white hover:text-primary transition-colors">
                <Box className="w-6 h-6 text-primary" />
                <span className="font-bold tracking-tight text-lg">API Sandbox</span>
              </Link>
              <div className="flex gap-4">
                <Link href="/upload" className="text-sm font-medium text-white/70 hover:text-white transition-colors">
                  New Sandbox
                </Link>
                <LogoutButton />
              </div>
            </div>
          </nav>
          <main className="flex-1 container mx-auto px-4 py-8">
            {children}
          </main>
          <Toaster position="bottom-right" toastOptions={{ className: 'glass-panel text-white' }} />
        </AuthProvider>
      </body>
    </html>
  );
}
