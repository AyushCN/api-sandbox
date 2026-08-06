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
      <div className="max-w-md mx-auto mt-20 glass-panel p-8 text-center">
        <h1 className="text-2xl font-bold text-white mb-4">Invalid Reset Link</h1>
        <p className="text-white/80 mb-6">
          This password reset link is invalid or missing the reset code.
        </p>
        <Link href="/forgot-password" className="glass-button py-2 px-4">
          Request New Link
        </Link>
      </div>
    );
  }

  return (
    <div className="max-w-md mx-auto mt-20 glass-panel p-8">
      <h1 className="text-2xl font-bold text-white mb-2 text-center">Create New Password</h1>
      <p className="text-center text-white/60 text-sm mb-6">
        Please enter your new password below.
      </p>
      
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-white/80 mb-1">New Password</label>
          <input
            type="password"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            className="glass-input w-full px-4 py-2"
            required
            minLength={12}
          />
          <p className="text-xs text-white/50 mt-1">
            Must be at least 12 characters and include upper, lower, numbers, and special characters.
          </p>
        </div>
        <div>
          <label className="block text-sm font-medium text-white/80 mb-1">Confirm New Password</label>
          <input
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            className="glass-input w-full px-4 py-2"
            required
            minLength={12}
          />
        </div>
        <button
          type="submit"
          disabled={loading}
          className="glass-button w-full py-2 mt-4"
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
