"use client";

import { useAuth } from "@/lib/auth";
import { LogOut } from "lucide-react";

export function LogoutButton() {
  const { isAuthenticated, logout } = useAuth();

  if (!isAuthenticated) return null;

  return (
    <button
      onClick={logout}
      className="text-sm font-medium text-white/70 hover:text-white transition-colors flex items-center gap-2"
    >
      <LogOut className="w-4 h-4" />
      Sign out
    </button>
  );
}
