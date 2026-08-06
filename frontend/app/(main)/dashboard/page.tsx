"use client";

import useSWR from "swr";
import Link from "next/link";
import { Plus, Server, GitBranch, Activity, Clock, Box } from "lucide-react";
import { formatDistanceToNow } from "date-fns";
import { motion } from "framer-motion";

import { fetchWithAuth } from "@/lib/auth";

const fetcher = (url: string) => fetchWithAuth(url);

interface Environment {
  id: string;
  name: string;
  gitUrl: string;
  githubBranch: string;
  status: string;
  publicUrl: string | null;
  createdAt: string;
}

const statusColors: Record<string, string> = {
  IDLE: "text-gray-400 bg-gray-400/10",
  BUILDING: "text-blue-400 bg-blue-400/10 animate-pulse",
  RUNNING: "text-emerald-400 bg-emerald-400/10",
  STOPPED: "text-orange-400 bg-orange-400/10",
  FAILED: "text-red-400 bg-red-400/10",
};

export default function Dashboard() {
  const { data: environments, error, isLoading } = useSWR<Environment[]>("/api/environments", fetcher, {
    refreshInterval: 3000,
  });

  return (
    <div className="max-w-6xl mx-auto space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-4xl font-bold tracking-tight text-on-surface mb-2">Deployments</h1>
          <p className="text-on-surface-variant text-lg">Manage and monitor your API sandbox environments.</p>
        </div>
        <Link 
          href="/upload" 
          className="bg-primary-container text-on-primary-fixed-variant px-6 py-3 rounded-lg font-bold flex items-center gap-2 hover:shadow-[0_0_15px_rgba(0,240,255,0.2)] active:scale-95 transition-all"
        >
          <span className="material-symbols-outlined text-[20px]">add</span>
          New Sandbox
        </Link>
      </div>

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {[1, 2, 3].map((i) => (
            <div key={i} className="glass-panel h-48 animate-pulse bg-white/5" />
          ))}
        </div>
      ) : error ? (
        <div className="bg-error-container/10 border border-error/20 p-8 rounded-xl text-center text-error">
          Failed to load environments. Ensure the backend is running.
        </div>
      ) : environments?.length === 0 ? (
        <div className="bg-surface-container-lowest border border-outline-variant rounded-xl p-16 text-center flex flex-col items-center justify-center border-dashed">
          <span className="material-symbols-outlined text-[48px] text-on-surface-variant/50 mb-4">inventory_2</span>
          <h3 className="text-xl font-bold text-on-surface mb-2">No deployments yet</h3>
          <p className="text-on-surface-variant mb-8">Create your first sandbox environment to get started.</p>
          <Link href="/upload" className="bg-primary-container text-on-primary-fixed-variant px-8 py-3 rounded-lg font-bold hover:shadow-[0_0_15px_rgba(0,240,255,0.2)] active:scale-95 transition-all">
            Deploy Now
          </Link>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {environments?.map((env, idx) => (
            <motion.div 
              key={env.id}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: idx * 0.05 }}
            >
              <Link href={`/environments/${env.id}`} className="block group">
                <div className="bg-surface-container-lowest border border-outline-variant rounded-xl p-6 h-full transition-all duration-300 hover:border-primary-fixed hover:shadow-[0_0_20px_rgba(0,240,255,0.1)] relative overflow-hidden">
                  <div className="absolute inset-0 bg-gradient-to-br from-primary-fixed/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity"></div>
                  <div className="flex justify-between items-start mb-4 relative z-10">
                    <h3 className="font-bold text-lg text-on-surface group-hover:text-primary-fixed transition-colors truncate pr-4">
                      {env.name}
                    </h3>
                    <span className={`inline-flex items-center gap-1.5 px-2 py-1 rounded-sm text-[10px] font-bold tracking-widest uppercase border ${
                      env.status === 'RUNNING' ? 'border-primary-fixed/20 text-primary-fixed bg-primary-fixed/10' :
                      env.status === 'IDLE' || env.status === 'STOPPED' ? 'border-secondary-fixed/20 text-secondary-fixed bg-secondary-fixed/10' :
                      env.status === 'BUILDING' ? 'border-tertiary-fixed/20 text-tertiary-fixed bg-tertiary-fixed/10' :
                      'border-error/20 text-error bg-error/10'
                    }`}>
                      {env.status === 'RUNNING' && <span className="w-1.5 h-1.5 rounded-full bg-primary-fixed animate-pulse shadow-[0_0_5px_#00f0ff]"></span>}
                      {env.status === 'BUILDING' && <span className="w-1.5 h-1.5 rounded-full bg-tertiary-fixed animate-bounce"></span>}
                      {env.status}
                    </span>
                  </div>
                  
                  <div className="space-y-3 mb-6 relative z-10">
                    <div className="flex items-center text-sm text-on-surface-variant font-mono">
                      <span className="material-symbols-outlined text-[16px] mr-2 text-on-surface-variant/50">account_tree</span>
                      <span className="truncate">{env.gitUrl.replace('https://github.com/', '')}</span>
                      <span className="mx-2 text-outline-variant">•</span>
                      <span className="font-bold text-on-surface">{env.githubBranch}</span>
                    </div>
                    {env.publicUrl && (
                      <div className="flex items-center text-sm text-primary-fixed-dim hover:text-primary-fixed transition-colors font-mono">
                        <span className="material-symbols-outlined text-[16px] mr-2">public</span>
                        <span className="truncate hover:underline flex items-center gap-1">{env.publicUrl} <span className="material-symbols-outlined text-[14px]">open_in_new</span></span>
                      </div>
                    )}
                  </div>

                  <div className="flex items-center justify-between text-[11px] font-bold text-on-surface-variant tracking-wider uppercase pt-4 border-t border-outline-variant/50 relative z-10">
                    <div className="flex items-center">
                      <span className="material-symbols-outlined text-[14px] mr-1.5">schedule</span>
                      {formatDistanceToNow(new Date(env.createdAt), { addSuffix: true })}
                    </div>
                    <span className="material-symbols-outlined opacity-0 group-hover:opacity-100 transition-opacity text-primary-fixed">arrow_forward</span>
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
