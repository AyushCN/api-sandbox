"use client";
import React, { use } from "react";
import useSWR from "swr";
import Link from "next/link";
import { ArrowLeft, Folder, Users, Loader2 } from "lucide-react";
import { fetchWithAuth } from "@/lib/auth";
import TeamCollaborationDashboard from "@/components/TeamCollaborationDashboard";

const fetcher = (url: string) => fetchWithAuth(url);

export default function ProjectDetailsPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { data: project, error, isLoading } = useSWR(`/api/projects/${id}`, fetcher);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64 text-on-surface-variant">
        <Loader2 className="w-8 h-8 animate-spin" />
      </div>
    );
  }

  if (error || !project) {
    return (
      <div className="p-8 text-center text-red-400">
        Failed to load project details.
      </div>
    );
  }

  return (
    <div className="flex flex-col h-[calc(100vh-8rem)]">
      {/* Header */}
      <div className="flex items-center gap-4 mb-6 shrink-0">
        <Link 
          href="/projects"
          className="w-10 h-10 rounded-full border border-outline-variant flex items-center justify-center hover:bg-surface-container-high transition-colors"
        >
          <ArrowLeft className="w-5 h-5 text-on-surface-variant" />
        </Link>
        <div className="flex items-center gap-3">
          <div className="w-12 h-12 rounded-xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center">
            <Folder className="w-6 h-6 text-indigo-400" />
          </div>
          <div>
            <h1 className="text-2xl font-bold tracking-tight text-on-surface">{project.name}</h1>
            <div className="flex items-center gap-2 text-sm text-on-surface-variant mt-1">
              <Users className="w-4 h-4" />
              <span>{project.name === "Default Workspace" ? "Private Workspace" : "Team Workspace"}</span>
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

      {/* Collaboration Dashboard Component */}
      <div className="bg-surface-container-lowest border border-outline-variant rounded-xl overflow-hidden flex-1 flex flex-col min-h-0">
        <TeamCollaborationDashboard projectId={id} />
      </div>
    </div>
  );
}
