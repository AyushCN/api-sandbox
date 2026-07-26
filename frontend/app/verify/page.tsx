"use client";

import { useEffect, useState } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import Link from "next/link";

import { Suspense } from "react";

function VerifyContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const code = searchParams.get("code");
  
  const [status, setStatus] = useState<"loading" | "success" | "error">("loading");
  const [message, setMessage] = useState("Verifying your email...");

  useEffect(() => {
    if (!code) {
      setStatus("error");
      setMessage("No verification code provided.");
      return;
    }

    const verifyEmail = async () => {
      try {
        const res = await fetch(`/api/auth/verify?code=${code}`);
        const data = await res.json();

        if (res.ok) {
          setStatus("success");
          setMessage(data.message || "Email verified! You can now login.");
        } else {
          setStatus("error");
          setMessage(data.error || "Failed to verify email.");
        }
      } catch (err) {
        setStatus("error");
        setMessage("An error occurred during verification.");
      }
    };

    verifyEmail();
  }, [code]);

  return (
    <div className="flex items-start mt-20 justify-center p-4">
      <div className="w-full max-w-md space-y-8 rounded-2xl bg-surface-container-lowest p-8 text-center shadow-xl border border-outline-variant">
        <h2 className="text-3xl font-bold tracking-tight text-on-surface">
          Email Verification
        </h2>
        
        <div className={`p-4 rounded-lg font-mono text-sm ${status === 'loading' ? 'bg-tertiary-fixed/10 text-tertiary-fixed' : status === 'success' ? 'bg-primary-fixed/10 text-primary-fixed' : 'bg-error/10 text-error'}`}>
          {message}
        </div>

        {status === "success" && (
          <div className="mt-6">
            <Link 
              href="/login" 
              className="inline-flex w-full justify-center rounded-lg bg-primary-container text-on-primary-fixed-variant px-4 py-3 text-sm font-bold shadow-sm hover:shadow-[0_0_15px_rgba(0,240,255,0.2)] active:scale-95 transition-all"
            >
              Go to Login
            </Link>
          </div>
        )}
      </div>
    </div>
  );
}

export default function VerifyPage() {
  return (
    <Suspense fallback={<div className="flex items-start mt-20 justify-center text-on-surface">Loading...</div>}>
      <VerifyContent />
    </Suspense>
  );
}
