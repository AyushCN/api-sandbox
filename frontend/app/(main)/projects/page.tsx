"use client";
import React, { useState } from "react";
import useSWR from "swr";
import Link from "next/link";
import {
  Folder,
  Users,
  ArrowRight,
  Loader2,
  Plus,
  XCircle,
} from "lucide-react";
import { fetchWithAuth } from "@/lib/auth";
import toast from "react-hot-toast";
import { motion } from "framer-motion";

const fetcher = (url: string) => fetchWithAuth(url);

export default function ProjectsPage() {
  const {
    data: projects,
    error,
    isLoading,
    mutate,
  } = useSWR("/api/projects", fetcher);

  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [newProjectName, setNewProjectName] = useState("");
  const [newProjectDesc, setNewProjectDesc] = useState("");
  const [isCreating, setIsCreating] = useState(false);

  const handleCreateProject = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newProjectName.trim()) return;
    setIsCreating(true);
    try {
      await fetchWithAuth("/api/projects", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: newProjectName,
          description: newProjectDesc,
        }),
      });
      toast.success("Workspace Project created successfully!");
      setIsCreateModalOpen(false);
      setNewProjectName("");
      setNewProjectDesc("");
      mutate();
    } catch (err: unknown) {
      toast.error((err as Error).message || "Failed to create project");
    } finally {
      setIsCreating(false);
    }
  };

  return (
    <div className="space-y-8 pb-12">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-primary-fixed/10 border-primary-fixed/20 flex items-center justify-center">
            <Folder className="w-5 h-5 text-primary-fixed" />
          </div>
          <div>
            <h1 className="text-2xl font-bold tracking-tight text-on-surface">
              Workspace Projects
            </h1>
            <p className="text-on-surface-variant text-sm">
              Organize sandboxes and collaborate with your team
            </p>
          </div>
        </div>
        <button
          onClick={() => setIsCreateModalOpen(true)}
          className="flex items-center gap-2 bg-indigo-500 text-white px-5 py-2.5 rounded-xl font-bold text-sm hover:bg-indigo-600 hover:shadow-[0_0_20px_rgba(99,102,241,0.3)] active:scale-95 transition-all"
        >
          <Plus className="w-4 h-4" />
          New Workspace
        </button>
      </div>

      <div className="mt-8">
        {isLoading ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
            {[1, 2, 3].map((i) => (
              <div
                key={i}
                className="bg-surface-container-lowest border border-outline-variant rounded-xl h-40 animate-pulse"
              />
            ))}
          </div>
        ) : error ? (
          <div className="bg-error-container/10 border border-error/20 p-10 rounded-xl text-center">
            <XCircle className="w-10 h-10 text-error mx-auto mb-3" />
            <p className="text-error font-semibold">Failed to load projects</p>
          </div>
        ) : projects?.length === 0 ? (
          <div className="bg-surface-container-lowest border border-outline-variant border-dashed rounded-xl py-24 text-center flex flex-col items-center">
            <div className="w-16 h-16 rounded-2xl bg-primary-fixed/5 border border-primary-fixed/10 flex items-center justify-center mb-5">
              <Folder className="w-8 h-8 text-on-surface-variant/30" />
            </div>
            <h3 className="text-xl font-bold text-on-surface mb-2">
              No projects yet
            </h3>
            <p className="text-on-surface-variant mb-8 max-w-xs">
              Create your first collaborative workspace project.
            </p>
            <button
              onClick={() => setIsCreateModalOpen(true)}
              className="bg-primary-container text-on-primary-fixed-variant hover:shadow-[0_0_20px_rgba(0,240,255,0.2)]"
            >
              New Workspace
            </button>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
            {projects?.map(
              (
                project: {
                  id: string;
                  name: string;
                  description?: string;
                  status: string;
                  environments: { id: string }[];
                },
                idx: number,
              ) => (
                <motion.div
                  key={project.id}
                  initial={{ opacity: 0, y: 16 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ delay: idx * 0.04, duration: 0.35 }}
                >
                  <Link
                    href={`/projects/${project.id}`}
                    className="block group h-full"
                  >
                    <div className="bg-surface-container-lowest border border-outline-variant rounded-xl p-5 h-full flex flex-col gap-4 transition-all duration-300 hover:border-primary-fixed/40 hover:shadow-[0_0_24px_rgba(0,240,255,0.08)] relative overflow-hidden">
                      <div className="absolute inset-0 bg-gradient-to-br from-primary-fixed/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none" />
                      <div className="relative z-10 flex-1 min-w-0">
                        <h3 className="font-bold text-lg text-on-surface group-hover:text-primary-fixed transition-colors truncate mb-1.5">
                          {project.name}
                        </h3>
                        <p className="text-sm text-on-surface-variant line-clamp-2 mb-4">
                          {project.description || "No description provided."}
                        </p>
                      </div>
                      <div className="relative z-10 flex items-center justify-between text-[10px] font-bold text-on-surface-variant tracking-wider uppercase pt-3 border-t border-outline-variant/50">
                        <div className="flex items-center gap-1">
                          <Users className="w-3 h-3" />
                          {project.name === "Default Workspace"
                            ? "Private"
                            : "Team Workspace"}
                        </div>
                        <ArrowRight className="w-3.5 h-3.5 opacity-0 group-hover:opacity-100 text-primary-fixed transition-all group-hover:translate-x-0.5 duration-200" />
                      </div>
                    </div>
                  </Link>
                </motion.div>
              ),
            )}
          </div>
        )}
      </div>

      {isCreateModalOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4"
          style={{ backgroundColor: "rgba(0,0,0,0.75)" }}
        >
          <div className="w-full max-w-md bg-surface-container-lowest rounded-2xl border border-outline-variant shadow-2xl overflow-hidden">
            <div className="flex items-center justify-between px-5 py-4 border-b border-outline-variant bg-surface-container/30">
              <h3 className="font-semibold text-on-surface">
                New Workspace Project
              </h3>
              <button
                onClick={() => setIsCreateModalOpen(false)}
                className="text-on-surface-variant hover:text-on-surface flex items-center justify-center"
              >
                <span className="material-symbols-outlined">close</span>
              </button>
            </div>
            <form onSubmit={handleCreateProject} className="p-5 space-y-4">
              <div>
                <label className="text-sm font-bold tracking-wide text-on-surface-variant uppercase mb-1.5 block">
                  Project Name
                </label>
                <input
                  type="text"
                  required
                  minLength={3}
                  value={newProjectName}
                  onChange={(e) => setNewProjectName(e.target.value)}
                  placeholder="e.g. Backend Services API"
                  className="w-full bg-surface-container px-4 py-3 rounded-lg border border-outline-variant text-on-surface focus:border-primary-fixed focus:ring-primary-fixed transition-all outline-none"
                />
              </div>
              <div>
                <label className="text-sm font-bold tracking-wide text-on-surface-variant uppercase mb-1.5 block">
                  Description (Optional)
                </label>
                <textarea
                  value={newProjectDesc}
                  onChange={(e) => setNewProjectDesc(e.target.value)}
                  placeholder="Briefly describe this workspace..."
                  className="w-full bg-surface-container px-4 py-3 rounded-lg border border-outline-variant text-on-surface focus:border-primary-fixed focus:ring-primary-fixed transition-all h-24 resize-none outline-none"
                />
              </div>
              <div className="pt-2 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setIsCreateModalOpen(false)}
                  className="px-4 py-2 text-sm font-medium text-on-surface-variant hover:text-on-surface"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isCreating || !newProjectName.trim()}
                  className="px-5 py-2 bg-primary-container text-on-primary-fixed-variant hover:shadow-[0_0_20px_rgba(0,240,255,0.2)] hover:brightness-110 disabled:opacity-50 disabled:pointer-events-none flex items-center gap-2 transition-all"
                >
                  {isCreating && <Loader2 className="w-4 h-4 animate-spin" />}
                  Create Workspace
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
