"use client";

import { useState, Suspense } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import Link from "next/link";
import toast from "react-hot-toast";

function ResetPasswordForm() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const code = searchParams.get("code");
  
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (newPassword !== confirmPassword) {
      toast.error("Passwords do not match");
      return;
    }
    
    setLoading(true);
    try {
      const res = await fetch("/api/auth/reset-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ code, newPassword }),
      });
      
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Failed to reset password");
      
      toast.success(data.message || "Password successfully reset");
      router.push("/login");
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setLoading(false);
    }
  };

  if (!code) {
    return (
      <div className="max-w-md mx-auto mt-20 bg-surface-container-lowest border border-outline-variant p-8 text-center rounded-2xl shadow-xl">
        <h1 className="text-2xl font-bold text-on-surface mb-4">Invalid Reset Link</h1>
        <p className="text-on-surface-variant mb-6">
          This password reset link is invalid or missing the reset code.
        </p>
        <Link href="/forgot-password" className="bg-primary-container text-on-primary-fixed-variant px-6 py-2.5 rounded-lg font-bold hover:shadow-[0_0_15px_rgba(0,240,255,0.2)] active:scale-95 transition-all inline-block">
          Request New Link
        </Link>
      </div>
    );
  }

  return (
    <div className="max-w-md mx-auto mt-20 bg-surface-container-lowest border border-outline-variant p-8 rounded-2xl shadow-xl">
      <h1 className="text-2xl font-bold text-on-surface mb-2 text-center">Create New Password</h1>
      <p className="text-center text-on-surface-variant text-sm mb-6">
        Please enter your new password below.
      </p>
      
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-bold tracking-wide text-on-surface-variant uppercase mb-1">New Password</label>
          <input
            type="password"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            className="w-full bg-surface-container px-4 py-3 rounded-lg border border-outline-variant text-on-surface focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed transition-all"
            required
            minLength={12}
          />
          <p className="text-xs text-on-surface-variant/70 mt-2 font-mono">
            Must be at least 12 characters and include upper, lower, numbers, and special characters.
          </p>
        </div>
        <div>
          <label className="block text-sm font-bold tracking-wide text-on-surface-variant uppercase mb-1">Confirm New Password</label>
          <input
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            className="w-full bg-surface-container px-4 py-3 rounded-lg border border-outline-variant text-on-surface focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed transition-all"
            required
            minLength={12}
          />
        </div>
        <button
          type="submit"
          disabled={loading}
          className="w-full py-3 mt-6 bg-primary-container text-on-primary-fixed-variant rounded-lg font-bold hover:shadow-[0_0_15px_rgba(0,240,255,0.2)] active:scale-95 transition-all disabled:opacity-50 disabled:pointer-events-none"
        >
          {loading ? "Resetting..." : "Reset Password"}
        </button>
      </form>
    </div>
  );
}

export default function ResetPasswordPage() {
  return (
    <Suspense fallback={<div className="text-center mt-20 text-white">Loading...</div>}>
      <ResetPasswordForm />
    </Suspense>
  );
}
