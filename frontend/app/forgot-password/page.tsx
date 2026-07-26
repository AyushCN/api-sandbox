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
      <div className="max-w-md mx-auto mt-20 glass-panel p-8 text-center">
        <h1 className="text-2xl font-bold text-white mb-4">Check Your Email</h1>
        <p className="text-white/80 mb-6">
          If an account exists for that email, we've sent a password reset link.
          Please check your spam folder if you don't see it.
        </p>
        <Link href="/login" className="glass-button py-2 px-4">
          Back to Login
        </Link>
      </div>
    );
  }

  return (
    <div className="max-w-md mx-auto mt-20 glass-panel p-8">
      <h1 className="text-2xl font-bold text-white mb-2 text-center">Reset Password</h1>
      <p className="text-center text-white/60 text-sm mb-6">
        Enter your email address and we'll send you a link to reset your password.
      </p>
      
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-white/80 mb-1">Email</label>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="glass-input w-full px-4 py-2"
            required
          />
        </div>
        <button
          type="submit"
          disabled={loading}
          className="glass-button w-full py-2 mt-4"
        >
          {loading ? "Sending link..." : "Send Reset Link"}
        </button>
      </form>
      <p className="text-center text-white/60 text-sm mt-6">
        Remember your password? <Link href="/login" className="text-primary hover:underline">Log in</Link>
      </p>
    </div>
  );
}
