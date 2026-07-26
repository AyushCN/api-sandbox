"use client";

import { useState } from "react";
import Link from "next/link";
import toast from "react-hot-toast";

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [loading, setLoading] = useState(false);
  const [submitted, setSubmitted] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const res = await fetch("/api/auth/forgot-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
      
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Failed to process request");
      
      toast.success(data.message || "Reset link sent");
      setSubmitted(true);
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setLoading(false);
    }
  };

  if (submitted) {
    return (
      <div className="max-w-md mx-auto mt-20 bg-surface-container-lowest border border-outline-variant p-8 text-center rounded-2xl shadow-xl">
        <h1 className="text-2xl font-bold text-on-surface mb-4">Check Your Email</h1>
        <p className="text-on-surface-variant mb-6">
          If an account exists for that email, we've sent a password reset link.
          Please check your spam folder if you don't see it.
        </p>
        <Link href="/login" className="bg-primary-container text-on-primary-fixed-variant px-6 py-2.5 rounded-lg font-bold hover:shadow-[0_0_15px_rgba(0,240,255,0.2)] active:scale-95 transition-all inline-block">
          Back to Login
        </Link>
      </div>
    );
  }

  return (
    <div className="max-w-md mx-auto mt-20 bg-surface-container-lowest border border-outline-variant p-8 rounded-2xl shadow-xl">
      <h1 className="text-2xl font-bold text-on-surface mb-2 text-center">Reset Password</h1>
      <p className="text-center text-on-surface-variant text-sm mb-6">
        Enter your email address and we'll send you a link to reset your password.
      </p>
      
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-bold tracking-wide text-on-surface-variant uppercase mb-1">Email</label>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="w-full bg-surface-container px-4 py-3 rounded-lg border border-outline-variant text-on-surface focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed transition-all"
            required
          />
        </div>
        <button
          type="submit"
          disabled={loading}
          className="w-full py-3 mt-6 bg-primary-container text-on-primary-fixed-variant rounded-lg font-bold hover:shadow-[0_0_15px_rgba(0,240,255,0.2)] active:scale-95 transition-all disabled:opacity-50 disabled:pointer-events-none"
        >
          {loading ? "Sending link..." : "Send Reset Link"}
        </button>
      </form>
      <p className="text-center text-on-surface-variant text-sm mt-6">
        Remember your password? <Link href="/login" className="font-bold text-primary-fixed hover:underline">Log in</Link>
      </p>
    </div>
  );
}
