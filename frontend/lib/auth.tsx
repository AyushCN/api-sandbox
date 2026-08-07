"use client";

import { createContext, useContext, useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";

interface AuthContextType {
  isAuthenticated: boolean;
  login: () => void;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  // Instead of storing the token, we just track if the user is authenticated
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(false);
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    // Check if the backend recognizes our cookie session by hitting /api/user/me
    fetchWithAuth("/api/user/me", { cache: "no-store" })
      .then(() => {
        setIsAuthenticated(true);
        if (
          pathname === "/login" || 
          pathname === "/register" ||
          pathname === "/forgot-password" ||
          pathname === "/reset-password" ||
          pathname === "/verify"
        ) {
          router.push("/dashboard");
        }
      })
      .catch(() => {
        setIsAuthenticated(false);
        if (
          pathname !== "/" &&
          pathname !== "/login" && 
          pathname !== "/register" &&
          pathname !== "/forgot-password" &&
          pathname !== "/reset-password" &&
          pathname !== "/verify"
        ) {
          router.push("/login");
        }
      });
  }, [pathname, router]);

  const login = () => {
    setIsAuthenticated(true);
    router.push("/dashboard");
  };

  const logout = async () => {
    try {
      await fetchWithAuth("/api/auth/logout", { method: "POST" });
    } catch (e) {
      // Ignore
    }
    setIsAuthenticated(false);
    router.push("/login");
  };

  return (
    <AuthContext.Provider value={{ isAuthenticated, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}

export const fetchWithAuth = async (url: string, options: RequestInit = {}) => {
  // Always include credentials so the browser sends the HttpOnly cookie
  const res = await fetch(url, { ...options, credentials: "include" });
  
  if (res.status === 401) {
    if (typeof window !== "undefined") {
      const publicPaths = ["/login", "/register", "/forgot-password", "/reset-password", "/verify", "/"];
      if (!publicPaths.includes(window.location.pathname)) {
        window.location.href = "/login";
      }
    }
    throw new Error("Unauthorized");
  }
  
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.error || "An error occurred");
  }
  
  return res.json();
};
