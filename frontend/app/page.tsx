"use client";

import useSWR from "swr";
import Link from "next/link";
import { Plus, Server, GitBranch, Activity, Clock, Box } from "lucide-react";
import { formatDistanceToNow } from "date-fns";
import { motion } from "framer-motion";

const fetcher = (url: string) => fetch(url).then((res) => res.json());

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
          <h1 className="text-3xl font-bold tracking-tight text-white mb-2">Deployments</h1>
          <p className="text-white/60">Manage and monitor your API sandbox environments.</p>
        </div>
        <Link 
          href="/upload" 
          className="glass-button px-6 py-2.5 flex items-center gap-2"
        >
          <Plus className="w-4 h-4" />
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
        <div className="glass-panel p-8 text-center text-red-400">
          Failed to load environments. Ensure the backend is running.
        </div>
      ) : environments?.length === 0 ? (
        <div className="glass-panel p-16 text-center flex flex-col items-center justify-center border-dashed">
          <Box className="w-12 h-12 text-white/20 mb-4" />
          <h3 className="text-xl font-medium text-white mb-2">No deployments yet</h3>
          <p className="text-white/50 mb-6">Create your first sandbox environment to get started.</p>
          <Link href="/upload" className="glass-button px-6 py-2">
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
                <div className="glass-panel p-6 h-full transition-all duration-300 hover:bg-white/10 hover:border-white/20">
                  <div className="flex justify-between items-start mb-4">
                    <h3 className="font-semibold text-lg text-white group-hover:text-primary transition-colors truncate pr-4">
                      {env.name}
                    </h3>
                    <span className={`px-2.5 py-1 rounded-full text-xs font-medium tracking-wide flex-shrink-0 ${statusColors[env.status] || statusColors.IDLE}`}>
                      {env.status}
                    </span>
                  </div>
                  
                  <div className="space-y-3 mb-6">
                    <div className="flex items-center text-sm text-white/60">
                      <GitBranch className="w-4 h-4 mr-2 text-white/40" />
                      <span className="truncate">{env.gitUrl.replace('https://github.com/', '')}</span>
                      <span className="mx-2 text-white/20">•</span>
                      <span>{env.githubBranch}</span>
                    </div>
                    {env.publicUrl && (
                      <div className="flex items-center text-sm text-primary">
                        <Activity className="w-4 h-4 mr-2" />
                        <span className="truncate hover:underline">{env.publicUrl}</span>
                      </div>
                    )}
                  </div>

                  <div className="flex items-center text-xs text-white/40 pt-4 border-t border-white/5">
                    <Clock className="w-3.5 h-3.5 mr-1.5" />
                    {formatDistanceToNow(new Date(env.createdAt), { addSuffix: true })}
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
