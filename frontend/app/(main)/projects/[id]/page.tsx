"use client";
import React, { use, useState } from "react";
import useSWR from "swr";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  ArrowLeft,
  Folder,
  Users,
  Loader2,
  GitBranch,
  Box,
  XCircle,
  Code,
  ArrowRight,
  GitMerge,
  Play,
  Database,
  AlertCircle,
  CheckCircle2,
  CircleDot,
  Plus,
  Lock,
  Share2,
  Crown,
  ShieldCheck,
  Eye,
  Clock,
  Trash2,
  Copy,
  Edit3,
  Send,
  Search,
  RefreshCw,
  UserPlus,
  Shield,
  X,
  Check,
} from "lucide-react";
import { formatDistanceToNow } from "date-fns";
import toast from "react-hot-toast";
import { fetchWithAuth } from "@/lib/auth";
import { motion } from "framer-motion";
import { InviteCollaboratorModal } from "@/components/editor/InviteCollaboratorModal";

const fetcher = (url: string) => fetchWithAuth(url);

type Tab = "overview" | "repositories" | "members" | "workspaces" | "environments" | "change-requests";

// ── Domain types ───────────────────────────────────────────────────────────────

interface Project {
  id: string;
  name: string;
  description?: string;
  createdAt?: string;
}

interface ProjectMember {
  id: string;
  userId: string;
  role: string;
  status: string;
  user?: { username?: string; email?: string };
  acceptedAt?: string;
  createdAt?: string;
  expiresAt?: string;
  inviteId?: string;
  inviterName?: string;
}

interface ProjectRepository {
  id: string;
  name?: string;
  gitUrl: string;
  defaultBranch?: string;
}

interface WorkspaceRepository {
  id: string;
  branch: string;
  currentCommit?: string;
  workingDirectory?: string;
}

interface Workspace {
  id: string;
  type: string;
  environmentId?: string;
  environment?: { status: string };
  repositories?: WorkspaceRepository[];
}

interface Environment {
  id: string;
  name: string;
  status: string;
  createdAt?: string;
}

interface ChangeRequest {
  id: string;
  title: string;
  description?: string;
  status: string;
  createdAt?: string;
}

// ── Status helpers ─────────────────────────────────────────────────────────────

const envStatusConfig: Record<string, { color: string; dot: string; label: string }> = {
  IDLE:     { color: "text-gray-400 bg-gray-400/10 border-gray-400/20",        dot: "bg-gray-400",                                   label: "Idle"     },
  BUILDING: { color: "text-blue-400 bg-blue-400/10 border-blue-400/20",        dot: "bg-blue-400 animate-bounce",                    label: "Building" },
  RUNNING:  { color: "text-emerald-400 bg-emerald-400/10 border-emerald-400/20", dot: "bg-emerald-400 animate-pulse shadow-[0_0_6px_#34d399]", label: "Running" },
  STOPPED:  { color: "text-orange-400 bg-orange-400/10 border-orange-400/20",  dot: "bg-orange-400",                                 label: "Stopped"  },
  FAILED:   { color: "text-red-400 bg-red-400/10 border-red-400/20",           dot: "bg-red-400",                                    label: "Failed"   },
};

function StatusBadge({ status }: { status: string }) {
  const cfg = envStatusConfig[status] ?? envStatusConfig.IDLE;
  return (
    <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] font-bold tracking-widest uppercase border ${cfg.color}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${cfg.dot}`} />
      {cfg.label}
    </span>
  );
}

const roleConfig: Record<string, { color: string; label: string }> = {
  OWNER:  { color: "text-amber-400 bg-amber-400/10 border-amber-400/20",   label: "Owner"  },
  EDITOR: { color: "text-blue-400 bg-blue-400/10 border-blue-400/20",      label: "Editor" },
  VIEWER: { color: "text-gray-400 bg-gray-400/10 border-gray-400/20",      label: "Viewer" },
};

function RoleBadge({ role }: { role: string }) {
  const cfg = roleConfig[role] ?? roleConfig.VIEWER;
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-bold tracking-widest uppercase border ${cfg.color}`}>
      {cfg.label}
    </span>
  );
}

const crStatusConfig: Record<string, { color: string; icon: React.ElementType; label: string }> = {
  OPEN:       { color: "text-blue-400 bg-blue-400/10 border-blue-400/20",       icon: CircleDot,     label: "Open"       },
  APPROVED:   { color: "text-emerald-400 bg-emerald-400/10 border-emerald-400/20", icon: CheckCircle2, label: "Approved"   },
  MERGED:     { color: "text-purple-400 bg-purple-400/10 border-purple-400/20", icon: GitMerge,      label: "Merged"     },
  CONFLICTED: { color: "text-red-400 bg-red-400/10 border-red-400/20",          icon: AlertCircle,   label: "Conflicted" },
  REJECTED:   { color: "text-gray-400 bg-gray-400/10 border-gray-400/20",       icon: XCircle,       label: "Rejected"   },
};

function CRStatusBadge({ status }: { status: string }) {
  const cfg = crStatusConfig[status] ?? crStatusConfig.OPEN;
  const Icon = cfg.icon;
  return (
    <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] font-bold tracking-widest uppercase border ${cfg.color}`}>
      <Icon className="w-3 h-3" />
      {cfg.label}
    </span>
  );
}

// ── Main component ─────────────────────────────────────────────────────────────

export default function ProjectDetailsPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const [activeTab, setActiveTab] = useState<Tab>("overview");
  const [isOpening, setIsOpening] = useState(false);

  const handleOpenWorkspace = async () => {
    setIsOpening(true);
    try {
      const res = await fetch(`/api/projects/${id}/edit`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${localStorage.getItem("token")}`
        }
      });
      const data = await res.json().catch(() => ({}));
      if (res.ok && data.environmentId) {
        router.push(`/environments/${data.environmentId}`);
      } else {
        router.push(`/upload?projectId=${id}`);
      }
    } catch {
      router.push(`/upload?projectId=${id}`);
    } finally {
      setIsOpening(false);
    }
  };

  const { data: projectData, error: projectError, isLoading: projectLoading } = useSWR<Project>(`/api/projects/${id}`, fetcher);
  const fallbackProject: Project = {
    id,
    name: id === "default-project" ? "Default Project" : id.replace(/^proj-/, "").replace(/-/g, " ").replace(/\b\w/g, (l) => l.toUpperCase()),
    description: "Active engineering workspace",
    createdAt: new Date().toISOString(),
  };
  const project = projectData || fallbackProject;

  const { data: members, mutate: mutateMembers } = useSWR<ProjectMember[]>(`/api/projects/${id}/members`, fetcher, { refreshInterval: 10000 });
  const { data: currentUser } = useSWR<{ id?: string; username?: string; email?: string }>("/api/me", fetcher);
  const { data: repositories, mutate: mutateRepositories } = useSWR<ProjectRepository[]>(`/api/projects/${id}/repositories`, fetcher);
  const { data: workspaces }   = useSWR<Workspace[]>(`/api/projects/${id}/workspaces`, fetcher, { refreshInterval: 5000 });
  const { data: changeRequests } = useSWR<ChangeRequest[]>(`/api/projects/${id}/change-requests`, fetcher, { refreshInterval: 5000 });
  const { data: envsRaw } = useSWR<any>(`/api/environments?projectId=${id}`, fetcher, { refreshInterval: 3000 });
  const environments: Environment[] = (Array.isArray(envsRaw)
    ? envsRaw
    : envsRaw?.sandboxes || []).map((sb: any) => ({
      id: sb.id,
      name: sb.name || sb.subdomain || `sandbox-${sb.id.slice(0, 8)}`,
      status: (sb.status || "IDLE").toUpperCase(),
      createdAt: sb.createdAt || sb.created_at || new Date().toISOString(),
      gitUrl: sb.gitUrl || sb.url || "",
      githubBranch: sb.githubBranch || sb.branch || "master",
    }));

  if (projectLoading && !projectData) {
    return (
      <div className="flex items-center justify-center h-64 text-on-surface-variant">
        <Loader2 className="w-8 h-8 animate-spin" />
      </div>
    );
  }

  const openCRs = Array.isArray(changeRequests) ? changeRequests.filter((cr: ChangeRequest) => cr.status === "OPEN").length : 0;

  const isDefaultProject = id === "default-project" || (project as any)?.isDefault === true;
  const isOwner = isDefaultProject || Boolean(
    currentUser && (
      (project as any)?.createdBy === currentUser.email ||
      (project as any)?.createdBy === currentUser.username ||
      members?.some(
        (m) =>
          (m.role === "OWNER" || m.role === "ADMIN") &&
          (m.userId === currentUser.id ||
            m.user?.email === currentUser.email ||
            m.user?.username === currentUser.username)
      )
    )
  );

  const allTabs: { id: Tab; label: string; icon: React.ElementType; count?: number }[] = [
    { id: "overview",        label: "Overview",        icon: Folder    },
    { id: "repositories",    label: "Repositories",    icon: Database,  count: Array.isArray(repositories) ? repositories.length : undefined },
    { id: "members",         label: "Members",         icon: Users,     count: Array.isArray(members) ? members.length : undefined },
    { id: "workspaces",      label: "Workspaces",      icon: Box,       count: Array.isArray(workspaces) ? workspaces.length : undefined },
    { id: "environments",    label: "Environments",    icon: Play,      count: Array.isArray(environments) ? environments.length : undefined },
    { id: "change-requests", label: "Change Requests", icon: GitMerge,  count: openCRs || undefined },
  ];

  const tabs = isDefaultProject ? allTabs.filter(t => t.id !== "members") : allTabs;

  return (
    <div className="flex flex-col gap-6 pb-12">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Link href="/projects" className="w-10 h-10 rounded-full border border-outline-variant flex items-center justify-center hover:bg-surface-container-high transition-colors">
          <ArrowLeft className="w-5 h-5 text-on-surface-variant" />
        </Link>
        <div className="flex items-center gap-3 flex-1 min-w-0">
          <div className="w-12 h-12 rounded-xl bg-primary-fixed/10 border border-primary-fixed/20 flex items-center justify-center shrink-0">
            <Folder className="w-6 h-6 text-primary-fixed" />
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h1 className="text-2xl font-bold tracking-tight text-on-surface truncate">{project.name}</h1>
              {isDefaultProject ? (
                <span className="px-2.5 py-0.5 rounded-full bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 font-mono text-[10px] font-bold uppercase tracking-wider flex items-center gap-1">
                  <Lock className="w-3 h-3" /> Personal Space
                </span>
              ) : (
                <span className="px-2.5 py-0.5 rounded-full bg-sky-500/10 border border-sky-500/20 text-sky-400 font-mono text-[10px] font-bold uppercase tracking-wider flex items-center gap-1">
                  <Share2 className="w-3 h-3" /> Collaborative Space
                </span>
              )}
            </div>
            {project.description && <p className="text-sm text-on-surface-variant truncate">{project.description}</p>}
          </div>
        </div>
        <div className="flex items-center gap-3 shrink-0">
          <Link
            href={`/upload?projectId=${id}`}
            className="flex items-center gap-1.5 px-4 py-2.5 rounded-xl text-xs font-bold uppercase tracking-wider bg-surface-container-high text-on-surface hover:bg-surface-container-highest border border-outline-variant transition-all shadow-sm"
          >
            <Plus className="w-4 h-4 text-[#FF416C]" />
            <span>New Sandbox</span>
          </Link>
          <button 
            onClick={handleOpenWorkspace}
            disabled={isOpening}
            className="flex items-center gap-2 bg-gradient-to-r from-[#FF416C] to-[#FF4B2B] text-white px-5 py-2.5 rounded-xl font-bold text-sm hover:opacity-95 active:scale-95 transition-all disabled:opacity-50 shadow-md"
          >
            {isOpening ? <Loader2 className="w-4 h-4 animate-spin" /> : <Code className="w-4 h-4" />}
            Open Workspace
          </button>
        </div>
      </div>

      {/* Default Project notice */}
      {isDefaultProject && (
        <div className="bg-emerald-500/10 border border-emerald-500/20 rounded-2xl p-4 flex items-center gap-3 text-xs text-on-surface">
          <Lock className="w-4 h-4 text-emerald-400 shrink-0" />
          <div>
            <strong>Personal Default Space:</strong> All workspaces, environments, and containers in this project are strictly private to you. Collaboration and sharing controls are permanently disabled for personal spaces.
          </div>
        </div>
      )}

      {/* Tab Navigation */}
      <div className="flex items-center gap-1 border-b border-outline-variant overflow-x-auto">
        {tabs.map((tab) => {
          const Icon = tab.icon;
          const isActive = activeTab === tab.id;
          return (
            <button
              key={tab.id}
              id={`tab-${tab.id}`}
              onClick={() => setActiveTab(tab.id)}
              className={`flex items-center gap-2 px-4 py-3 text-sm font-semibold whitespace-nowrap border-b-2 transition-all duration-200 ${
                isActive
                  ? "border-primary-fixed text-primary-fixed"
                  : "border-transparent text-on-surface-variant hover:text-on-surface hover:border-outline-variant"
              }`}
            >
              <Icon className="w-4 h-4" />
              {tab.label}
              {tab.count != null && (
                <span className={`ml-1 px-1.5 py-0.5 rounded-full text-[10px] font-bold ${isActive ? "bg-primary-fixed/20 text-primary-fixed" : "bg-surface-container text-on-surface-variant"}`}>
                  {tab.count}
                </span>
              )}
            </button>
          );
        })}
      </div>

      {/* Tab Content */}
      <motion.div key={activeTab} initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.2 }}>
        {activeTab === "overview"        && <OverviewTab project={project} members={members} repositories={repositories} workspaces={workspaces} openCRs={openCRs} />}
        {activeTab === "repositories"    && <RepositoriesTab repositories={repositories} projectId={id} mutateRepositories={mutateRepositories} />}
        {activeTab === "members"         && (
          <MembersTab
            members={members}
            projectId={id}
            projectName={project.name}
            isOwner={isOwner}
            isDefaultProject={isDefaultProject}
            currentUser={currentUser}
            mutateMembers={mutateMembers}
          />
        )}
        {activeTab === "workspaces"      && <WorkspacesTab workspaces={workspaces} projectId={id} />}
        {activeTab === "environments"    && <EnvironmentsTab environments={environments} projectId={id} />}
        {activeTab === "change-requests" && <ChangeRequestsTab changeRequests={changeRequests} />}
      </motion.div>
    </div>
  );
}

// ── Overview Tab ───────────────────────────────────────────────────────────────

function OverviewTab({ project, members, repositories, workspaces, openCRs }: {
  project: Project;
  members?: ProjectMember[];
  repositories?: ProjectRepository[];
  workspaces?: Workspace[];
  openCRs: number;
}) {
  const stats = [
    { label: "Repositories",    value: Array.isArray(repositories) ? repositories.length : "–", icon: Database },
    { label: "Members",         value: Array.isArray(members) ? members.length : "–",            icon: Users    },
    { label: "Workspaces",      value: Array.isArray(workspaces) ? workspaces.length : "–",      icon: Box      },
    { label: "Open Change Reqs",value: openCRs,                                                  icon: GitMerge },
  ];

  return (
    <div className="space-y-6">
      {/* Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        {stats.map((s, i) => {
          const Icon = s.icon;
          return (
            <motion.div key={s.label} initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: i * 0.05 }}
              className="bg-surface-container-lowest border border-outline-variant rounded-xl p-5 flex items-center gap-4"
            >
              <div className="w-10 h-10 rounded-lg bg-primary-fixed/10 border border-primary-fixed/20 flex items-center justify-center shrink-0">
                <Icon className="w-5 h-5 text-primary-fixed" />
              </div>
              <div>
                <p className="text-2xl font-bold text-on-surface">{s.value}</p>
                <p className="text-xs text-on-surface-variant font-medium">{s.label}</p>
              </div>
            </motion.div>
          );
        })}
      </div>

      {/* Project Info */}
      <div className="bg-surface-container-lowest border border-outline-variant rounded-xl p-6">
        <h3 className="text-sm font-bold text-on-surface-variant uppercase tracking-wider mb-4">Project Details</h3>
        <dl className="space-y-3">
          <div className="flex items-center justify-between">
            <dt className="text-sm text-on-surface-variant">Project ID</dt>
            <dd className="text-sm font-mono text-on-surface">{project.id}</dd>
          </div>
          <div className="flex items-center justify-between">
            <dt className="text-sm text-on-surface-variant">Created</dt>
            <dd className="text-sm text-on-surface">{project.createdAt ? formatDistanceToNow(new Date(project.createdAt), { addSuffix: true }) : "–"}</dd>
          </div>
          {project.description && (
            <div className="flex items-start justify-between gap-4">
              <dt className="text-sm text-on-surface-variant shrink-0">Description</dt>
              <dd className="text-sm text-on-surface text-right">{project.description}</dd>
            </div>
          )}
        </dl>
      </div>
    </div>
  );
}

// ── Repositories Tab ───────────────────────────────────────────────────────────

function RepositoriesTab({ repositories, projectId, mutateRepositories }: { repositories?: ProjectRepository[], projectId: string, mutateRepositories: () => void }) {
  const [isAddOpen, setIsAddOpen] = useState(false);
  const [name, setName] = useState("");
  const [gitUrl, setGitUrl] = useState("");
  const [defaultBranch, setDefaultBranch] = useState("main");
  const [isAdding, setIsAdding] = useState(false);

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name || !gitUrl) return;
    setIsAdding(true);
    try {
      const res = await fetch(`/api/projects/${projectId}/repositories`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${localStorage.getItem("token")}` },
        body: JSON.stringify({ name, gitUrl, defaultBranch })
      });
      if (res.ok) {
        setIsAddOpen(false);
        setName("");
        setGitUrl("");
        setDefaultBranch("main");
        mutateRepositories();
      } else {
        const errorData = await res.json();
        alert(errorData.error || "Failed to add repository");
      }
    } finally {
      setIsAdding(false);
    }
  };

  if (!Array.isArray(repositories)) return <LoadingPlaceholder />;

  return (
    <div className="space-y-3">
      <div className="flex justify-between items-center pb-2">
        <h2 className="text-sm font-bold tracking-wide text-on-surface-variant uppercase">Project Repositories</h2>
        <button
          onClick={() => setIsAddOpen(true)}
          className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-primary-fixed/10 text-primary-fixed hover:bg-primary-fixed/20 text-sm font-semibold transition-colors border border-primary-fixed/20"
        >
          <Plus className="w-4 h-4" /> Add Repository
        </button>
      </div>

      {isAddOpen && (
        <form onSubmit={handleAdd} className="bg-surface-container-lowest border border-outline-variant rounded-xl p-5 mb-4">
          <div className="space-y-4">
            <div>
              <label className="text-xs font-bold text-on-surface-variant uppercase mb-1 block">Name</label>
              <input type="text" required value={name} onChange={e => setName(e.target.value)} className="w-full bg-surface-container border border-outline-variant rounded-lg px-3 py-2 text-sm text-on-surface focus:outline-none focus:border-primary-fixed" placeholder="frontend-app" />
            </div>
            <div>
              <label className="text-xs font-bold text-on-surface-variant uppercase mb-1 block">Git URL</label>
              <input type="text" required value={gitUrl} onChange={e => setGitUrl(e.target.value)} className="w-full bg-surface-container border border-outline-variant rounded-lg px-3 py-2 text-sm text-on-surface focus:outline-none focus:border-primary-fixed" placeholder="https://github.com/user/repo" />
            </div>
            <div>
              <label className="text-xs font-bold text-on-surface-variant uppercase mb-1 block">Default Branch</label>
              <input type="text" value={defaultBranch} onChange={e => setDefaultBranch(e.target.value)} className="w-full bg-surface-container border border-outline-variant rounded-lg px-3 py-2 text-sm text-on-surface focus:outline-none focus:border-primary-fixed" placeholder="main" />
            </div>
            <div className="flex justify-end gap-3 pt-2">
              <button type="button" onClick={() => setIsAddOpen(false)} className="px-4 py-2 text-sm font-semibold text-on-surface-variant hover:text-on-surface">Cancel</button>
              <button type="submit" disabled={isAdding} className="px-4 py-2 text-sm font-semibold bg-primary-fixed text-on-primary-fixed rounded-lg hover:bg-primary-fixed/90 disabled:opacity-50 flex items-center gap-2">
                {isAdding && <Loader2 className="w-4 h-4 animate-spin" />}
                Add Repository
              </button>
            </div>
          </div>
        </form>
      )}

      {repositories.length === 0 && !isAddOpen ? (
        <EmptyState icon={Database} title="No repositories" description="Add a Git repository to this project to get started." />
      ) : (
        repositories.map((repo: ProjectRepository, idx: number) => (
          <motion.div key={repo.id} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: idx * 0.04 }}
            className="bg-surface-container-lowest border border-outline-variant rounded-xl p-5 flex items-center gap-4"
          >
            <div className="w-10 h-10 rounded-lg bg-primary-fixed/10 border border-primary-fixed/20 flex items-center justify-center shrink-0">
              <GitBranch className="w-5 h-5 text-primary-fixed" />
            </div>
            <div className="flex-1 min-w-0">
              <p className="font-semibold text-on-surface truncate">{repo.name || repo.gitUrl}</p>
              <p className="text-xs font-mono text-on-surface-variant truncate">{repo.gitUrl}</p>
            </div>
            <div className="flex items-center gap-2 shrink-0">
              <span className="text-xs text-on-surface-variant font-mono bg-surface-container px-2 py-1 rounded-lg">
                {repo.defaultBranch || "main"}
              </span>
            </div>
          </motion.div>
        ))
      )}
    </div>
  );
}

// ── Members Tab ────────────────────────────────────────────────────────────────

interface MembersTabProps {
  members?: ProjectMember[];
  projectId: string;
  projectName: string;
  isOwner: boolean;
  isDefaultProject: boolean;
  currentUser?: { id?: string; username?: string; email?: string };
  mutateMembers: () => void;
}

function MembersTab({
  members,
  projectId,
  projectName,
  isOwner,
  isDefaultProject,
  currentUser,
  mutateMembers,
}: MembersTabProps) {
  const [isInviteOpen, setIsInviteOpen] = useState(false);
  const [isInviting, setIsInviting] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [roleFilter, setRoleFilter] = useState<"ALL" | "OWNER" | "EDITOR" | "VIEWER" | "PENDING">("ALL");

  // Confirmation Modals State
  const [roleChangeModal, setRoleChangeModal] = useState<{
    member: ProjectMember;
    targetRole: "EDITOR" | "VIEWER";
  } | null>(null);

  const [removeModal, setRemoveModal] = useState<ProjectMember | null>(null);
  const [revokeModal, setRevokeModal] = useState<ProjectMember | null>(null);
  const [isSubmittingAction, setIsSubmittingAction] = useState(false);

  // Invite handler
  const handleInvite = async (identifier: string, role: string) => {
    setIsInviting(true);
    try {
      const res = await fetch(`/api/projects/${projectId}/invite`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${localStorage.getItem("token")}`,
        },
        body: JSON.stringify({
          identifier,
          role: role === "VIEWER" ? "VIEWER" : "EDITOR",
          inviter: currentUser
            ? {
                userId: currentUser.id,
                username: currentUser.username,
                email: currentUser.email,
              }
            : undefined,
        }),
      });

      const data = await res.json().catch(() => ({}));
      if (res.ok) {
        setIsInviteOpen(false);
        toast.success(`Invitation sent to ${identifier}`);
        mutateMembers();
      } else {
        toast.error(data.error || "Failed to invite user");
        throw new Error(data.error || "Failed to invite user");
      }
    } finally {
      setIsInviting(false);
    }
  };

  // Change Role handler
  const handleConfirmRoleChange = async () => {
    if (!roleChangeModal) return;
    const { member, targetRole } = roleChangeModal;
    const memberId = member.userId || member.id;
    setIsSubmittingAction(true);
    try {
      const res = await fetch(`/api/projects/${projectId}/collaborators/${memberId}`, {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${localStorage.getItem("token")}`,
        },
        body: JSON.stringify({ role: targetRole }),
      });
      const data = await res.json().catch(() => ({}));
      if (res.ok) {
        toast.success(`Role updated to ${targetRole} for ${member.user?.username || member.user?.email}`);
        mutateMembers();
        setRoleChangeModal(null);
      } else {
        toast.error(data.error || "Failed to update member role");
      }
    } catch {
      toast.error("Network error while updating role");
    } finally {
      setIsSubmittingAction(false);
    }
  };

  // Remove Member handler
  const handleConfirmRemoveMember = async () => {
    if (!removeModal) return;
    const memberId = removeModal.userId || removeModal.id;
    setIsSubmittingAction(true);
    try {
      const res = await fetch(`/api/projects/${projectId}/collaborators/${memberId}`, {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${localStorage.getItem("token")}`,
        },
      });
      const data = await res.json().catch(() => ({}));
      if (res.ok) {
        toast.success(`Removed ${removeModal.user?.username || removeModal.user?.email} from project`);
        mutateMembers();
        setRemoveModal(null);
      } else {
        toast.error(data.error || "Failed to remove member");
      }
    } catch {
      toast.error("Network error while removing member");
    } finally {
      setIsSubmittingAction(false);
    }
  };

  // Resend Invite handler
  const handleResendInvite = async (member: ProjectMember) => {
    const inviteId = member.inviteId || member.id;
    try {
      const res = await fetch(`/api/projects/${projectId}/invites/${inviteId}/resend`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${localStorage.getItem("token")}`,
        },
      });
      const data = await res.json().catch(() => ({}));
      if (res.ok) {
        toast.success(`Invitation resent to ${member.user?.username || member.user?.email}`);
        mutateMembers();
      } else {
        toast.error(data.error || "Failed to resend invitation");
      }
    } catch {
      toast.error("Network error while resending invitation");
    }
  };

  // Revoke Invite handler
  const handleConfirmRevokeInvite = async () => {
    if (!revokeModal) return;
    const inviteId = revokeModal.inviteId || revokeModal.id;
    setIsSubmittingAction(true);
    try {
      const res = await fetch(`/api/projects/${projectId}/invites/${inviteId}/revoke`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${localStorage.getItem("token")}`,
        },
      });
      const data = await res.json().catch(() => ({}));
      if (res.ok) {
        toast.success(`Revoked invitation for ${revokeModal.user?.username || revokeModal.user?.email}`);
        mutateMembers();
        setRevokeModal(null);
      } else {
        toast.error(data.error || "Failed to revoke invitation");
      }
    } catch {
      toast.error("Network error while revoking invitation");
    } finally {
      setIsSubmittingAction(false);
    }
  };

  if (!Array.isArray(members)) return <LoadingPlaceholder />;

  // Personal space isolation guard banner
  if (isDefaultProject) {
    return (
      <div className="space-y-4">
        <div className="p-5 rounded-2xl bg-primary-fixed/10 border border-primary-fixed/30 flex items-start gap-4 shadow-sm">
          <div className="w-10 h-10 rounded-xl bg-primary-fixed/20 border border-primary-fixed/30 flex items-center justify-center text-primary-fixed shrink-0">
            <Shield className="w-5 h-5" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h3 className="font-bold text-sm uppercase tracking-wider text-primary-fixed">
                Default Project (Personal Space)
              </h3>
              <span className="text-[10px] bg-primary-fixed/20 text-primary-fixed px-2 py-0.5 rounded-full font-bold">
                Private Space
              </span>
            </div>
            <p className="text-xs text-on-surface-variant mt-1 leading-relaxed">
              This environment is part of your private Default Project and is owned exclusively by you (
              <strong>{currentUser?.username || "You"}</strong>). It cannot be shared with collaborators. To share containers and invite teammates with role-based access, create a Shareable Project from the Projects section.
            </p>
          </div>
        </div>
      </div>
    );
  }

  // Summary counts
  const totalCount = members.length;
  const ownerCount = members.filter((m) => m.role === "OWNER").length;
  const editorCount = members.filter((m) => m.role === "EDITOR" || m.role === "COLLABORATOR" || m.role === "ADMIN").length;
  const viewerCount = members.filter((m) => m.role === "VIEWER").length;
  const pendingCount = members.filter((m) => m.status === "PENDING" || !m.acceptedAt).length;

  // Filtered members
  const filteredMembers = members.filter((m) => {
    const q = searchQuery.toLowerCase().trim();
    const uname = (m.user?.username || "").toLowerCase();
    const email = (m.user?.email || "").toLowerCase();
    const role = (m.role || "").toLowerCase();

    const matchesSearch = !q || uname.includes(q) || email.includes(q) || role.includes(q);
    if (!matchesSearch) return false;

    if (roleFilter === "ALL") return true;
    if (roleFilter === "PENDING") return m.status === "PENDING" || !m.acceptedAt;
    if (roleFilter === "OWNER") return m.role === "OWNER";
    if (roleFilter === "EDITOR") return m.role === "EDITOR" || m.role === "COLLABORATOR" || m.role === "ADMIN";
    if (roleFilter === "VIEWER") return m.role === "VIEWER";
    return true;
  });

  return (
    <div className="space-y-6">
      {/* Header & Action Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-outline-variant/60">
        <div>
          <div className="flex items-center gap-2">
            <Users className="w-5 h-5 text-emerald-500" />
            <h2 className="text-base font-bold text-on-surface">Project Members & Access Control</h2>
          </div>
          <p className="text-xs text-on-surface-variant mt-0.5">
            Manage project member permissions, roles (Owner, Editor, Viewer), and collaboration invitations.
          </p>
        </div>

        <div className="flex items-center gap-2.5">
          <button
            onClick={() => {
              mutateMembers();
              toast.success("Members list refreshed");
            }}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface-container hover:bg-surface-container-high border border-outline-variant text-on-surface-variant hover:text-on-surface transition-colors text-xs font-semibold cursor-pointer"
            title="Refresh list"
          >
            <RefreshCw className="w-3.5 h-3.5" />
            Refresh
          </button>

          {isOwner && (
            <button
              onClick={() => setIsInviteOpen(true)}
              className="flex items-center gap-2 px-4 py-2 rounded-xl bg-emerald-500 hover:bg-emerald-600 text-white text-xs font-bold transition-all shadow-md shadow-emerald-500/20 cursor-pointer"
            >
              <UserPlus className="w-4 h-4" />
              Invite Members
            </button>
          )}
        </div>
      </div>

      {/* Summary Stats Row */}
      <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
        <div className="bg-surface-container-lowest border border-outline-variant/60 rounded-xl p-3.5 flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg bg-primary-fixed/10 border border-primary-fixed/20 flex items-center justify-center text-primary-fixed shrink-0">
            <Users className="w-4 h-4" />
          </div>
          <div>
            <div className="text-[11px] font-medium text-on-surface-variant uppercase tracking-wider">Total</div>
            <div className="text-lg font-bold text-on-surface leading-tight">{totalCount}</div>
          </div>
        </div>

        <div className="bg-surface-container-lowest border border-outline-variant/60 rounded-xl p-3.5 flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg bg-amber-500/10 border border-amber-500/20 flex items-center justify-center text-amber-500 shrink-0">
            <Crown className="w-4 h-4" />
          </div>
          <div>
            <div className="text-[11px] font-medium text-on-surface-variant uppercase tracking-wider">Owners</div>
            <div className="text-lg font-bold text-on-surface leading-tight">{ownerCount}</div>
          </div>
        </div>

        <div className="bg-surface-container-lowest border border-outline-variant/60 rounded-xl p-3.5 flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-500 shrink-0">
            <ShieldCheck className="w-4 h-4" />
          </div>
          <div>
            <div className="text-[11px] font-medium text-on-surface-variant uppercase tracking-wider">Editors</div>
            <div className="text-lg font-bold text-on-surface leading-tight">{editorCount}</div>
          </div>
        </div>

        <div className="bg-surface-container-lowest border border-outline-variant/60 rounded-xl p-3.5 flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg bg-slate-500/10 border border-slate-500/20 flex items-center justify-center text-slate-400 shrink-0">
            <Eye className="w-4 h-4" />
          </div>
          <div>
            <div className="text-[11px] font-medium text-on-surface-variant uppercase tracking-wider">Viewers</div>
            <div className="text-lg font-bold text-on-surface leading-tight">{viewerCount}</div>
          </div>
        </div>

        <div className="bg-surface-container-lowest border border-outline-variant/60 rounded-xl p-3.5 flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg bg-orange-500/10 border border-orange-500/20 flex items-center justify-center text-orange-500 shrink-0">
            <Clock className="w-4 h-4" />
          </div>
          <div>
            <div className="text-[11px] font-medium text-on-surface-variant uppercase tracking-wider">Pending</div>
            <div className="text-lg font-bold text-on-surface leading-tight">{pendingCount}</div>
          </div>
        </div>
      </div>

      {/* Search & Filter Toolbar */}
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 bg-surface-container/30 p-2.5 rounded-xl border border-outline-variant/50">
        <div className="relative flex-1 max-w-md">
          <Search className="w-4 h-4 text-on-surface-variant/50 absolute left-3 top-1/2 -translate-y-1/2" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search by email, username or role..."
            className="w-full pl-9 pr-3 py-1.5 text-xs rounded-lg bg-surface-container border border-outline-variant text-on-surface placeholder:text-on-surface-variant/40 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition-all font-medium"
          />
          {searchQuery && (
            <button
              onClick={() => setSearchQuery("")}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-[10px] text-on-surface-variant hover:text-on-surface"
            >
              Clear
            </button>
          )}
        </div>

        <div className="flex items-center gap-1.5 overflow-x-auto pb-1 sm:pb-0">
          {(["ALL", "OWNER", "EDITOR", "VIEWER", "PENDING"] as const).map((filter) => {
            const isActive = roleFilter === filter;
            return (
              <button
                key={filter}
                onClick={() => setRoleFilter(filter)}
                className={`px-3 py-1 rounded-lg text-xs font-semibold transition-all whitespace-nowrap cursor-pointer ${
                  isActive
                    ? "bg-emerald-500/15 text-emerald-500 border border-emerald-500/30"
                    : "text-on-surface-variant hover:text-on-surface hover:bg-surface-container border border-transparent"
                }`}
              >
                {filter === "ALL" ? "All Roles" : filter.charAt(0) + filter.slice(1).toLowerCase()}
              </button>
            );
          })}
        </div>
      </div>

      {/* Members Management Table */}
      <div className="bg-surface-container-lowest border border-outline-variant rounded-xl overflow-hidden shadow-sm">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="bg-surface-container/40 border-b border-outline-variant/60 text-[11px] font-bold uppercase tracking-wider text-on-surface-variant">
                <th className="py-3 px-4">Member</th>
                <th className="py-3 px-4">Email</th>
                <th className="py-3 px-4">Assigned Role</th>
                <th className="py-3 px-4">Status</th>
                <th className="py-3 px-4">Joined / Invited</th>
                <th className="py-3 px-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-outline-variant/40 text-xs">
              {filteredMembers.length === 0 ? (
                <tr>
                  <td colSpan={6} className="py-12 text-center text-on-surface-variant">
                    <Users className="w-8 h-8 mx-auto mb-2 opacity-30" />
                    <p className="font-semibold text-sm">No members found</p>
                    <p className="text-xs text-on-surface-variant/70 mt-1">
                      {searchQuery || roleFilter !== "ALL"
                        ? "Try clearing your search query or switching filters."
                        : "Invite teammates to collaborate with Editor or Viewer roles."}
                    </p>
                  </td>
                </tr>
              ) : (
                filteredMembers.map((m) => {
                  const memberUserId = m.userId || m.id;
                  const isMe = currentUser && (
                    currentUser.id === memberUserId ||
                    currentUser.email === m.user?.email ||
                    currentUser.username === m.user?.username
                  );
                  const isTargetOwner = m.role === "OWNER";
                  const isPending = m.status === "PENDING" || !m.acceptedAt;

                  // Role badge config
                  const roleConfig: Record<string, { label: string; icon: any; badgeClass: string; avatarBorder: string }> = {
                    OWNER: {
                      label: "Owner",
                      icon: Crown,
                      badgeClass: "bg-amber-500/10 text-amber-500 border-amber-500/30",
                      avatarBorder: "border-amber-500/40 bg-amber-500/10 text-amber-500",
                    },
                    EDITOR: {
                      label: "Editor",
                      icon: ShieldCheck,
                      badgeClass: "bg-emerald-500/10 text-emerald-500 border-emerald-500/30",
                      avatarBorder: "border-emerald-500/40 bg-emerald-500/10 text-emerald-500",
                    },
                    COLLABORATOR: {
                      label: "Editor",
                      icon: ShieldCheck,
                      badgeClass: "bg-emerald-500/10 text-emerald-500 border-emerald-500/30",
                      avatarBorder: "border-emerald-500/40 bg-emerald-500/10 text-emerald-500",
                    },
                    ADMIN: {
                      label: "Admin",
                      icon: ShieldCheck,
                      badgeClass: "bg-purple-500/10 text-purple-500 border-purple-500/30",
                      avatarBorder: "border-purple-500/40 bg-purple-500/10 text-purple-500",
                    },
                    VIEWER: {
                      label: "Viewer",
                      icon: Eye,
                      badgeClass: "bg-slate-500/10 text-slate-400 border-slate-500/30",
                      avatarBorder: "border-slate-500/40 bg-slate-500/10 text-slate-400",
                    },
                  };

                  const currentRoleCfg = roleConfig[m.role] || roleConfig.VIEWER;
                  const RoleIcon = currentRoleCfg.icon;

                  const rawDate = isPending ? m.createdAt : m.acceptedAt || m.createdAt;
                  const formattedDate = rawDate
                    ? formatDistanceToNow(new Date(rawDate), { addSuffix: true })
                    : "–";

                  return (
                    <tr key={m.id || memberUserId} className="hover:bg-surface-container/20 transition-colors">
                      {/* Member */}
                      <td className="py-3 px-4">
                        <div className="flex items-center gap-3">
                          <div
                            className={`w-9 h-9 rounded-full border-2 flex items-center justify-center font-bold text-xs shrink-0 ${currentRoleCfg.avatarBorder}`}
                          >
                            {(m.user?.username || m.user?.email || "?")[0]?.toUpperCase()}
                          </div>
                          <div>
                            <div className="flex items-center gap-1.5 font-semibold text-on-surface">
                              <span>{m.user?.username || "Collaborator"}</span>
                              {isMe && (
                                <span className="text-[10px] bg-primary-fixed/20 text-primary-fixed px-1.5 py-0.2 rounded-full font-bold">
                                  You
                                </span>
                              )}
                            </div>
                            {isPending && m.inviterName && (
                              <span className="text-[10px] text-on-surface-variant/60 block">
                                Invited by {m.inviterName}
                              </span>
                            )}
                          </div>
                        </div>
                      </td>

                      {/* Email */}
                      <td className="py-3 px-4 text-on-surface-variant">
                        <div className="flex items-center gap-1.5 max-w-[200px]">
                          <span className="truncate">{m.user?.email || "–"}</span>
                          {m.user?.email && (
                            <button
                              onClick={() => {
                                navigator.clipboard.writeText(m.user?.email || "");
                                toast.success("Email copied to clipboard");
                              }}
                              className="text-on-surface-variant/40 hover:text-on-surface transition-colors cursor-pointer"
                              title="Copy email"
                            >
                              <Copy className="w-3 h-3" />
                            </button>
                          )}
                        </div>
                      </td>

                      {/* Role */}
                      <td className="py-3 px-4">
                        <span
                          className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold border ${currentRoleCfg.badgeClass}`}
                        >
                          <RoleIcon className="w-3 h-3" />
                          <span>{currentRoleCfg.label}</span>
                        </span>
                      </td>

                      {/* Status */}
                      <td className="py-3 px-4">
                        {isPending ? (
                          <span className="inline-flex items-center gap-1 text-[11px] bg-amber-500/15 text-amber-500 px-2 py-0.5 rounded-full font-semibold border border-amber-500/30">
                            <Clock className="w-3 h-3 animate-spin" />
                            Pending Invite
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1 text-[11px] bg-emerald-500/15 text-emerald-500 px-2 py-0.5 rounded-full font-semibold border border-emerald-500/30">
                            <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse" />
                            Active
                          </span>
                        )}
                      </td>

                      {/* Joined / Invited Date */}
                      <td className="py-3 px-4 text-on-surface-variant text-[11px]">
                        {formattedDate}
                      </td>

                      {/* Actions */}
                      <td className="py-3 px-4 text-right">
                        {isPending ? (
                          /* Pending Invitation Actions */
                          isOwner && (
                            <div className="flex items-center justify-end gap-2">
                              <button
                                onClick={() => handleResendInvite(m)}
                                className="px-2.5 py-1 rounded-lg bg-surface-container hover:bg-surface-container-high border border-outline-variant text-on-surface-variant hover:text-on-surface text-[11px] font-semibold transition-colors flex items-center gap-1 cursor-pointer"
                                title="Resend invitation"
                              >
                                <Send className="w-3 h-3" />
                                Resend
                              </button>
                              <button
                                onClick={() => setRevokeModal(m)}
                                className="px-2.5 py-1 rounded-lg bg-error/10 hover:bg-error/20 border border-error/20 text-error text-[11px] font-semibold transition-colors flex items-center gap-1 cursor-pointer"
                                title="Revoke invitation"
                              >
                                <X className="w-3 h-3" />
                                Revoke
                              </button>
                            </div>
                          )
                        ) : isTargetOwner ? (
                          /* Primary Owner Protection */
                          <span className="text-[11px] font-medium text-amber-500/80 italic">
                            Primary Owner
                          </span>
                        ) : (
                          /* Accepted Member Actions */
                          isOwner && !isMe && (
                            <div className="flex items-center justify-end gap-2">
                              <button
                                onClick={() =>
                                  setRoleChangeModal({
                                    member: m,
                                    targetRole: m.role === "VIEWER" ? "EDITOR" : "VIEWER",
                                  })
                                }
                                className="px-2.5 py-1 rounded-lg bg-surface-container hover:bg-surface-container-high border border-outline-variant text-on-surface-variant hover:text-on-surface text-[11px] font-semibold transition-colors flex items-center gap-1 cursor-pointer"
                                title="Change Role"
                              >
                                <Edit3 className="w-3 h-3" />
                                Change Role
                              </button>
                              <button
                                onClick={() => setRemoveModal(m)}
                                className="px-2.5 py-1 rounded-lg bg-error/10 hover:bg-error/20 border border-error/20 text-error text-[11px] font-semibold transition-colors flex items-center gap-1 cursor-pointer"
                                title="Remove member"
                              >
                                <Trash2 className="w-3 h-3" />
                                Remove
                              </button>
                            </div>
                          )
                        )}
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Role-Based Access Control (RBAC) Permissions Matrix */}
      <div className="border border-outline-variant/60 rounded-xl bg-surface-container/30 p-5 space-y-3">
        <div className="flex items-center gap-2">
          <Shield className="w-4 h-4 text-emerald-500" />
          <h4 className="text-xs font-bold text-on-surface uppercase tracking-wider">
            Role-Based Access Control (RBAC) Permissions Matrix
          </h4>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
          <div className="p-3.5 rounded-xl bg-surface-container/60 border border-outline-variant/40 space-y-1.5">
            <div className="flex items-center gap-1.5 font-bold text-amber-500">
              <Crown className="w-3.5 h-3.5" />
              <span>Owner</span>
            </div>
            <p className="text-[11px] text-on-surface-variant leading-relaxed">
              Full administrative control. Can invite collaborators, assign/change member roles, transfer ownership, modify environment configurations, and delete sandboxes.
            </p>
          </div>

          <div className="p-3.5 rounded-xl bg-surface-container/60 border border-outline-variant/40 space-y-1.5">
            <div className="flex items-center gap-1.5 font-bold text-emerald-500">
              <ShieldCheck className="w-3.5 h-3.5" />
              <span>Editor</span>
            </div>
            <p className="text-[11px] text-on-surface-variant leading-relaxed">
              Read and write development access. Can access workspaces, create/edit code in Monaco editor, execute terminal commands via xterm.js, and deploy containers.
            </p>
          </div>

          <div className="p-3.5 rounded-xl bg-surface-container/60 border border-outline-variant/40 space-y-1.5">
            <div className="flex items-center gap-1.5 font-bold text-slate-400">
              <Eye className="w-3.5 h-3.5" />
              <span>Viewer</span>
            </div>
            <p className="text-[11px] text-on-surface-variant leading-relaxed">
              Read-only audit access. Can view live preview URLs, inspect real-time logs, stream WebSocket outputs, and review project metrics without modifying resources.
            </p>
          </div>
        </div>
      </div>

      {/* Invite Collaborator Modal */}
      <InviteCollaboratorModal
        isOpen={isInviteOpen}
        onClose={() => setIsInviteOpen(false)}
        onInvite={handleInvite}
        isInviting={isInviting}
        projectId={projectId}
        projectName={projectName}
      />

      {/* Role Change Confirmation Modal */}
      {roleChangeModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
          <div className="w-full max-w-md bg-surface-container-lowest rounded-2xl border border-outline-variant p-6 space-y-4 shadow-2xl">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-500 shrink-0">
                <Edit3 className="w-5 h-5" />
              </div>
              <div>
                <h3 className="font-bold text-sm text-on-surface">Change Member Role</h3>
                <p className="text-xs text-on-surface-variant">
                  Update role for <strong>{roleChangeModal.member.user?.username || roleChangeModal.member.user?.email}</strong>
                </p>
              </div>
            </div>

            <div className="space-y-2 pt-2">
              <label className="text-xs font-bold uppercase tracking-wider text-on-surface-variant block">
                Assign New Role
              </label>

              <div
                onClick={() =>
                  setRoleChangeModal({ ...roleChangeModal, targetRole: "EDITOR" })
                }
                className={`p-3 rounded-xl border-2 cursor-pointer transition-all ${
                  roleChangeModal.targetRole === "EDITOR"
                    ? "border-emerald-500 bg-emerald-500/5"
                    : "border-outline-variant/60 hover:border-outline-variant"
                }`}
              >
                <div className="flex items-center justify-between font-bold text-xs text-on-surface">
                  <span>Editor</span>
                  {roleChangeModal.targetRole === "EDITOR" && (
                    <span className="text-[10px] bg-emerald-500/20 text-emerald-500 px-2 py-0.5 rounded-full">
                      Selected
                    </span>
                  )}
                </div>
                <p className="text-[11px] text-on-surface-variant mt-1">
                  Full development access to edit files, execute commands, and run containers.
                </p>
              </div>

              <div
                onClick={() =>
                  setRoleChangeModal({ ...roleChangeModal, targetRole: "VIEWER" })
                }
                className={`p-3 rounded-xl border-2 cursor-pointer transition-all ${
                  roleChangeModal.targetRole === "VIEWER"
                    ? "border-emerald-500 bg-emerald-500/5"
                    : "border-outline-variant/60 hover:border-outline-variant"
                }`}
              >
                <div className="flex items-center justify-between font-bold text-xs text-on-surface">
                  <span>Viewer</span>
                  {roleChangeModal.targetRole === "VIEWER" && (
                    <span className="text-[10px] bg-emerald-500/20 text-emerald-500 px-2 py-0.5 rounded-full">
                      Selected
                    </span>
                  )}
                </div>
                <p className="text-[11px] text-on-surface-variant mt-1">
                  Read-only access to view files, container logs, and preview URLs.
                </p>
              </div>
            </div>

            <div className="pt-3 flex items-center justify-end gap-3 border-t border-outline-variant/60">
              <button
                type="button"
                onClick={() => setRoleChangeModal(null)}
                disabled={isSubmittingAction}
                className="px-4 py-2 text-xs font-semibold text-on-surface-variant hover:text-on-surface rounded-lg hover:bg-surface-container transition-colors"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleConfirmRoleChange}
                disabled={isSubmittingAction}
                className="px-4 py-2 text-xs font-bold rounded-xl bg-emerald-500 hover:bg-emerald-600 text-white shadow-md flex items-center gap-2 cursor-pointer"
              >
                {isSubmittingAction && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                Confirm Role Change
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Remove Member Confirmation Modal */}
      {removeModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
          <div className="w-full max-w-md bg-surface-container-lowest rounded-2xl border border-outline-variant p-6 space-y-4 shadow-2xl">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-error/10 border border-error/30 flex items-center justify-center text-error shrink-0">
                <Trash2 className="w-5 h-5" />
              </div>
              <div>
                <h3 className="font-bold text-sm text-on-surface">Remove Member</h3>
                <p className="text-xs text-on-surface-variant">
                  Revoke project access for <strong>{removeModal.user?.username || removeModal.user?.email}</strong>
                </p>
              </div>
            </div>

            <p className="text-xs text-on-surface-variant leading-relaxed">
              Are you sure you want to remove <strong>{removeModal.user?.username || removeModal.user?.email}</strong> from this project? They will immediately lose access to all shared repositories, workspaces, and preview environments.
            </p>

            <div className="pt-3 flex items-center justify-end gap-3 border-t border-outline-variant/60">
              <button
                type="button"
                onClick={() => setRemoveModal(null)}
                disabled={isSubmittingAction}
                className="px-4 py-2 text-xs font-semibold text-on-surface-variant hover:text-on-surface rounded-lg hover:bg-surface-container transition-colors"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleConfirmRemoveMember}
                disabled={isSubmittingAction}
                className="px-4 py-2 text-xs font-bold rounded-xl bg-error hover:bg-error/90 text-on-error shadow-md flex items-center gap-2 cursor-pointer"
              >
                {isSubmittingAction && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                Remove Member
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Revoke Invitation Confirmation Modal */}
      {revokeModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
          <div className="w-full max-w-md bg-surface-container-lowest rounded-2xl border border-outline-variant p-6 space-y-4 shadow-2xl">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-error/10 border border-error/30 flex items-center justify-center text-error shrink-0">
                <X className="w-5 h-5" />
              </div>
              <div>
                <h3 className="font-bold text-sm text-on-surface">Revoke Invitation</h3>
                <p className="text-xs text-on-surface-variant">
                  Cancel pending invitation for <strong>{revokeModal.user?.username || revokeModal.user?.email}</strong>
                </p>
              </div>
            </div>

            <p className="text-xs text-on-surface-variant leading-relaxed">
              Are you sure you want to revoke the pending invitation for <strong>{revokeModal.user?.username || revokeModal.user?.email}</strong>? The invitation link will immediately expire.
            </p>

            <div className="pt-3 flex items-center justify-end gap-3 border-t border-outline-variant/60">
              <button
                type="button"
                onClick={() => setRevokeModal(null)}
                disabled={isSubmittingAction}
                className="px-4 py-2 text-xs font-semibold text-on-surface-variant hover:text-on-surface rounded-lg hover:bg-surface-container transition-colors"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleConfirmRevokeInvite}
                disabled={isSubmittingAction}
                className="px-4 py-2 text-xs font-bold rounded-xl bg-error hover:bg-error/90 text-on-error shadow-md flex items-center gap-2 cursor-pointer"
              >
                {isSubmittingAction && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                Revoke Invitation
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Workspaces Tab ─────────────────────────────────────────────────────────────

function WorkspacesTab({ workspaces, projectId }: { workspaces?: Workspace[]; projectId: string }) {
  if (!Array.isArray(workspaces)) return <LoadingPlaceholder />;
  if (workspaces.length === 0) return <EmptyState icon={Box} title="No workspaces" description="Workspaces are created when editors join the project." />;

  return (
    <div className="space-y-3">
      {workspaces.map((ws: Workspace, idx: number) => (
        <motion.div key={ws.id} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: idx * 0.04 }}
          className="bg-surface-container-lowest border border-outline-variant rounded-xl p-5"
        >
          <div className="flex items-center gap-4">
            <div className="w-10 h-10 rounded-lg bg-primary-fixed/10 border border-primary-fixed/20 flex items-center justify-center shrink-0">
              {ws.type === "CANONICAL" ? <Folder className="w-5 h-5 text-amber-400" /> : <Code className="w-5 h-5 text-primary-fixed" />}
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 flex-wrap">
                <p className="font-semibold text-on-surface">{ws.type === "CANONICAL" ? "Canonical Workspace" : `Editor Workspace`}</p>
                <span className={`text-[10px] font-bold tracking-wider px-2 py-0.5 rounded-full uppercase border ${ws.type === "CANONICAL" ? "bg-amber-400/10 text-amber-400 border-amber-400/20" : "bg-blue-400/10 text-blue-400 border-blue-400/20"}`}>
                  {ws.type}
                </span>
              </div>
              <p className="text-xs text-on-surface-variant font-mono">{ws.id}</p>
            </div>
            {ws.environment && (
              <StatusBadge status={ws.environment.status} />
            )}
            {ws.environmentId && (
              <Link href={`/environments/${ws.environmentId}`}
                className="flex items-center gap-1 text-xs font-semibold text-primary-fixed hover:underline shrink-0"
              >
                Open <ArrowRight className="w-3.5 h-3.5" />
              </Link>
            )}
          </div>
          {Array.isArray(ws.repositories) && ws.repositories.length > 0 && (
            <div className="mt-3 pt-3 border-t border-outline-variant/50 flex flex-wrap gap-2">
              {ws.repositories.map((r: WorkspaceRepository) => (
                <span key={r.id} className="text-xs font-mono bg-surface-container px-2 py-1 rounded-lg text-on-surface-variant flex items-center gap-1.5">
                  <GitBranch className="w-3 h-3" />
                  {r.branch}
                  {r.currentCommit && <span className="text-[10px] opacity-60">{r.currentCommit.slice(0, 7)}</span>}
                </span>
              ))}
            </div>
          )}
        </motion.div>
      ))}
    </div>
  );
}

// ── Environments Tab ───────────────────────────────────────────────────────────

function EnvironmentsTab({ environments, projectId }: { environments?: Environment[]; projectId: string }) {
  if (!Array.isArray(environments)) return <LoadingPlaceholder />;
  if (environments.length === 0) {
    return (
      <div className="bg-surface-container-lowest border border-outline-variant rounded-2xl p-8 text-center flex flex-col items-center max-w-lg mx-auto">
        <div className="w-12 h-12 rounded-xl bg-primary-fixed/10 border border-primary-fixed/20 flex items-center justify-center text-primary-fixed mb-4">
          <Play className="w-6 h-6" />
        </div>
        <h3 className="font-bold text-on-surface text-lg mb-1">No Sandboxes in this Project</h3>
        <p className="text-sm text-on-surface-variant mb-6">Deploy a sandbox to start coding and testing within this workspace project.</p>
        <Link
          href={`/upload?projectId=${projectId}`}
          className="flex items-center gap-2 bg-gradient-to-r from-[#FF416C] to-[#FF4B2B] text-white px-5 py-2.5 rounded-xl font-bold text-xs uppercase tracking-wider hover:opacity-95 transition-all shadow-md"
        >
          <Plus className="w-4 h-4" />
          <span>Deploy First Sandbox</span>
        </Link>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {environments.map((env: Environment, idx: number) => (
        <motion.div key={env.id} initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: idx * 0.04 }}>
          <Link href={`/environments/${env.id}`} className="block group h-full">
            <div className="bg-surface-container-lowest border border-outline-variant rounded-xl p-5 h-full flex flex-col gap-4 transition-all duration-300 hover:border-primary-fixed/40 hover:shadow-[0_0_24px_rgba(0,240,255,0.08)] relative overflow-hidden">
              <div className="absolute inset-0 bg-gradient-to-br from-primary-fixed/5 to-transparent opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none" />
              <div className="flex items-start justify-between gap-3 relative z-10">
                <div className="w-9 h-9 rounded-lg bg-primary-fixed/10 border border-primary-fixed/20 flex items-center justify-center shrink-0">
                  <Box className="w-4 h-4 text-primary-fixed" />
                </div>
                <StatusBadge status={env.status} />
              </div>
              <div className="relative z-10 flex-1 min-w-0">
                <h3 className="font-bold text-base text-on-surface group-hover:text-primary-fixed transition-colors truncate mb-1.5">{env.name}</h3>
                <p className="text-xs text-on-surface-variant font-mono truncate">
                  {env.createdAt ? formatDistanceToNow(new Date(env.createdAt), { addSuffix: true }) : ""}
                </p>
              </div>
              <div className="relative z-10 flex justify-end">
                <ArrowRight className="w-3.5 h-3.5 opacity-0 group-hover:opacity-100 text-primary-fixed transition-all group-hover:translate-x-0.5 duration-200" />
              </div>
            </div>
          </Link>
        </motion.div>
      ))}
    </div>
  );
}

// ── Change Requests Tab ────────────────────────────────────────────────────────

function ChangeRequestsTab({ changeRequests }: { changeRequests?: ChangeRequest[] }) {
  if (!Array.isArray(changeRequests)) return <LoadingPlaceholder />;
  if (changeRequests.length === 0) return <EmptyState icon={GitMerge} title="No change requests" description="Change requests appear here when editors submit their work for review." />;

  return (
    <div className="space-y-3">
      {changeRequests.map((cr: ChangeRequest, idx: number) => (
        <motion.div key={cr.id} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} transition={{ delay: idx * 0.04 }}
          className="bg-surface-container-lowest border border-outline-variant rounded-xl p-5"
        >
          <div className="flex items-start gap-4">
            <div className="w-10 h-10 rounded-lg bg-primary-fixed/10 border border-primary-fixed/20 flex items-center justify-center shrink-0">
              <GitMerge className="w-5 h-5 text-primary-fixed" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 flex-wrap mb-1">
                <p className="font-semibold text-on-surface">{cr.title}</p>
                <CRStatusBadge status={cr.status} />
              </div>
              {cr.description && <p className="text-sm text-on-surface-variant mb-2">{cr.description}</p>}
              <p className="text-xs text-on-surface-variant">
                {cr.createdAt ? formatDistanceToNow(new Date(cr.createdAt), { addSuffix: true }) : ""}
              </p>
            </div>
          </div>
        </motion.div>
      ))}
    </div>
  );
}

// ── Shared helpers ─────────────────────────────────────────────────────────────

function LoadingPlaceholder() {
  return (
    <div className="space-y-3">
      {[1, 2, 3].map((i) => (
        <div key={i} className="bg-surface-container-lowest border border-outline-variant rounded-xl h-20 animate-pulse" />
      ))}
    </div>
  );
}

function EmptyState({ icon: Icon, title, description }: { icon: React.ElementType; title: string; description: string }) {
  return (
    <div className="bg-surface-container-lowest border border-outline-variant border-dashed rounded-xl py-16 text-center flex flex-col items-center gap-3">
      <div className="w-12 h-12 rounded-2xl bg-primary-fixed/5 border border-primary-fixed/10 flex items-center justify-center">
        <Icon className="w-6 h-6 text-on-surface-variant/30" />
      </div>
      <h3 className="text-lg font-bold text-on-surface">{title}</h3>
      <p className="text-sm text-on-surface-variant max-w-xs">{description}</p>
    </div>
  );
}
