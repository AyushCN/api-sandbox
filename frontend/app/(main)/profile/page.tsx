"use client";

import { useState } from "react";
import useSWR from "swr";
import toast from "react-hot-toast";
import { formatDistanceToNow, format } from "date-fns";
import {
  User, Mail, ShieldCheck, ShieldAlert, Building2,
  Crown, Users, Calendar, Server, Zap, KeyRound,
  Eye, EyeOff, Loader2, CheckCircle2, AlertCircle,
  Clock, Lock
} from "lucide-react";
import { fetchWithAuth } from "@/lib/auth";

const fetcher = (url: string) => fetchWithAuth(url);

interface UserProfile {
  id: string;
  email: string;
  isEmailVerified: boolean;
  maxEnvironments: number;
  maxBuildsPerHour: number;
  createdAt: string;
  envCount: number;
  orgName: string;
  orgRole: string;
}

function ProgressBar({ value, max, color = "bg-primary-fixed" }: { value: number; max: number; color?: string }) {
  const pct = Math.min(100, Math.round((value / max) * 100));
  const isWarning = pct >= 80;
  const barColor = isWarning ? "bg-orange-400" : color;
  return (
    <div className="w-full">
      <div className="flex justify-between text-xs text-on-surface-variant mb-1.5">
        <span>{value} used</span>
        <span>{max} limit</span>
      </div>
      <div className="w-full bg-surface-container-high rounded-full h-2 overflow-hidden">
        <div
          className={`h-2 rounded-full transition-all duration-700 ${barColor}`}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

function SectionCard({ title, icon: Icon, children }: { title: string; icon: any; children: React.ReactNode }) {
  return (
    <div className="bg-surface-container-lowest border border-outline-variant rounded-xl overflow-hidden">
      <div className="flex items-center gap-2.5 px-5 py-3.5 border-b border-outline-variant bg-surface-container/50">
        <Icon className="w-4 h-4 text-primary-fixed" />
        <h2 className="text-sm font-bold text-on-surface tracking-wide">{title}</h2>
      </div>
      <div className="p-5">{children}</div>
    </div>
  );
}

export default function ProfilePage() {
  const { data: user, error, isLoading } = useSWR<UserProfile>("/api/user/me", fetcher);

  const [currentPw, setCurrentPw] = useState("");
  const [newPw, setNewPw] = useState("");
  const [confirmPw, setConfirmPw] = useState("");
  const [showCurrent, setShowCurrent] = useState(false);
  const [showNew, setShowNew] = useState(false);
  const [isChanging, setIsChanging] = useState(false);

  const initials = user?.email?.slice(0, 2).toUpperCase() ?? "??";
  const memberSince = user ? format(new Date(user.createdAt), "MMMM d, yyyy") : "";
  const memberAgo = user ? formatDistanceToNow(new Date(user.createdAt), { addSuffix: true }) : "";

  const handlePasswordChange = async (e: React.FormEvent) => {
    e.preventDefault();
    if (newPw !== confirmPw) {
      toast.error("New passwords do not match");
      return;
    }
    if (newPw.length < 12) {
      toast.error("Password must be at least 12 characters");
      return;
    }

    setIsChanging(true);
    try {
      const token = localStorage.getItem("token");
      const res = await fetch("/api/user/me/password", {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ currentPassword: currentPw, newPassword: newPw }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Failed to change password");
      toast.success("Password changed successfully!");
      setCurrentPw("");
      setNewPw("");
      setConfirmPw("");
    } catch (e: any) {
      toast.error(e.message);
    } finally {
      setIsChanging(false);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-32">
        <Loader2 className="w-8 h-8 animate-spin text-primary-fixed" />
      </div>
    );
  }

  if (error || !user) {
    return (
      <div className="flex flex-col items-center justify-center py-32 gap-3">
        <AlertCircle className="w-10 h-10 text-error" />
        <p className="text-error font-semibold">Failed to load profile</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center gap-3">
        <div className="w-10 h-10 rounded-xl bg-primary-fixed/10 border border-primary-fixed/20 flex items-center justify-center">
          <User className="w-5 h-5 text-primary-fixed" />
        </div>
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-on-surface">Account</h1>
          <p className="text-on-surface-variant text-sm">Manage your profile and preferences</p>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">

        {/* Left Column: Profile card + Org card */}
        <div className="lg:col-span-1 space-y-5">

          {/* Profile Card */}
          <div className="bg-surface-container-lowest border border-outline-variant rounded-xl p-6 flex flex-col items-center text-center">
            {/* Avatar */}
            <div className="relative mb-4">
              <div className="w-20 h-20 rounded-2xl bg-gradient-to-br from-primary-fixed/30 to-primary-container/20 border-2 border-primary-fixed/30 flex items-center justify-center">
                <span className="text-2xl font-black text-primary-fixed tracking-tight">{initials}</span>
              </div>
              <div className={`absolute -bottom-1.5 -right-1.5 w-6 h-6 rounded-full border-2 border-surface-container-lowest flex items-center justify-center ${user.isEmailVerified ? "bg-emerald-500" : "bg-orange-400"}`}>
                {user.isEmailVerified
                  ? <CheckCircle2 className="w-3.5 h-3.5 text-white" />
                  : <AlertCircle className="w-3.5 h-3.5 text-white" />
                }
              </div>
            </div>

            {/* Email */}
            <div className="flex items-center gap-1.5 mb-1">
              <Mail className="w-3.5 h-3.5 text-on-surface-variant/60" />
              <span className="text-sm font-semibold text-on-surface">{user.email}</span>
            </div>

            {/* Verified badge */}
            <span className={`inline-flex items-center gap-1.5 mt-1 px-2.5 py-1 rounded-full text-[10px] font-bold tracking-wider uppercase border ${
              user.isEmailVerified
                ? "text-emerald-400 bg-emerald-400/10 border-emerald-400/20"
                : "text-orange-400 bg-orange-400/10 border-orange-400/20"
            }`}>
              {user.isEmailVerified ? <ShieldCheck className="w-3 h-3" /> : <ShieldAlert className="w-3 h-3" />}
              {user.isEmailVerified ? "Verified" : "Unverified"}
            </span>

            <div className="w-full border-t border-outline-variant/50 mt-5 pt-4 space-y-2.5 text-left">
              <div className="flex items-center gap-2 text-xs text-on-surface-variant">
                <Calendar className="w-3.5 h-3.5 shrink-0 text-on-surface-variant/50" />
                <span>Joined <span className="text-on-surface font-semibold">{memberSince}</span></span>
              </div>
              <div className="flex items-center gap-2 text-xs text-on-surface-variant">
                <Clock className="w-3.5 h-3.5 shrink-0 text-on-surface-variant/50" />
                <span className="text-on-surface-variant/70 italic">{memberAgo}</span>
              </div>
              <div className="flex items-center gap-2 text-xs text-on-surface-variant font-mono">
                <Lock className="w-3.5 h-3.5 shrink-0 text-on-surface-variant/50" />
                <span className="truncate text-on-surface-variant/60">{user.id.slice(0, 18)}…</span>
              </div>
            </div>
          </div>

          {/* Organization Card */}
          <SectionCard title="Organization" icon={Building2}>
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-lg bg-primary-fixed/10 border border-primary-fixed/20 flex items-center justify-center shrink-0">
                <Building2 className="w-5 h-5 text-primary-fixed" />
              </div>
              <div className="min-w-0">
                <p className="font-semibold text-on-surface text-sm truncate">{user.orgName || "No workspace"}</p>
                <span className={`inline-flex items-center gap-1 mt-1 px-2 py-0.5 rounded-full text-[10px] font-bold tracking-wider uppercase border ${
                  user.orgRole === "ADMIN"
                    ? "text-primary-fixed bg-primary-fixed/10 border-primary-fixed/20"
                    : "text-on-surface-variant bg-surface-container-high border-outline-variant"
                }`}>
                  {user.orgRole === "ADMIN" ? <Crown className="w-2.5 h-2.5" /> : <Users className="w-2.5 h-2.5" />}
                  {user.orgRole || "Member"}
                </span>
              </div>
            </div>
          </SectionCard>
        </div>

        {/* Right Column: Usage + Password */}
        <div className="lg:col-span-2 space-y-5">

          {/* Usage Limits */}
          <SectionCard title="Usage & Limits" icon={Zap}>
            <div className="space-y-6">
              <div>
                <div className="flex items-center gap-2 mb-3">
                  <Server className="w-4 h-4 text-on-surface-variant/60" />
                  <p className="text-sm font-semibold text-on-surface">Sandbox Environments</p>
                </div>
                <ProgressBar value={Number(user.envCount)} max={user.maxEnvironments} />
              </div>

              <div className="border-t border-outline-variant/50 pt-5">
                <div className="flex items-center gap-2 mb-3">
                  <Zap className="w-4 h-4 text-on-surface-variant/60" />
                  <p className="text-sm font-semibold text-on-surface">Max Builds Per Hour</p>
                </div>
                <ProgressBar value={0} max={user.maxBuildsPerHour} color="bg-blue-400" />
                <p className="text-xs text-on-surface-variant/50 mt-2">Build quota resets every 60 minutes</p>
              </div>

              {/* Quick stat pills */}
              <div className="border-t border-outline-variant/50 pt-4 grid grid-cols-3 gap-3">
                {[
                  { label: "Environments", value: user.envCount, icon: Server, color: "text-primary-fixed" },
                  { label: "Env. Limit", value: user.maxEnvironments, icon: Server, color: "text-on-surface-variant" },
                  { label: "Builds/Hr Limit", value: user.maxBuildsPerHour, icon: Zap, color: "text-blue-400" },
                ].map(({ label, value, icon: Icon, color }) => (
                  <div key={label} className="flex flex-col items-center text-center bg-surface-container-low border border-outline-variant/50 rounded-xl p-3">
                    <Icon className={`w-5 h-5 mb-1.5 ${color}`} />
                    <p className="text-lg font-black text-on-surface">{value}</p>
                    <p className="text-[10px] text-on-surface-variant/60 tracking-wide">{label}</p>
                  </div>
                ))}
              </div>
            </div>
          </SectionCard>

          {/* Change Password */}
          <SectionCard title="Change Password" icon={KeyRound}>
            <form onSubmit={handlePasswordChange} className="space-y-4">
              {/* Current Password */}
              <div>
                <label className="block text-xs font-bold text-on-surface-variant mb-1.5 tracking-wide uppercase">Current Password</label>
                <div className="relative">
                  <input
                    type={showCurrent ? "text" : "password"}
                    value={currentPw}
                    onChange={e => setCurrentPw(e.target.value)}
                    required
                    placeholder="Enter current password"
                    className="w-full bg-surface-container border border-outline-variant rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-on-surface-variant/40 focus:outline-none focus:border-primary-fixed/60 transition-colors pr-11"
                  />
                  <button type="button" onClick={() => setShowCurrent(v => !v)} className="absolute right-3 top-1/2 -translate-y-1/2 text-on-surface-variant/40 hover:text-on-surface-variant transition-colors">
                    {showCurrent ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                {/* New Password */}
                <div>
                  <label className="block text-xs font-bold text-on-surface-variant mb-1.5 tracking-wide uppercase">New Password</label>
                  <div className="relative">
                    <input
                      type={showNew ? "text" : "password"}
                      value={newPw}
                      onChange={e => setNewPw(e.target.value)}
                      required
                      placeholder="Min. 12 chars, mixed"
                      className="w-full bg-surface-container border border-outline-variant rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-on-surface-variant/40 focus:outline-none focus:border-primary-fixed/60 transition-colors pr-11"
                    />
                    <button type="button" onClick={() => setShowNew(v => !v)} className="absolute right-3 top-1/2 -translate-y-1/2 text-on-surface-variant/40 hover:text-on-surface-variant transition-colors">
                      {showNew ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    </button>
                  </div>
                </div>

                {/* Confirm Password */}
                <div>
                  <label className="block text-xs font-bold text-on-surface-variant mb-1.5 tracking-wide uppercase">Confirm Password</label>
                  <input
                    type="password"
                    value={confirmPw}
                    onChange={e => setConfirmPw(e.target.value)}
                    required
                    placeholder="Repeat new password"
                    className={`w-full bg-surface-container border rounded-lg px-4 py-2.5 text-sm text-on-surface placeholder:text-on-surface-variant/40 focus:outline-none transition-colors ${
                      confirmPw && confirmPw !== newPw
                        ? "border-error/50 focus:border-error"
                        : "border-outline-variant focus:border-primary-fixed/60"
                    }`}
                  />
                </div>
              </div>

              {/* Password rules hint */}
              <p className="text-[11px] text-on-surface-variant/50 leading-5">
                Must be at least 12 characters and contain uppercase, lowercase, a number, and a special character (!@#$%^&*).
              </p>

              <div className="flex justify-end pt-1">
                <button
                  type="submit"
                  disabled={isChanging || !currentPw || !newPw || !confirmPw}
                  className="flex items-center gap-2 bg-primary-container text-on-primary-fixed-variant px-6 py-2.5 rounded-xl font-bold text-sm hover:shadow-[0_0_20px_rgba(0,240,255,0.2)] active:scale-95 transition-all disabled:opacity-50 disabled:pointer-events-none"
                >
                  {isChanging ? <Loader2 className="w-4 h-4 animate-spin" /> : <KeyRound className="w-4 h-4" />}
                  Update Password
                </button>
              </div>
            </form>
          </SectionCard>
        </div>
      </div>
    </div>
  );
}
