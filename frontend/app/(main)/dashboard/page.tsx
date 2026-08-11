"use client";
import React, { useState } from "react";

import useSWR, { mutate } from "swr";
import Link from "next/link";
import NewDeploymentModal from "@/components/NewDeploymentModal";
import { 
  Plus, Server, GitBranch, Clock, Box, 
  ExternalLink, Activity, Zap, XCircle, 
  PauseCircle, Loader2, ArrowRight, LayoutDashboard
} from "lucide-react";
import { formatDistanceToNow } from "date-fns";
import { motion } from "framer-motion";

import { fetchWithAuth } from "@/lib/auth";
import toast from "react-hot-toast";

const fetcher = (url: string) => fetchWithAuth(url);

interface Deployment {
  id: string;
  name: string;
  gitUrl: string;
  githubBranch: string;
  status: string;
  publicUrl: string | null;
  createdAt: string;
}

interface ProjectCollaborator {
  id: string;
  projectId: string;
  role: string;
  project: {
    id: string;
    name: string;
    description: string;
  };
}

const statusConfig: Record<string, { color: string; dot: string; label: string }> = {
  IDLE:     { color: "text-gray-400 bg-gray-400/10 border-gray-400/20",    dot: "bg-gray-400",    label: "Idle" },
  BUILDING: { color: "text-blue-400 bg-blue-400/10 border-blue-400/20",    dot: "bg-blue-400 animate-bounce",  label: "Building" },
  RUNNING:  { color: "text-emerald-400 bg-emerald-400/10 border-emerald-400/20", dot: "bg-emerald-400 animate-pulse shadow-[0_0_6px_#34d399]", label: "Running" },
  STOPPED:  { color: "text-orange-400 bg-orange-400/10 border-orange-400/20", dot: "bg-orange-400", label: "Stopped" },
  FAILED:   { color: "text-red-400 bg-red-400/10 border-red-400/20",        dot: "bg-red-400",     label: "Failed" },
};

function StatusBadge({ status }: { status: string }) {
  const cfg = statusConfig[status] ?? statusConfig.IDLE;
  return (
    <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] font-bold tracking-widest uppercase border ${cfg.color}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${cfg.dot}`} />
      {cfg.label}
    </span>
  );
}

function StatCard({ label, value, icon: Icon, color }: { label: string; value: number; icon: any; color: string }) {
  return (
    <div className={`flex items-center gap-4 bg-surface-container-lowest border border-outline-variant rounded-xl px-5 py-4 relative overflow-hidden group hover:border-primary-fixed/30 transition-colors`}>
      <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${color} shrink-0`}>
        <Icon className="w-5 h-5" />
      </div>
      <div>
        <p className="text-2xl font-bold text-on-surface">{value}</p>
        <p className="text-xs text-on-surface-variant font-medium">{label}</p>
      </div>
      <div className={`absolute inset-0 bg-gradient-to-br ${color.replace('bg-', 'from-').replace('/20', '/5')} to-transparent opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none`} />
    </div>
  );
}

export default function Dashboard() {
  const { data: deployments, error, isLoading } = useSWR<Deployment[]>("/api/deployments", fetcher, {
    refreshInterval: 3000,
  });

  const stats = {
    total:    deployments?.length ?? 0,
    running:  deployments?.filter(e => e.status === "RUNNING").length ?? 0,
    building: deployments?.filter(e => e.status === "BUILDING").length ?? 0,
    failed:   deployments?.filter(e => e.status === "FAILED").length ?? 0,
  };

  const { data: invites, mutate: mutateInvites } = useSWR<ProjectCollaborator[]>("/api/user/invites", fetcher);
  const [isProcessingInvite, setIsProcessingInvite] = useState<string | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);

  const handleInviteAction = async (projectId: string, action: 'accept' | 'decline') => {
    setIsProcessingInvite(projectId);
    try {
      const token = localStorage.getItem("token");
      const res = await fetch(`/api/projects/${projectId}/invites/${action}`, {
        method: "POST",
        credentials: "include"
      });
      if (!res.ok) throw new Error(`Failed to ${action} invite`);
      
      toast.success(`Invite ${action}ed successfully`);
      mutateInvites();
      if (action === 'accept') {
        // Refresh deployments to show new project's sandboxes
        mutate("/api/deployments");
      }
    } catch (e: any) {
      toast.error(e.message);
    } finally {
      setIsProcessingInvite(null);
    }
  };

  return (
    <div className="space-y-8">

      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-primary-fixed/10 border border-primary-fixed/20 flex items-center justify-center">
            <LayoutDashboard className="w-5 h-5 text-primary-fixed" />
          </div>
          <div>
            <h1 className="text-2xl font-bold tracking-tight text-on-surface">Deployments</h1>
            <p className="text-on-surface-variant text-sm">Manage and monitor your sandbox deployments</p>
          </div>
        </div>
        <button
          onClick={() => setIsModalOpen(true)}
          className="flex items-center gap-2 bg-primary-container text-on-primary-fixed-variant px-5 py-2.5 rounded-xl font-bold text-sm hover:shadow-[0_0_20px_rgba(0,240,255,0.25)] active:scale-95 transition-all"
        >
          <Plus className="w-4 h-4" />
          New Deployment
        </button>
      </div>

      <NewDeploymentModal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        onDeploy={(d) => {
          mutate("/api/deployments");
        }}
      />

      {/* Pending Invites Section */}
      {invites && invites.length > 0 && (
        <div className="bg-primary-fixed/5 border border-primary-fixed/20 rounded-xl overflow-hidden mb-8">
          <div className="bg-primary-fixed/10 px-5 py-3 border-b border-primary-fixed/20 flex items-center gap-2">
            <Zap className="w-4 h-4 text-primary-fixed" />
            <h2 className="font-bold text-primary-fixed text-sm">Pending Project Invites</h2>
          </div>
          <div className="divide-y divide-outline-variant/30">
            {invites.map(invite => (
              <div key={invite.id} className="p-5 flex items-center justify-between">
                <div>
                  <h3 className="font-bold text-on-surface text-lg">{invite.project.name}</h3>
                  <p className="text-sm text-on-surface-variant mt-1">You have been invited to join this project as a <strong>{invite.role}</strong>.</p>
                </div>
                <div className="flex gap-3">
                  <button
                    onClick={() => handleInviteAction(invite.projectId, 'decline')}
                    disabled={isProcessingInvite === invite.projectId}
                    className="px-4 py-2 rounded-lg font-semibold text-sm border border-error/30 text-error hover:bg-error/10 disabled:opacity-50 transition-colors"
                  >
                    Decline
                  </button>
                  <button
                    onClick={() => handleInviteAction(invite.projectId, 'accept')}
                    disabled={isProcessingInvite === invite.projectId}
                    className="px-4 py-2 rounded-lg font-semibold text-sm bg-primary-fixed text-on-primary-fixed hover:bg-primary-fixed/90 disabled:opacity-50 transition-colors flex items-center gap-2"
                  >
                    {isProcessingInvite === invite.projectId && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                    Accept Invite
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Stats Bar */}
      {!isLoading && !error && (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <StatCard label="Total Sandboxes" value={stats.total}    icon={Server}   color="bg-primary-fixed/10 text-primary-fixed" />
          <StatCard label="Running"         value={stats.running}  icon={Activity} color="bg-emerald-400/10 text-emerald-400" />
          <StatCard label="Building"        value={stats.building} icon={Zap}      color="bg-blue-400/10 text-blue-400" />
          <StatCard label="Failed"          value={stats.failed}   icon={XCircle}  color="bg-red-400/10 text-red-400" />
        </div>
      )}

      {/* Environments Grid */}
      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-5">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="bg-surface-container-lowest border border-outline-variant rounded-xl h-52 animate-pulse" />
          ))}
        </div>
      ) : error ? (
        <div className="bg-error-container/10 border border-error/20 p-10 rounded-xl text-center">
          <XCircle className="w-10 h-10 text-error mx-auto mb-3" />
          <p className="text-error font-semibold">Failed to load deployments</p>
          <p className="text-on-surface-variant text-sm mt-1">Ensure the backend is running.</p>
        </div>
      ) : deployments?.length === 0 ? (
        <div className="bg-surface-container-lowest border border-outline-variant border-dashed rounded-xl py-24 text-center flex flex-col items-center">
          <div className="w-16 h-16 rounded-2xl bg-primary-fixed/5 border border-primary-fixed/10 flex items-center justify-center mb-5">
            <Server className="w-8 h-8 text-on-surface-variant/30" />
          </div>
          <h3 className="text-xl font-bold text-on-surface mb-2">No deployments yet</h3>
          <p className="text-on-surface-variant mb-8 max-w-xs">Create your first sandbox environment to start deploying.</p>
          <Link href="/upload" className="bg-primary-container text-on-primary-fixed-variant px-8 py-3 rounded-xl font-bold hover:shadow-[0_0_20px_rgba(0,240,255,0.2)] active:scale-95 transition-all">
            Deploy Now
          </Link>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-5">
          {deployments?.map((env, idx) => (
            <motion.div
              key={env.id}
              initial={{ opacity: 0, y: 16 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.04, duration: 0.35 }}
            >
              <Link href={`/deployments/${env.id}`} className="block group h-full">
                <div className="bg-surface-container-lowest border border-outline-variant rounded-xl p-5 h-full flex flex-col gap-4 transition-all duration-300 hover:border-primary-fixed/40 hover:shadow-[0_0_24px_rgba(0,240,255,0.08)] relative overflow-hidden">
                  
                  {/* Hover glow */}
                  <div className="absolute inset-0 bg-gradient-to-br from-primary-fixed/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none" />
                  
                  {/* Card Header */}
                  <div className="flex items-start justify-between gap-3 relative z-10">
                    <div className="w-9 h-9 rounded-lg bg-primary-fixed/10 border border-primary-fixed/20 flex items-center justify-center shrink-0">
                      <Box className="w-4 h-4 text-primary-fixed" />
                    </div>
                    <StatusBadge status={env.status} />
                  </div>

                  {/* Name + Repo */}
                  <div className="relative z-10 flex-1 min-w-0">
                    <h3 className="font-bold text-base text-on-surface group-hover:text-primary-fixed transition-colors truncate mb-1.5">
                      {env.name}
                    </h3>
                    <div className="flex items-center gap-1.5 text-xs text-on-surface-variant font-mono">
                      <GitBranch className="w-3.5 h-3.5 shrink-0 text-on-surface-variant/50" />
                      <span className="truncate">{env.gitUrl.replace("https://github.com/", "")}</span>
                    </div>
                    <div className="flex items-center gap-1.5 text-xs text-on-surface-variant mt-1">
                      <span className="text-on-surface-variant/40">on</span>
                      <span className="font-bold text-on-surface">{env.githubBranch}</span>
                    </div>
                  </div>

                  {/* Public URL */}
                  {env.publicUrl && (
                    <div className="relative z-10 flex items-center gap-1.5 text-xs text-primary-fixed-dim hover:text-primary-fixed transition-colors font-mono truncate">
                      <ExternalLink className="w-3 h-3 shrink-0" />
                      <span className="truncate">{env.publicUrl}</span>
                    </div>
                  )}

                  {/* Footer */}
                  <div className="relative z-10 flex items-center justify-between text-[10px] font-bold text-on-surface-variant tracking-wider uppercase pt-3 border-t border-outline-variant/50">
                    <div className="flex items-center gap-1">
                      <Clock className="w-3 h-3" />
                      {formatDistanceToNow(new Date(env.createdAt), { addSuffix: true })}
                    </div>
                    <ArrowRight className="w-3.5 h-3.5 opacity-0 group-hover:opacity-100 text-primary-fixed transition-all group-hover:translate-x-0.5 duration-200" />
                  </div>
                </div>
              </Link>
            </motion.div>
          ))}
        </div>
      )}
    </div>
  );
}
