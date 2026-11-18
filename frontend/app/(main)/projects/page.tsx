"use client";

import React, { useState } from "react";
import useSWR from "swr";
import Link from "next/link";
import {
  Folder,
  Users,
  ArrowRight,
  Plus,
  XCircle,
  X,
  Lock,
  Share2,
  UserCheck,
  Check,
  Shield,
  Eye,
  Edit3,
  Layers,
  Sparkles,
} from "lucide-react";
import { fetchWithAuth } from "@/lib/auth";
import toast from "react-hot-toast";
import { motion } from "framer-motion";
import { InviteCollaboratorModal } from "@/components/editor/InviteCollaboratorModal";

const fetcher = (url: string) => fetchWithAuth(url);

interface Invitation {
  id: string;
  projectId: string;
  projectName?: string;
  project?: { name: string; description: string };
  inviterName?: string;
  inviter?: { username: string; email: string };
  role: "EDITOR" | "VIEWER";
  status: string;
  createdAt: string;
}

export default function ProjectsPage() {
  const { data: currentUser } = useSWR<{ id?: string; username?: string; email?: string }>("/api/me", fetcher);
  const {
    data: projects,
    error,
    isLoading,
    mutate,
  } = useSWR("/api/projects", fetcher);

  const {
    data: invitations,
    mutate: mutateInvites,
  } = useSWR<Invitation[]>("/api/user/invitations", fetcher);

  const [activeTab, setActiveTab] = useState<"all" | "personal" | "my" | "shared">("all");
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [newProjectName, setNewProjectName] = useState("");
  const [newProjectDesc, setNewProjectDesc] = useState("");
  const [isCreating, setIsCreating] = useState(false);

  // Invite modal state for quick invite from card
  const [inviteTargetProjectId, setInviteTargetProjectId] = useState<string | null>(null);
  const [isInviting, setIsInviting] = useState(false);

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
          creator: currentUser ? {
            userId: currentUser.id,
            username: currentUser.username,
            email: currentUser.email,
          } : undefined,
        }),
      });
      toast.success("Shareable Collaborative Project created!");
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

  const handleAcceptInvite = async (invitationId: string) => {
    try {
      await fetchWithAuth(`/api/invitations/${invitationId}/accept`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ user: currentUser }),
      });
      toast.success("Invitation accepted! Project added to Shared With Me.");
      mutateInvites();
      mutate();
    } catch (err: unknown) {
      toast.error((err as Error).message || "Failed to accept invitation");
    }
  };

  const handleRejectInvite = async (invitationId: string) => {
    try {
      await fetchWithAuth(`/api/invitations/${invitationId}/reject`, {
        method: "POST",
      });
      toast.success("Invitation declined.");
      mutateInvites();
    } catch (err: unknown) {
      toast.error((err as Error).message || "Failed to decline invitation");
    }
  };

  const handleQuickInvite = async (identifier: string, role: string) => {
    if (!inviteTargetProjectId) return;
    setIsInviting(true);
    try {
      await fetchWithAuth(`/api/projects/${inviteTargetProjectId}/invite`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          identifier,
          role: role === "VIEWER" ? "VIEWER" : "EDITOR",
          inviter: currentUser,
        }),
      });
      toast.success("Collaboration invitation sent!");
      setInviteTargetProjectId(null);
      mutate();
    } catch (err: unknown) {
      toast.error((err as Error).message || "Failed to send invitation");
    } finally {
      setIsInviting(false);
    }
  };

  // Classify projects
  const allProjects = Array.isArray(projects) ? projects : [];

  const defaultProjects = allProjects.filter(
    (p: any) => p.isDefault === true || p.id === "default-project"
  );

  const myProjects = allProjects.filter((p: any) => {
    if (p.isDefault === true || p.id === "default-project") return false;
    if (p.isOwner !== undefined) return p.isOwner === true;
    if (currentUser?.email && p.createdBy === currentUser.email) return true;
    if (Array.isArray(p.collaborators)) {
      const owner = p.collaborators.find((c: any) => c.role === "OWNER");
      if (owner && (owner.userId === currentUser?.id || owner.user?.email === currentUser?.email)) {
        return true;
      }
    }
    return true; // default fallback to owned
  });

  const sharedProjects = allProjects.filter((p: any) => {
    if (p.isDefault === true || p.id === "default-project") return false;
    if (p.isOwner !== undefined) return p.isOwner === false;
    if (Array.isArray(p.collaborators) && currentUser) {
      const myMembership = p.collaborators.find(
        (c: any) => c.role !== "OWNER" && (c.userId === currentUser.id || c.user?.email === currentUser.email)
      );
      if (myMembership) return true;
    }
    return false;
  });

  const pendingInvites = Array.isArray(invitations)
    ? invitations.filter((i) => i.status === "PENDING")
    : [];

  return (
    <div className="space-y-10 pb-16">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-6 pb-4 border-b border-outline-variant">
        <div>
          <div className="flex items-center gap-2 mb-2">
            <span className="w-2 h-2 rounded-full bg-sky-500 dark:bg-sky-400" />
            <span className="text-[11px] font-mono font-bold uppercase tracking-wider text-on-surface-variant">
              Multi-Tenant Workspace & Collaboration
            </span>
          </div>
          <h1 className="text-3xl sm:text-4xl font-extrabold tracking-tight text-on-surface">
            Workspaces & Projects
          </h1>
          <p className="text-sm text-on-surface-variant mt-1">
            Manage your private personal environment and collaborate on shared microservices.
          </p>
        </div>

        <button
          onClick={() => setIsCreateModalOpen(true)}
          className="h-11 px-6 rounded-full bg-gradient-to-r from-[#FF416C] to-[#FF4B2B] text-white text-xs font-bold uppercase tracking-wider hover:opacity-95 transition-all shadow-lg flex items-center gap-2 self-start sm:self-auto cursor-pointer"
        >
          <Plus className="w-4 h-4" />
          <span>New Shareable Project</span>
        </button>
      </div>

      {/* Pending Invitations Banner */}
      {pendingInvites.length > 0 && (
        <motion.div
          initial={{ opacity: 0, y: -10 }}
          animate={{ opacity: 1, y: 0 }}
          className="bg-amber-500/10 border border-amber-500/30 rounded-[24px] p-6 space-y-4"
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2.5">
              <span className="w-2.5 h-2.5 rounded-full bg-amber-400 animate-ping" />
              <h3 className="text-sm font-bold text-on-surface uppercase tracking-wider">
                Collaboration Invitations ({pendingInvites.length} Pending)
              </h3>
            </div>
            <span className="text-[11px] font-mono text-on-surface-variant">
              Access granted upon acceptance
            </span>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {pendingInvites.map((invite) => {
              const projName = invite.projectName || invite.project?.name || "Shared Project";
              const inviter = invite.inviterName || invite.inviter?.username || "Project Owner";
              return (
                <div
                  key={invite.id}
                  className="bg-surface-container-lowest border border-outline-variant rounded-2xl p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-4 shadow-sm"
                >
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="font-bold text-sm text-on-surface">{projName}</span>
                      <span className={`text-[10px] font-mono font-bold px-2 py-0.5 rounded-full uppercase border ${
                        invite.role === "EDITOR"
                          ? "bg-blue-500/10 text-blue-400 border-blue-500/20"
                          : "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                      }`}>
                        {invite.role}
                      </span>
                    </div>
                    <p className="text-xs text-on-surface-variant">
                      Invited by <strong className="text-on-surface">{inviter}</strong>
                    </p>
                  </div>

                  <div className="flex items-center gap-2 self-end sm:self-auto">
                    <button
                      onClick={() => handleRejectInvite(invite.id)}
                      className="px-4 py-1.5 rounded-full text-xs font-semibold text-on-surface-variant hover:text-on-surface hover:bg-surface-container transition-colors"
                    >
                      Decline
                    </button>
                    <button
                      onClick={() => handleAcceptInvite(invite.id)}
                      className="px-4 py-1.5 rounded-full bg-gradient-to-r from-[#FF416C] to-[#FF4B2B] text-white text-xs font-bold hover:opacity-95 shadow-sm transition-all flex items-center gap-1.5"
                    >
                      <Check className="w-3.5 h-3.5" />
                      <span>Accept</span>
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        </motion.div>
      )}

      {/* Navigation Filter Tabs */}
      <div className="flex items-center gap-2 border-b border-outline-variant pb-2 overflow-x-auto">
        <button
          onClick={() => setActiveTab("all")}
          className={`px-4 py-2 rounded-full text-xs font-bold tracking-wider uppercase transition-colors whitespace-nowrap ${
            activeTab === "all"
              ? "bg-on-surface text-surface"
              : "text-on-surface-variant hover:text-on-surface"
          }`}
        >
          All Spaces ({allProjects.length})
        </button>
        <button
          onClick={() => setActiveTab("personal")}
          className={`px-4 py-2 rounded-full text-xs font-bold tracking-wider uppercase transition-colors whitespace-nowrap flex items-center gap-1.5 ${
            activeTab === "personal"
              ? "bg-on-surface text-surface"
              : "text-on-surface-variant hover:text-on-surface"
          }`}
        >
          <Lock className="w-3 h-3" />
          <span>Default Project (Personal)</span>
        </button>
        <button
          onClick={() => setActiveTab("my")}
          className={`px-4 py-2 rounded-full text-xs font-bold tracking-wider uppercase transition-colors whitespace-nowrap flex items-center gap-1.5 ${
            activeTab === "my"
              ? "bg-on-surface text-surface"
              : "text-on-surface-variant hover:text-on-surface"
          }`}
        >
          <Share2 className="w-3 h-3" />
          <span>My Shareable Projects ({myProjects.length})</span>
        </button>
        <button
          onClick={() => setActiveTab("shared")}
          className={`px-4 py-2 rounded-full text-xs font-bold tracking-wider uppercase transition-colors whitespace-nowrap flex items-center gap-1.5 ${
            activeTab === "shared"
              ? "bg-on-surface text-surface"
              : "text-on-surface-variant hover:text-on-surface"
          }`}
        >
          <Users className="w-3 h-3" />
          <span>Shared With Me ({sharedProjects.length})</span>
        </button>
      </div>

      {/* Section 1: Default Project (Personal Space) */}
      {(activeTab === "all" || activeTab === "personal") && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Lock className="w-4 h-4 text-emerald-400" />
              <h2 className="text-base font-bold uppercase tracking-wider text-on-surface">
                Personal Space (Private)
              </h2>
            </div>
            <span className="text-[11px] font-mono text-on-surface-variant">
              Exclusive to you • Sharing disabled
            </span>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {defaultProjects.map((proj: any) => (
              <div
                key={proj.id}
                className="bg-surface-container-lowest border-2 border-emerald-500/30 rounded-[28px] p-6 flex flex-col justify-between shadow-sm relative overflow-hidden group hover:border-emerald-500/60 transition-all"
              >
                <div className="absolute top-4 right-4 flex items-center gap-1 px-2.5 py-1 rounded-full bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 font-mono text-[10px] font-bold uppercase tracking-wider">
                  <Lock className="w-3 h-3" />
                  <span>Personal Space</span>
                </div>

                <div>
                  <div className="w-10 h-10 rounded-xl bg-surface-container border border-outline-variant flex items-center justify-center mb-4 text-emerald-400">
                    <Folder className="w-5 h-5" />
                  </div>

                  <h3 className="font-bold text-lg text-on-surface truncate mb-1">
                    {proj.name}
                  </h3>
                  <p className="text-xs text-on-surface-variant line-clamp-2 leading-relaxed mb-4">
                    Your personal, private development environment. Workspaces and containers here are accessible only to you and cannot be shared.
                  </p>
                </div>

                <div className="pt-4 border-t border-outline-variant flex items-center justify-between text-xs">
                  <span className="text-[11px] font-mono text-on-surface-variant flex items-center gap-1">
                    <Shield className="w-3.5 h-3.5 text-emerald-400" />
                    <span>Private Environment</span>
                  </span>

                  <Link
                    href={`/projects/${proj.id}`}
                    className="flex items-center gap-1 text-emerald-400 font-bold hover:underline"
                  >
                    <span>Open Space</span>
                    <ArrowRight className="w-3.5 h-3.5" />
                  </Link>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Section 2: My Projects (Collaborative Space) */}
      {(activeTab === "all" || activeTab === "my") && (
        <div className="space-y-4 pt-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Share2 className="w-4 h-4 text-sky-400" />
              <h2 className="text-base font-bold uppercase tracking-wider text-on-surface">
                My Projects (Collaborative)
              </h2>
            </div>
            <span className="text-[11px] font-mono text-on-surface-variant">
              Created by you • Role-based sharing enabled
            </span>
          </div>

          {myProjects.length === 0 ? (
            <div className="bg-surface-container-lowest border border-outline-variant rounded-2xl p-8 text-center space-y-2">
              <p className="text-xs text-on-surface-variant">
                You haven't created any collaborative projects yet.
              </p>
              <button
                onClick={() => setIsCreateModalOpen(true)}
                className="text-xs font-bold text-[#FF416C] hover:underline"
              >
                + Create Shareable Project
              </button>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {myProjects.map((proj: any, idx: number) => {
                const memberCount = Array.isArray(proj.collaborators)
                  ? proj.collaborators.length
                  : proj.members ? proj.members.length : 1;

                return (
                  <motion.div
                    key={proj.id}
                    initial={{ opacity: 0, y: 12 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: idx * 0.03, duration: 0.3 }}
                    className="bg-surface-container-lowest border border-outline-variant rounded-[28px] p-6 flex flex-col justify-between transition-all duration-300 hover:border-outline-variant/80 hover:shadow-lg dark:hover:shadow-[0_16px_48px_rgba(0,0,0,0.45)] hover:-translate-y-1"
                  >
                    <div>
                      <div className="flex items-center justify-between mb-4">
                        <div className="w-10 h-10 rounded-xl bg-surface-container border border-outline-variant flex items-center justify-center text-sky-400">
                          <Folder className="w-5 h-5" />
                        </div>
                        <span className="px-2.5 py-0.5 rounded-full bg-sky-500/10 border border-sky-500/20 text-sky-400 font-mono text-[10px] font-bold uppercase">
                          Owner
                        </span>
                      </div>

                      <h3 className="font-bold text-lg text-on-surface truncate mb-1">
                        {proj.name}
                      </h3>
                      <p className="text-xs text-on-surface-variant line-clamp-2 leading-relaxed mb-4">
                        {proj.description || "Collaborative engineering workspace"}
                      </p>
                    </div>

                    <div className="space-y-4">
                      <div className="flex items-center justify-between text-xs text-on-surface-variant font-mono text-[11px] pt-3 border-t border-outline-variant">
                        <span className="flex items-center gap-1.5">
                          <Users className="w-3.5 h-3.5" />
                          <span>{memberCount} {memberCount === 1 ? "Member" : "Members"}</span>
                        </span>
                        <span className="text-emerald-400 font-semibold">Active Collaboration</span>
                      </div>

                      <div className="flex items-center justify-between pt-2">
                        <button
                          onClick={() => setInviteTargetProjectId(proj.id)}
                          className="px-3 py-1.5 rounded-full bg-surface-container border border-outline-variant text-[11px] font-bold text-on-surface hover:border-[#FF416C] transition-colors flex items-center gap-1.5 cursor-pointer"
                        >
                          <Plus className="w-3 h-3 text-[#FF416C]" />
                          <span>Invite Members</span>
                        </button>

                        <Link
                          href={`/projects/${proj.id}`}
                          className="flex items-center gap-1 text-xs font-bold text-on-surface hover:text-[#FF416C] transition-colors"
                        >
                          <span>Manage</span>
                          <ArrowRight className="w-3.5 h-3.5" />
                        </Link>
                      </div>
                    </div>
                  </motion.div>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* Section 3: Shared With Me (Collaborative Space) */}
      {(activeTab === "all" || activeTab === "shared") && (
        <div className="space-y-4 pt-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Users className="w-4 h-4 text-purple-400" />
              <h2 className="text-base font-bold uppercase tracking-wider text-on-surface">
                Shared With Me
              </h2>
            </div>
            <span className="text-[11px] font-mono text-on-surface-variant">
              Shared by platform team members
            </span>
          </div>

          {sharedProjects.length === 0 ? (
            <div className="bg-surface-container-lowest border border-outline-variant rounded-2xl p-8 text-center space-y-2">
              <p className="text-xs text-on-surface-variant">
                No projects have been shared with you yet. Accepted invitations will appear here.
              </p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {sharedProjects.map((proj: any, idx: number) => {
                const userRole = proj.userRole || proj.role || "EDITOR";
                const ownerName = proj.createdBy || "Project Owner";

                return (
                  <motion.div
                    key={proj.id}
                    initial={{ opacity: 0, y: 12 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: idx * 0.03, duration: 0.3 }}
                    className="bg-surface-container-lowest border border-outline-variant rounded-[28px] p-6 flex flex-col justify-between hover:border-purple-500/40 transition-all shadow-sm"
                  >
                    <div>
                      <div className="flex items-center justify-between mb-4">
                        <div className="w-10 h-10 rounded-xl bg-surface-container border border-outline-variant flex items-center justify-center text-purple-400">
                          <Folder className="w-5 h-5" />
                        </div>
                        <span className={`px-2.5 py-0.5 rounded-full font-mono text-[10px] font-bold uppercase border ${
                          userRole === "EDITOR"
                            ? "bg-blue-500/10 text-blue-400 border-blue-500/20"
                            : "bg-amber-500/10 text-amber-400 border-amber-500/20"
                        }`}>
                          {userRole === "EDITOR" ? (
                            <span className="flex items-center gap-1">
                              <Edit3 className="w-3 h-3" /> Editor
                            </span>
                          ) : (
                            <span className="flex items-center gap-1">
                              <Eye className="w-3 h-3" /> Viewer
                            </span>
                          )}
                        </span>
                      </div>

                      <h3 className="font-bold text-lg text-on-surface truncate mb-1">
                        {proj.name}
                      </h3>
                      <p className="text-xs text-on-surface-variant line-clamp-2 leading-relaxed mb-4">
                        {proj.description || "Shared microservices project"}
                      </p>
                    </div>

                    <div className="pt-4 border-t border-outline-variant space-y-3">
                      <div className="flex items-center justify-between text-xs text-on-surface-variant font-mono text-[11px]">
                        <span>Owner: <strong className="text-on-surface">{ownerName}</strong></span>
                        <span>{proj.sandboxIds?.length || 0} Sandboxes</span>
                      </div>

                      <div className="flex items-center justify-end">
                        <Link
                          href={`/projects/${proj.id}`}
                          className="flex items-center gap-1 text-xs font-bold text-purple-400 hover:underline"
                        >
                          <span>Open Shared Project</span>
                          <ArrowRight className="w-3.5 h-3.5" />
                        </Link>
                      </div>
                    </div>
                  </motion.div>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* Quick Invite Modal */}
      {inviteTargetProjectId && (
        <InviteCollaboratorModal
          isOpen={true}
          onClose={() => setInviteTargetProjectId(null)}
          onInvite={handleQuickInvite}
          isInviting={isInviting}
        />
      )}

      {/* Create Modal */}
      {isCreateModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-md p-4">
          <div className="bg-surface-container-lowest border border-outline-variant rounded-[32px] p-8 max-w-md w-full shadow-2xl relative">
            <button
              onClick={() => setIsCreateModalOpen(false)}
              className="absolute top-6 right-6 text-on-surface-variant hover:text-on-surface transition-colors cursor-pointer"
            >
              <X className="w-5 h-5" />
            </button>

            <div className="flex items-center gap-2 mb-1">
              <Share2 className="w-5 h-5 text-sky-400" />
              <h2 className="text-xl font-bold text-on-surface">
                New Shareable Project
              </h2>
            </div>
            <p className="text-xs text-on-surface-variant mb-6">
              Created outside your Default Project. You can invite team members with Editor or Viewer roles.
            </p>

            <form onSubmit={handleCreateProject} className="space-y-4">
              <div>
                <label className="text-xs font-bold uppercase tracking-wider text-on-surface-variant block mb-1.5">
                  Project Name
                </label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Payment Gateway Service"
                  value={newProjectName}
                  onChange={(e) => setNewProjectName(e.target.value)}
                  className="w-full h-11 px-4 rounded-xl bg-surface-container border border-outline-variant text-xs font-mono text-on-surface placeholder:text-on-surface-variant/40 focus:outline-none focus:border-primary-fixed transition-colors"
                />
              </div>

              <div>
                <label className="text-xs font-bold uppercase tracking-wider text-on-surface-variant block mb-1.5">
                  Description (optional)
                </label>
                <textarea
                  rows={3}
                  placeholder="e.g. Microservice backend and database environments for team testing"
                  value={newProjectDesc}
                  onChange={(e) => setNewProjectDesc(e.target.value)}
                  className="w-full p-3 rounded-xl bg-surface-container border border-outline-variant text-xs font-mono text-on-surface placeholder:text-on-surface-variant/40 focus:outline-none focus:border-primary-fixed transition-colors"
                />
              </div>

              <div className="pt-2 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setIsCreateModalOpen(false)}
                  className="px-5 py-2.5 rounded-full text-xs font-bold uppercase tracking-wider text-on-surface-variant hover:text-on-surface transition-colors cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isCreating}
                  className="px-6 py-2.5 rounded-full bg-gradient-to-r from-[#FF416C] to-[#FF4B2B] text-white text-xs font-bold uppercase tracking-wider hover:opacity-95 transition-all shadow-md disabled:opacity-50 cursor-pointer"
                >
                  {isCreating ? "Creating..." : "Create Project"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
