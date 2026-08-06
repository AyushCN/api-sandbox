import Link from "next/link";
import { LogoutButton } from "@/components/LogoutButton";

export default function MainLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <>
      <nav className="sticky top-0 z-50 w-full border-b border-outline-variant bg-surface-container-lowest/80 backdrop-blur-xl">
        <div className="container mx-auto px-4 h-16 flex items-center justify-between">
          <Link href="/dashboard" className="flex items-center gap-2 text-on-surface hover:text-primary transition-colors">
            <span className="material-symbols-outlined text-primary text-3xl">token</span>
            <span className="font-bold tracking-tight text-xl text-primary">API Sandbox</span>
          </Link>
          <div className="flex items-center gap-6">
            <Link href="/upload" className="text-sm font-bold tracking-wide text-on-surface-variant hover:text-primary transition-colors uppercase">
              New Sandbox
            </Link>
            <LogoutButton />
          </div>
        </div>
      </nav>
      <main className="flex-1 container mx-auto px-4 py-8">
        {children}
      </main>
    </>
  );
}
