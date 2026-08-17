"use client";
import React, { use } from "react";
import useSWR from "swr";
import Link from "next/link";
import {
  ArrowLeft,
  Folder,
  Users,
  Loader2,
  GitBranch,
  Clock,
  Box,
  XCircle,
  Code,
  ArrowRight,
} from "lucide-react";
import { formatDistanceToNow } from "date-fns";
import { fetchWithAuth } from "@/lib/auth";
import { motion } from "framer-motion";

const fetcher = (url: string) => fetchWithAuth(url);

interface Environment {
  id: string;
  name: string;
  gitUrl: string;
  githubBranch: string;
  status: string;
  createdAt: string;
}

const statusConfig: Record<
  string,
  { color: string; dot: string; label: string }
> = {
  IDLE: {
    color: "text-gray-400 bg-gray-400/10 border-gray-400/20",
    dot: "bg-gray-400",
    label: "Idle",
  },
  BUILDING: {
    color: "text-blue-400 bg-blue-400/10 border-blue-400/20",
    dot: "bg-blue-400 animate-bounce",
    label: "Building",
  },
  RUNNING: {
    color: "text-emerald-400 bg-emerald-400/10 border-emerald-400/20",
    dot: "bg-emerald-400 animate-pulse shadow-[0_0_6px_#34d399]",
    label: "Running",
  },
  STOPPED: {
    color: "text-orange-400 bg-orange-400/10 border-orange-400/20",
    dot: "bg-orange-400",
    label: "Stopped",
  },
  FAILED: {
    color: "text-red-400 bg-red-400/10 border-red-400/20",
    dot: "bg-red-400",
    label: "Failed",
  },
};

function StatusBadge({ status }: { status: string }) {
  const cfg = statusConfig[status] ?? statusConfig.IDLE;
  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] font-bold tracking-widest uppercase border ${cfg.color}`}
    >
      <span className={`w-1.5 h-1.5 rounded-full ${cfg.dot}`} />
      {cfg.label}
    </span>
  );
}

export default function ProjectDetailsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);

  // Fetch Project Details
  const {
    data: project,
    error: projectError,
    isLoading: projectLoading,
  } = useSWR(`/api/projects/${id}`, fetcher);

  // Fetch Project Sandboxes
  const {
    data: environments,
    error: envError,
    isLoading: envLoading,
  } = useSWR<Environment[]>(`/api/environments?projectId=${id}`, fetcher, {
    refreshInterval: 3000,
  });

  if (projectLoading) {
    return (
      <div className="flex items-center justify-center h-64 text-on-surface-variant">
        <Loader2 className="w-8 h-8 animate-spin" />
      </div>
    );
  }

  if (projectError || !project) {
    return (
      <div className="p-8 text-center text-red-400">
        Failed to load project details.
      </div>
    );
  }

  const isDefaultWorkspace = project.name === "Default Workspace";

  return (
    <div className="flex flex-col gap-8 pb-12">
      {/* Header */}
      <div className="flex items-center gap-4 shrink-0">
        <Link
          href="/projects"
          className="w-10 h-10 rounded-full border border-outline-variant flex items-center justify-center hover:bg-surface-container-high transition-colors"
        >
          <ArrowLeft className="w-5 h-5 text-on-surface-variant" />
        </Link>
        <div className="flex items-center gap-3">
          <div className="w-12 h-12 rounded-xl bg-primary-fixed/10 border border-primary-fixed/20 flex items-center justify-center">
            <Folder className="w-6 h-6 text-primary-fixed" />
          </div>
          <div>
            <h1 className="text-2xl font-bold tracking-tight text-on-surface">
              {project.name}
            </h1>
            <div className="flex items-center gap-2 text-sm text-on-surface-variant mt-1">
              <Users className="w-4 h-4" />
              <span>
                {isDefaultWorkspace ? "Private Workspace" : "Team Workspace"}
              </span>
              {project.description && (
                <>
                  <span className="w-1 h-1 rounded-full bg-on-surface-variant/30" />
                  <span>{project.description}</span>
                </>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Project Sandboxes */}
      <div className="space-y-4">
        <h2 className="text-lg font-bold text-on-surface flex items-center gap-2">
          <Code className="w-5 h-5 text-primary-fixed" />
          Project Sandboxes
        </h2>

        {envLoading ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-5">
            {[1, 2, 3].map((i) => (
              <div
                key={i}
                className="bg-surface-container-lowest border border-outline-variant rounded-xl h-52 animate-pulse"
              />
            ))}
          </div>
        ) : envError ? (
          <div className="bg-error-container/10 border border-error/20 p-8 rounded-xl text-center">
            <XCircle className="w-8 h-8 text-error mx-auto mb-3" />
            <p className="text-error font-semibold">Failed to load sandboxes</p>
          </div>
        ) : environments?.length === 0 ? (
          <div className="bg-surface-container-lowest border border-outline-variant border-dashed rounded-xl py-16 text-center flex flex-col items-center">
            <div className="w-12 h-12 rounded-2xl bg-primary-fixed/5 border border-primary-fixed/10 flex items-center justify-center mb-4">
              <Box className="w-6 h-6 text-on-surface-variant/30" />
            </div>
            <h3 className="text-lg font-bold text-on-surface mb-2">
              No sandboxes found
            </h3>
            <p className="text-on-surface-variant text-sm mb-6 max-w-xs">
              There are no sandboxes in this project yet.
            </p>
            <Link
              href="/upload"
              className="bg-primary-container text-on-primary-fixed-variant px-6 py-2.5 rounded-xl font-bold text-sm hover:shadow-[0_0_20px_rgba(0,240,255,0.2)] active:scale-95 transition-all"
            >
              New Sandbox
            </Link>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-5">
            {environments?.map((env, idx) => (
              <motion.div
                key={env.id}
                initial={{ opacity: 0, y: 16 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: idx * 0.04, duration: 0.35 }}
              >
                <Link
                  href={`/environments/${env.id}`}
                  className="block group h-full"
                >
                  <div className="bg-surface-container-lowest border border-outline-variant rounded-xl p-5 h-full flex flex-col gap-4 transition-all duration-300 hover:border-primary-fixed/40 hover:shadow-[0_0_24px_rgba(0,240,255,0.08)] relative overflow-hidden">
                    <div className="absolute inset-0 bg-gradient-to-br from-primary-fixed/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none" />
                    <div className="flex items-start justify-between gap-3 relative z-10">
                      <div className="w-9 h-9 rounded-lg bg-primary-fixed/10 border border-primary-fixed/20 flex items-center justify-center shrink-0">
                        <Box className="w-4 h-4 text-primary-fixed" />
                      </div>
                      <StatusBadge status={env.status} />
                    </div>
                    <div className="relative z-10 flex-1 min-w-0">
                      <h3 className="font-bold text-base text-on-surface group-hover:text-primary-fixed transition-colors truncate mb-1.5">
                        {env.name}
                      </h3>
                      <div className="flex items-center gap-1.5 text-xs text-on-surface-variant font-mono">
                        <GitBranch className="w-3.5 h-3.5 shrink-0 text-on-surface-variant/50" />
                        <span className="truncate">
                          {env.gitUrl.replace("https://github.com/", "")}
                        </span>
                      </div>
                    </div>
                    <div className="relative z-10 flex items-center justify-between text-[10px] font-bold text-on-surface-variant tracking-wider uppercase pt-3 border-t border-outline-variant/50">
                      <div className="flex items-center gap-1">
                        <Clock className="w-3 h-3" />
                        {formatDistanceToNow(new Date(env.createdAt), {
                          addSuffix: true,
                        })}
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
    </div>
  );
}
