"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import useSWR, { mutate } from "swr";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import toast from "react-hot-toast";
import "xterm/css/xterm.css";
import {
  Activity,
  Box,
  Clock,
  ExternalLink,
  GitBranch,
  Terminal as TerminalIcon,
  Loader2,
  Trash2,
  RefreshCw,
  File,
  Save,
  Code,
  Check,
  AlertCircle,
  FilePlus,
  FolderPlus,
  ScrollText,
  Users,
  Settings as SettingsIcon,
  Copy,
  Play,
  Crown,
  ShieldCheck,
  Eye,
  Search,
  Mail,
  UserPlus,
  LogOut,
  ChevronDown,
  CheckCircle2,
  XCircle,
  Shield,
  UserCheck,
  Sparkles,
  AlertTriangle,
  Send,
  X,
} from "lucide-react";

import { fetchWithAuth } from "@/lib/auth";
import { useTheme } from "@/lib/theme";
import TeamCollaborationDashboard from "@/components/TeamCollaborationDashboard";
import EnvironmentSettings from "@/components/EnvironmentSettings";
import { useEnvironmentChanges } from "@/hooks/useEnvironmentChanges";
import ActiveEditors from "@/components/ActiveEditors";
import { CommitModal, BranchPicker, GitStatusPanel, CommitHistoryPanel } from "@/components/GitUI";

import { FileTreeItem, type FileNode } from "@/components/editor/FileTree";
import { InviteCollaboratorModal } from "@/components/editor/InviteCollaboratorModal";
import { TransferSandboxModal } from "@/components/editor/TransferSandboxModal";

const fetcher = async (url: string) => {
  return fetchWithAuth(url);
};

function buildFileTree(flatEntries: any[]): FileNode[] {
  if (!Array.isArray(flatEntries)) return [];
  if (flatEntries.some((f) => f.children && f.children.length > 0)) {
    return flatEntries as FileNode[];
  }

  const rootNodes: FileNode[] = [];
  const dirMap = new Map<string, FileNode>();

  flatEntries.forEach((entry) => {
    const isDir = Boolean(entry.dir || entry.isDir);
    const rawPath = entry.path || "";
    if (!rawPath) return;
    const parts = rawPath.split("/").filter(Boolean);
    let currentPath = "";

    parts.forEach((part: string, index: number) => {
      const parentPath = currentPath;
      currentPath = parentPath ? `${parentPath}/${part}` : part;
      const isLast = index === parts.length - 1;
      const itemIsDir = isLast ? isDir : true;

      if (!dirMap.has(currentPath)) {
        const node: FileNode = {
          name: part,
          path: currentPath,
          isDir: itemIsDir,
          children: itemIsDir ? [] : undefined,
        };
        dirMap.set(currentPath, node);

        if (parentPath && dirMap.has(parentPath)) {
          const parent = dirMap.get(parentPath)!;
          if (!parent.children) parent.children = [];
          parent.children.push(node);
        } else if (!parentPath) {
          rootNodes.push(node);
        }
      }
    });
  });

  return rootNodes;
}

const statusColors: Record<string, string> = {
  idle: "text-gray-400 bg-gray-400/10 border-gray-400/20",
  building: "text-blue-400 bg-blue-400/10 border-blue-400/20 animate-pulse",
  queued: "text-yellow-400 bg-yellow-400/10 border-yellow-400/20 animate-pulse",
  running: "text-emerald-400 bg-emerald-400/10 border-emerald-400/20",
  stopped: "text-orange-400 bg-orange-400/10 border-orange-400/20",
  failed: "text-red-400 bg-red-400/10 border-red-400/20",
  expired: "text-zinc-400 bg-zinc-400/10 border-zinc-400/20",
  hibernated: "text-amber-400 bg-amber-400/10 border-amber-400/20",
  deleted: "text-zinc-500 bg-zinc-500/10 border-zinc-500/20",
  IDLE: "text-gray-400 bg-gray-400/10 border-gray-400/20",
  BUILDING: "text-blue-400 bg-blue-400/10 border-blue-400/20 animate-pulse",
  QUEUED: "text-yellow-400 bg-yellow-400/10 border-yellow-400/20 animate-pulse",
  RUNNING: "text-emerald-400 bg-emerald-400/10 border-emerald-400/20",
  STOPPED: "text-orange-400 bg-orange-400/10 border-orange-400/20",
  FAILED: "text-red-400 bg-red-400/10 border-red-400/20",
  EXPIRED: "text-zinc-400 bg-zinc-400/10 border-zinc-400/20",
  HIBERNATED: "text-amber-400 bg-amber-400/10 border-amber-400/20",
  DELETED: "text-zinc-500 bg-zinc-500/10 border-zinc-500/20",
};

const lightAshTheme = {
  background: "#EAECEF", // Ash grey in white mode
  foreground: "#1F2328",
  cursor: "#0969DA",
  selectionBackground: "rgba(9, 105, 218, 0.2)",
  black: "#24292F",
  red: "#CF222E",
  green: "#1A7F37",
  yellow: "#9A6700",
  blue: "#0969DA",
  magenta: "#8250DF",
  cyan: "#1B7C83",
  white: "#6E7781",
  brightBlack: "#57606A",
  brightRed: "#A40E26",
  brightGreen: "#116329",
  brightYellow: "#7D4E00",
  brightBlue: "#0550AE",
  brightMagenta: "#6639BA",
  brightCyan: "#124C53",
  brightWhite: "#1F2328",
};

const darkAshTheme = {
  background: "#1c1f24", // Ash grey in dark mode
  foreground: "#d7dae0",
  cursor: "#528bff",
  selectionBackground: "rgba(82, 139, 255, 0.25)",
  black: "#1e2227",
  red: "#e06c75",
  green: "#98c379",
  yellow: "#e5c07b",
  blue: "#61afef",
  magenta: "#c678dd",
  cyan: "#56b6c2",
  white: "#abb2bf",
  brightBlack: "#5c6370",
  brightRed: "#be5046",
  brightGreen: "#98c379",
  brightYellow: "#d19a66",
  brightBlue: "#61afef",
  brightMagenta: "#c678dd",
  brightCyan: "#56b6c2",
  brightWhite: "#ffffff",
};

export default function EnvironmentDetail() {
  const { isDark } = useTheme();
  const params = useParams();
  const id = params.id as string;
  const router = useRouter();
  const [isDeleting, setIsDeleting] = useState(false);
  const [isRestarting, setIsRestarting] = useState(false);
  const terminalRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<unknown>(null);
  const [terminalInstance, setTerminalInstance] = useState<any>(null);

  const { hasUncommittedChanges, setHasUncommittedChanges, activeEditors } =
    useEnvironmentChanges(id, {
      onReloadReady: () => {
        setSaveState("live");
        // After a short delay, return to idle so the 'Live' indicator fades gracefully
        setTimeout(() => {
          setSaveState((current) => (current === "live" ? "idle" : current));
        }, 2500);
      },
      onReloadFailed: () => {
        setSaveState("failed");
        setTimeout(() => {
          setSaveState((current) => (current === "failed" ? "idle" : current));
        }, 5000);
      },
    });
  const [isCommitting, setIsCommitting] = useState(false);

  const [isCommitModalOpen, setIsCommitModalOpen] = useState(false);

  const handleCommit = async (msg: string) => {
    if (!msg || !msg.trim()) return;
    setIsCommitting(true);
    try {
      const res = await fetch(`/api/environments/${id}/commit`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${localStorage.getItem("token")}`,
        },
        body: JSON.stringify({ message: msg.trim() }),
      });
      const data = await res.json();
      // the backend returns message not success field
      if (res.ok) {
        toast.success("Changes committed successfully!");
        setHasUncommittedChanges(false);
      } else {
        throw new Error(data.error || "Commit failed");
      }
    } catch (err: unknown) {
      toast.error((err as Error).message);
      throw err; // Re-throw to prevent pushing if commit failed
    } finally {
      setIsCommitting(false);
    }
  };

  const [isPushing, setIsPushing] = useState(false);
  const handlePush = async () => {
    setIsPushing(true);
    try {
      const res = await fetch(`/api/environments/${id}/push`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${localStorage.getItem("token")}`,
        },
      });
      const data = await res.json();
      if (res.ok) {
        toast.success("Successfully pushed to GitHub!");
      } else {
        throw new Error(data.error || "Push failed");
      }
    } catch (err: unknown) {
      toast.error((err as Error).message);
      throw err;
    } finally {
      setIsPushing(false);
    }
  };

  const handleCommitAndPush = async (msg: string) => {
    try {
      await handleCommit(msg);
      setIsCommitModalOpen(false);
    } catch {
      // Errors are toasted in handleCommit
    }
  };

  // Tab control
  const [activeTab, setActiveTab] = useState<
    "logs" | "workspace" | "collaborators" | "team-activity" | "settings"
  >("logs");

  // File explorer states
  const [selectedFilePath, setSelectedFilePath] = useState<string>("");
  const [fileContent, setFileContent] = useState<string>("");
  const [originalFileContent, setOriginalFileContent] = useState<string>("");
  const [isLoadingFile, setIsLoadingFile] = useState<boolean>(false);
  const [saveState, setSaveState] = useState<
    "idle" | "saving" | "reloading" | "live" | "failed"
  >("idle");
  const [isEditingFile, setIsEditingFile] = useState<boolean>(false);

  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const lineNumbersRef = useRef<HTMLDivElement>(null);

  // Docker Logs state
  const [dockerLogs, setDockerLogs] = useState<string>("");
  const [isLoadingDockerLogs, setIsLoadingDockerLogs] =
    useState<boolean>(false);

  // Modal states
  const [isInviteModalOpen, setIsInviteModalOpen] = useState(false);
  const [isInviting, setIsInviting] = useState(false);
  const [collabSearch, setCollabSearch] = useState("");
  const [collabRoleFilter, setCollabRoleFilter] = useState<
    "ALL" | "OWNER" | "EDITOR" | "VIEWER" | "PENDING"
  >("ALL");

  const [isTransferModalOpen, setIsTransferModalOpen] = useState(false);
  const [isTransferring, setIsTransferring] = useState(false);
  const [isForking, setIsForking] = useState(false);
  const { data: projects, isLoading: isProjectsLoading } = useSWR(
    "/api/projects",
    fetcher,
  );

  const { data: env, error, mutate: mutateEnv } = useSWR(`/api/environments/${id}`, fetcher, {
    refreshInterval: (data) => {
      const s = (data?.status || "").toLowerCase();
      return s === "building" || s === "queued" ? 1500 : 5000;
    },
  });

  const { data: deploymentsData, mutate: mutateDeployments } = useSWR(
    id ? `/api/sandboxes/${id}/deployments` : null,
    fetcher,
    {
      refreshInterval: (data) => {
        const latest = data?.deployments?.[0];
        const s = (env?.status || "").toLowerCase();
        return latest?.status === "building" || latest?.status === "pending" || s === "building" || s === "queued"
          ? 1500
          : 5000;
      },
    },
  );

  const envStatus = (env?.status || "idle").toLowerCase();
  const isBuilding = envStatus === "building" || envStatus === "queued";
  const isRunning = envStatus === "running";
  const isFailed = envStatus === "failed";
  const isDeleted = envStatus === "deleted";
  const isHibernated = envStatus === "hibernated" || envStatus === "stopped";

  const previewUrl = useMemo(() => {
    if (!env) return "";
    let url = env.publicUrl || env.url || "";
    if (!url && env.subdomain) {
      url = `http://${env.subdomain}.sandbox.localhost:8090`;
    }
    if (url && url.includes(".sandbox.localhost") && !url.includes(":8090")) {
      url = url.replace(".sandbox.localhost", ".sandbox.localhost:8090");
    }
    return url;
  }, [env]);

  const parsedBuildConfig = useMemo(() => {
    if (!env?.build_config) return null;
    try {
      return typeof env.build_config === "string"
        ? JSON.parse(env.build_config)
        : env.build_config;
    } catch {
      return null;
    }
  }, [env?.build_config]);

  const resolvedBuilder = parsedBuildConfig?.builder || env?.builder || (parsedBuildConfig?.dockerfile_path ? "dockerfile" : "");
  const resolvedRuntime = parsedBuildConfig?.provider || env?.detected_language || "";
  const resolvedPort = parsedBuildConfig?.port || env?.internal_port || 0;
  const resolvedDbEngine = env?.database_engine || "";

  const fetchDockerLogs = async () => {
    setIsLoadingDockerLogs(true);
    try {
      if (isFailed) {
        setDockerLogs(
          "[Build Failed]\nContainer failed during image build and is not running.\nPlease inspect the Build Logs panel on the left for details."
        );
        return;
      }
      if (isBuilding) {
        setDockerLogs(
          "[Sandbox Building]\nContainer image is currently being built by Railpack.\nLive application stdout/stderr will stream here once the container is running."
        );
        return;
      }
      if (envStatus === "hibernated") {
        setDockerLogs(
          "[Sandbox Hibernated]\nContainer is paused due to inactivity. Click 'Restart' to wake it up."
        );
        return;
      }
      if (envStatus === "expired" || envStatus === "deleted") {
        setDockerLogs(`[Sandbox ${envStatus.toUpperCase()}]\nThis sandbox has been torn down.`);
        return;
      }

      const res = await fetch(`/api/environments/${id}/docker-logs`, {
        credentials: "include",
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.message || data.error || "Failed to fetch container logs");
      }
      const data = await res.json().catch(() => null);
      if (data && typeof data === "object") {
        if (data.available && data.output) {
          setDockerLogs(data.output);
        } else if (data.message) {
          setDockerLogs(`[${data.status || envStatus.toUpperCase()}]\n${data.message}`);
        } else {
          setDockerLogs("(No container output yet. The application may still be starting.)");
        }
      } else {
        const text = typeof data === "string" ? data : "";
        setDockerLogs(
          text || "(No container output yet. The application may still be starting.)",
        );
      }
    } catch {
      if (isRunning) {
        setDockerLogs("(Connecting to live container logs...)");
      } else {
        setDockerLogs(`(Status: ${envStatus.toUpperCase()} - no container output available)`);
      }
    } finally {
      setIsLoadingDockerLogs(false);
    }
  };

  useEffect(() => {
    if (activeTab !== "logs") return;

    if (!isRunning) {
      fetchDockerLogs();
      return;
    }

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsUrl = `${protocol}//${window.location.hostname}:8080/api/sandboxes/${id}/logs/ws`;
    let ws: WebSocket | null = null;
    let isSubscribed = true;

    try {
      ws = new WebSocket(wsUrl);

      ws.onopen = () => {
        if (!isSubscribed) return;
        setDockerLogs((prev) =>
          prev ? prev + "\n[Connected to live container logs]\n" : "[Connected to live container logs]\n",
        );
      };

      ws.onmessage = (event) => {
        if (!isSubscribed) return;
        try {
          const msg = JSON.parse(event.data);
          if (msg.type === "log" && msg.line) {
            setDockerLogs((prev) => (prev ? prev + "\n" + msg.line : msg.line));
          } else if (msg.type === "unavailable") {
            setDockerLogs((prev) => (prev ? prev + `\n(${msg.reason})` : `(${msg.reason})`));
          } else if (msg.type === "closed") {
            setDockerLogs((prev) =>
              prev ? prev + `\n[Stream closed: ${msg.reason || "end of output"}]` : `[Stream closed: ${msg.reason || "end of output"}]`,
            );
          }
        } catch {
          if (typeof event.data === "string") {
            setDockerLogs((prev) => (prev ? prev + "\n" + event.data : event.data));
          }
        }
      };

      ws.onerror = () => {
        if (!isSubscribed) return;
        fetchDockerLogs();
      };
    } catch {
      fetchDockerLogs();
    }

    return () => {
      isSubscribed = false;
      if (ws) {
        ws.close();
      }
    };
  }, [activeTab, isRunning, isBuilding, isFailed, envStatus, id]);

  const handleTransfer = async (destProjectId: string) => {
    if (!destProjectId) return;
    setIsTransferring(true);
    try {
      await fetchWithAuth(`/api/environments/${id}/transfer`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ projectId: destProjectId }),
      });
      toast.success("Sandbox transferred successfully!");
      setIsTransferModalOpen(false);
      mutate(`/api/environments/${id}`);
      mutate(`/api/environments?projectId=${env?.projectId}`); // old project
      mutate(`/api/environments?projectId=${destProjectId}`); // new project
      mutate(`/api/environments`); // dashboard list
    } catch (e: unknown) {
      toast.error((e as Error).message);
    } finally {
      setIsTransferring(false);
    }
  };

  const handleFork = async () => {
    if (
      !confirm(
        "Are you sure you want to fork this sandbox? This will create a duplicate sandbox in this project.",
      )
    )
      return;
    setIsForking(true);
    try {
      const res = await fetchWithAuth(`/api/environments/${id}/fork`, {
        method: "POST",
      });
      toast.success("Sandbox forked successfully!");
      // The response contains the new environment
      router.push(`/environments/${res.id}`);
    } catch (e: unknown) {
      toast.error((e as Error).message || "Failed to fork sandbox");
      setIsForking(false);
    }
  };

  const handleInvite = async (identifier: string, role: string) => {
    if (!identifier.trim() || !env?.projectId) {
      toast.error("Project ID or user identifier missing");
      return;
    }
    setIsInviting(true);
    try {
      const token = localStorage.getItem("token");
      const normalizedRole = role === "VIEWER" ? "VIEWER" : "EDITOR";
      const res = await fetch(`/api/projects/${env.projectId}/invite`, {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
          Authorization: token ? `Bearer ${token}` : "",
        },
        body: JSON.stringify({
          identifier: identifier.trim(),
          role: normalizedRole,
        }),
      });

      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || "Failed to invite collaborator");
      }

      toast.success("Collaborator invited successfully!");
      setIsInviteModalOpen(false);
      mutate(`/api/projects/${env.projectId}`);
      mutate(`/api/projects/${env.projectId}/members`);
    } catch (e: unknown) {
      toast.error((e as Error).message || "Failed to invite collaborator");
    } finally {
      setIsInviting(false);
    }
  };

  const handleUpdateRole = async (userId: string, newRole: string) => {
    if (!env?.projectId) return;
    try {
      const token = localStorage.getItem("token");
      const res = await fetch(
        `/api/projects/${env.projectId}/collaborators/${userId}/role`,
        {
          method: "PUT",
          credentials: "include",
          headers: {
            "Content-Type": "application/json",
            Authorization: token ? `Bearer ${token}` : "",
          },
          body: JSON.stringify({ role: newRole }),
        },
      );
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || "Failed to update member role");
      }
      toast.success(`Role updated to ${newRole}`);
      mutate(`/api/projects/${env.projectId}`);
      mutate(`/api/projects/${env.projectId}/members`);
    } catch (e: unknown) {
      toast.error((e as Error).message || "Failed to update role");
    }
  };

  const handleRemoveCollaborator = async (userId: string, isSelf: boolean = false) => {
    if (!env?.projectId) return;
    const confirmMsg = isSelf
      ? "Are you sure you want to leave this project workspace?"
      : "Are you sure you want to remove this collaborator?";
    if (!confirm(confirmMsg)) return;

    try {
      const token = localStorage.getItem("token");
      const res = await fetch(
        `/api/projects/${env.projectId}/collaborators/${userId}`,
        {
          method: "DELETE",
          credentials: "include",
          headers: {
            Authorization: token ? `Bearer ${token}` : "",
          },
        },
      );
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || "Failed to remove collaborator");
      }
      toast.success(isSelf ? "You left the project" : "Collaborator removed successfully");
      mutate(`/api/projects/${env.projectId}`);
      mutate(`/api/projects/${env.projectId}/members`);
    } catch (e: unknown) {
      toast.error((e as Error).message || "Failed to remove collaborator");
    }
  };

  // Auto-switch to logs tab when environment is BUILDING or FAILED
  useEffect(() => {
    if (isBuilding || isFailed) {
      const timer = setTimeout(() => {
        setActiveTab("logs");
      }, 0);
      return () => clearTimeout(timer);
    }
  }, [isBuilding, isFailed]);

  const { data: rawFiles } = useSWR(
    activeTab === "workspace" ? `/api/environments/${id}/files` : null,
    fetcher,
  );
  const files: FileNode[] = buildFileTree(rawFiles?.files || rawFiles || []);

  const { data: currentUser } = useSWR("/api/me", fetcher);

  const { data: projectData, mutate: mutateProject } = useSWR(
    env?.projectId ? `/api/projects/${env.projectId}` : "/api/projects/default-project",
    fetcher,
  );

  const { data: projectMembersData, mutate: mutateMembers, isLoading: isMembersLoading } = useSWR(
    env?.projectId ? `/api/projects/${env.projectId}/members` : null,
    fetcher,
    { refreshInterval: 5000 },
  );

  const fallbackProject = useMemo(() => {
    const defaultUsername = currentUser?.username || "Owner";
    const defaultEmail =
      currentUser?.email ||
      (currentUser?.username
        ? `${currentUser.username.toLowerCase()}@users.api-sandbox.local`
        : "owner@api-sandbox.local");
    const defaultUserId = currentUser?.id || `user-${defaultUsername.toLowerCase()}`;
    return {
      id: env?.projectId || "default-project",
      name: "Default Workspace",
      isDefault: true,
      collaborators: [
        {
          id: "owner",
          userId: defaultUserId,
          role: "OWNER",
          status: "ACCEPTED",
          acceptedAt: new Date().toISOString(),
          createdAt: new Date().toISOString(),
          user: {
            email: defaultEmail,
            username: defaultUsername,
          },
        },
      ],
    };
  }, [env?.projectId, currentUser]);

  const project = projectData || fallbackProject;

  const isDefaultProject = Boolean(
    !env?.projectId ||
    env.projectId === "default-project" ||
    project?.isDefault ||
    project?.id === "default-project"
  );

  const collaborators = useMemo(() => {
    if (isDefaultProject) {
      return fallbackProject.collaborators;
    }
    if (Array.isArray(projectMembersData) && projectMembersData.length > 0) {
      return projectMembersData;
    }
    if (projectData?.members && Array.isArray(projectData.members) && projectData.members.length > 0) {
      return projectData.members;
    }
    if (projectData?.collaborators && Array.isArray(projectData.collaborators) && projectData.collaborators.length > 0) {
      return projectData.collaborators;
    }
    return fallbackProject.collaborators;
  }, [isDefaultProject, projectMembersData, projectData, fallbackProject]);

  const isEnvOwner = Boolean(
    currentUser && env && (env.userId ? currentUser.id === env.userId : true),
  );
  const myCollab = collaborators.find(
    (c: { userId?: string; id?: string; role?: string }) =>
      (c.userId && currentUser?.id && c.userId === currentUser.id) ||
      (c.id && currentUser?.id && c.id === currentUser.id),
  );
  const isOwnerOrAdmin = isDefaultProject || isEnvOwner || myCollab?.role === "OWNER" || myCollab?.role === "ADMIN";
  const isViewerRole = !isDefaultProject && !isOwnerOrAdmin && myCollab && myCollab.role === "VIEWER";

  const { data: lifecycleStatus, mutate: mutateLifecycle } = useSWR(
    id ? `/api/environments/${id}/lifecycle/status` : null,
    fetcher,
    { refreshInterval: 4000 }
  );

  const [isRestoring, setIsRestoring] = useState(false);
  const [isExtendingActivity, setIsExtendingActivity] = useState(false);

  const handleTouchActivity = async () => {
    setIsExtendingActivity(true);
    try {
      await fetchWithAuth(`/api/environments/${id}/activity`, { method: "POST" });
      toast.success("Presence confirmed! Idle countdown reset.");
      mutateLifecycle();
    } catch {
      toast.error("Failed to reset idle timer");
    } finally {
      setIsExtendingActivity(false);
    }
  };

  const handleRestoreSandbox = async () => {
    if (!confirm("Restore workspace files into a brand new sandbox instance? The expired container runtime will remain retired.")) {
      return;
    }
    setIsRestoring(true);
    try {
      const res = await fetchWithAuth(`/api/environments/${id}/restore`, { method: "POST" });
      toast.success("New sandbox restored successfully from preserved workspace!");
      if (res?.new_id) {
        router.push(`/environments/${res.new_id}`);
      } else {
        mutate(`/api/environments/${id}`);
      }
    } catch (e: unknown) {
      toast.error((e as Error).message || "Failed to restore sandbox");
    } finally {
      setIsRestoring(false);
    }
  };

  // Sync scroll for line numbers in textarea
  const handleScroll = () => {
    if (textareaRef.current && lineNumbersRef.current) {
      lineNumbersRef.current.scrollTop = textareaRef.current.scrollTop;
    }
  };

  // Fetch file content when path changes
  useEffect(() => {
    if (!selectedFilePath) return;

    const fetchFile = async () => {
      setIsLoadingFile(true);
      try {
        const res = await fetch(
          `/api/environments/${id}/file?path=${encodeURIComponent(selectedFilePath)}`,
          {
            credentials: "include",
          },
        );
        if (!res.ok) throw new Error("Failed to load file content");
        const data = await res.json();
        setFileContent(data.content ?? "");
        setOriginalFileContent(data.content ?? "");
        setIsEditingFile(false);
      } catch (e: unknown) {
        toast.error((e as Error).message);
        setSelectedFilePath("");
      } finally {
        setIsLoadingFile(false);
      }
    };

    fetchFile();
  }, [selectedFilePath, id]);

  const handleSaveFile = async () => {
    if (!selectedFilePath) return;
    setSaveState("saving");
    try {
      const res = await fetch(`/api/environments/${id}/file`, {
        method: "PUT",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          path: selectedFilePath,
          content: fileContent,
        }),
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || "Failed to save changes");
      }
      const data = await res.json();
      if (data.redeploying) {
        setSaveState("reloading");
        toast.success("File saved — redeploy initiated!");
        setTimeout(() => {
          setSaveState("idle");
          mutateEnv();
          mutateDeployments();
        }, 2000);
      } else {
        toast.success("File saved successfully!");
        setSaveState("idle");
      }
      setOriginalFileContent(fileContent);
      setIsEditingFile(false);
      setHasUncommittedChanges(true);
      mutateEnv();
    } catch (e: unknown) {
      toast.error((e as Error).message);
      setSaveState("failed");
      setTimeout(() => setSaveState("idle"), 3000);
    }
  };

  const handleCreateFileOrFolder = async (isDir: boolean) => {
    const typeStr = isDir ? "Folder" : "File";
    const name = prompt(`Enter path/name of new ${typeStr}:`);
    if (!name || !name.trim()) return;

    try {
      const res = await fetch(`/api/environments/${id}/file`, {
        method: "PUT",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          path: name.trim(),
          content: isDir ? "" : "// New file\n",
        }),
      });

      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || `Failed to create ${typeStr}`);
      }

      toast.success(`${typeStr} created successfully!`);
      mutate(`/api/environments/${id}/files`);
      setSelectedFilePath(name.trim());
    } catch (e: unknown) {
      toast.error((e as Error).message);
    }
  };

  const handleDeleteFileOrFolder = async (
    pathToDelete: string,
    e: React.MouseEvent,
  ) => {
    e.stopPropagation();
    if (
      !confirm(
        `Are you sure you want to delete "${pathToDelete}"? This will restart the container.`,
      )
    )
      return;

    try {
      const token = localStorage.getItem("token");
      const res = await fetch(`/api/environments/${id}/files/delete`, {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          path: pathToDelete,
        }),
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || "Failed to delete file or folder");
      }

      toast.success("Deleted successfully!");
      // If we deleted the currently active file (or its parent directory), clear editor
      if (
        selectedFilePath === pathToDelete ||
        selectedFilePath.startsWith(pathToDelete + "/")
      ) {
        setSelectedFilePath("");
        setFileContent("");
        setOriginalFileContent("");
      }

      // Refresh files list
      mutate(`/api/environments/${id}/files`);
      // Refresh logs
      mutate(`/api/environments/${id}`);
    } catch (e: unknown) {
      toast.error((e as Error).message);
    }
  };

  // Terminal log renderer
  const renderTerminalLogs = (term: any) => {
    if (!term) return;
    const rawDeployments = deploymentsData?.deployments || deploymentsData || [];
    const deployments = Array.isArray(rawDeployments) ? rawDeployments : [];
    const latest = deployments[0];

    const key = `${latest?.id || "none"}-${latest?.status || "none"}-${latest?.log_excerpt?.length || 0}-${envStatus}`;
    if (term._renderedKey === key) return;
    term._renderedKey = key;

    term.clear();

    if (latest) {
      const trigger = latest.trigger || "build";
      const builder = latest.builder || "railpack";
      const deployStatus = (latest.status || "").toUpperCase();
      term.writeln(`\x1b[35m=== Build Deployment [${builder}] · Status: ${deployStatus} · Trigger: ${trigger} ===\x1b[0m\r\n`);

      if (latest.log_excerpt && latest.log_excerpt.trim()) {
        const lines = latest.log_excerpt.split("\n");
        lines.forEach((rawLine: string) => {
          const line = rawLine.replace(/\r$/, "");
          const lower = line.toLowerCase();
          if (
            lower.includes("erro") ||
            lower.includes("failed") ||
            lower.includes("error") ||
            lower.includes("exit status") ||
            lower.includes("timeout")
          ) {
            term.writeln(`\x1b[31m${line}\x1b[0m`);
          } else if (lower.includes("warn") || lower.includes("warning")) {
            term.writeln(`\x1b[33m${line}\x1b[0m`);
          } else if (
            lower.includes("info") ||
            lower.includes("step") ||
            lower.includes("cloning") ||
            lower.includes("building") ||
            lower.includes("installing")
          ) {
            term.writeln(`\x1b[36m${line}\x1b[0m`);
          } else {
            term.writeln(line);
          }
        });
      } else if (isBuilding) {
        term.writeln("\x1b[36m[BUILDING] Provisioning container with Railpack engine...\x1b[0m");
        term.writeln("\x1b[90mCloning repository and inspecting build plan...\x1b[0m");
      } else if (isFailed) {
        term.writeln("\x1b[31m[FAILED] Deployment failed without logs.\x1b[0m");
      } else if (isRunning) {
        term.writeln("\x1b[32m[SUCCESS] Container built and deployed successfully.\x1b[0m");
      }
    } else if (env?.logs && env.logs.length > 0) {
      const sortedLogs = [...env.logs].reverse();
      sortedLogs.forEach((l: { message: string; level: string }) => {
        const msg = l.message.replace(/\n$/, "").replace(/\r$/, "");
        term.writeln(`[${l.level.toUpperCase()}] ${msg}`);
      });
    } else if (isBuilding) {
      term.writeln("\x1b[36m[BUILDING] Provisioning container with Railpack engine...\x1b[0m");
      term.writeln("\x1b[90mWaiting for build logs to stream from builder...\x1b[0m");
    } else if (isFailed) {
      term.writeln("\x1b[31m[FAILED] Build failed.\x1b[0m");
    } else {
      term.writeln("\x1b[36mWaiting for logs...\x1b[0m");
    }
  };

  // Initialize Terminal
  useEffect(() => {
    if (!terminalRef.current || activeTab !== "logs") return;

    let term: any = null;
    let fitAddon: any = null;

    const initTerminal = async () => {
      const { Terminal } = await import("xterm");
      const { FitAddon } = await import("xterm-addon-fit");

      term = new Terminal({
        theme: isDark ? darkAshTheme : lightAshTheme,
        fontFamily: 'Menlo, Monaco, "Courier New", monospace',
        fontSize: 13,
        lineHeight: 1.45,
        convertEol: true,
        cursorBlink: true,
      });

      fitAddon = new FitAddon();
      term.loadAddon(fitAddon);
      term.open(terminalRef.current!);
      fitAddon.fit();
      xtermRef.current = term;
      setTerminalInstance(term);
      renderTerminalLogs(term);
    };
    initTerminal();

    const handleResize = () => {
      if (fitAddon) fitAddon.fit();
    };
    window.addEventListener("resize", handleResize);

    return () => {
      if (term) term.dispose();
      xtermRef.current = null;
      setTerminalInstance(null);
      window.removeEventListener("resize", handleResize);
    };
  }, [id, activeTab, isDark]);

  // Update terminal theme dynamically when light/dark mode toggles
  useEffect(() => {
    if (!terminalInstance) return;
    terminalInstance.options.theme = isDark ? darkAshTheme : lightAshTheme;
    terminalInstance._renderedKey = null;
    renderTerminalLogs(terminalInstance);
  }, [isDark, terminalInstance]);

  // Update terminal when deployments data or status changes
  useEffect(() => {
    if (!terminalInstance || activeTab !== "logs") return;
    renderTerminalLogs(terminalInstance);
  }, [terminalInstance, deploymentsData, envStatus, activeTab]);

  const handleDelete = async () => {
    if (hasUncommittedChanges) {
      if (
        !confirm(
          "WARNING: You have uncommitted changes. These will be PERMANENTLY LOST if you delete this sandbox without pushing to GitHub. Are you sure you want to delete?",
        )
      )
        return;
    } else {
      if (
        !confirm(
          "Are you sure you want to delete this sandbox? This cannot be undone.",
        )
      )
        return;
    }
    setIsDeleting(true);
    try {
      try {
        await fetchWithAuth(`/api/sandboxes/${id}/actions/destroy`, {
          method: "POST",
        });
      } catch {
        await fetchWithAuth(`/api/environments/${id}`, {
          method: "DELETE",
        });
      }
      toast.success("Sandbox deleted successfully");
      mutate(`/api/environments`);
      mutate(`/api/sandboxes`);
      mutate(`/api/environments?projectId=${env?.projectId}`);
      mutate(`/api/projects/${env?.projectId}`);
      router.push("/dashboard");
    } catch (e: unknown) {
      toast.error((e as Error).message);
      setIsDeleting(false);
    }
  };

  const handleRestart = async () => {
    if (isDeleted) {
      toast.error("Destroyed sandboxes cannot be redeployed; create a new sandbox instead.");
      return;
    }
    if (
      !confirm(
        "Are you sure you want to restart this sandbox? It will trigger a fresh build and deploy.",
      )
    )
      return;
    setIsRestarting(true);
    try {
      const res = await fetch(`/api/environments/${id}/restart`, {
        method: "POST",
        credentials: "include",
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || "Failed to restart sandbox");
      }
      toast.success("Sandbox restart initiated");
      mutateEnv();
      mutateDeployments();
      if (xtermRef.current) {
        const t = xtermRef.current as any;
        t.clear();
        t._renderedKey = "";
      }
    } catch (e: unknown) {
      toast.error((e as Error).message);
    } finally {
      setIsRestarting(false);
    }
  };

  const lineCount = fileContent.split("\n").length;
  const hasUnsavedChanges = fileContent !== originalFileContent;

  if (error)
    return (
      <div className="p-8 text-center text-red-400">
        Failed to load environment
      </div>
    );
  if (!env)
    return (
      <div className="p-8 text-center text-on-surface-variant animate-pulse">
        Loading environment details...
      </div>
    );

  return (
    <>
      <CommitModal
        isOpen={isCommitModalOpen}
        onClose={() => setIsCommitModalOpen(false)}
        onCommit={handleCommit}
        onCommitAndPush={handleCommitAndPush}
        isCommitting={isCommitting}
        isPushing={isPushing}
        hasUncommittedChanges={hasUncommittedChanges}
      />
      <div className="space-y-4">
        {/* Header Card */}
        <div className="bg-surface-container-lowest border border-outline-variant rounded-xl px-6 py-4">
          <div className="flex items-center justify-between gap-4 flex-wrap">
            {/* Left: icon + name + meta */}
            <div className="flex items-center gap-4 min-w-0">
              <div className="w-10 h-10 rounded-xl bg-primary-fixed/10 border border-primary-fixed/20 flex items-center justify-center shrink-0">
                <Box className="w-5 h-5 text-primary-fixed" />
              </div>
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <h1 className="text-xl font-bold text-on-surface tracking-tight truncate">
                    {env.name}
                  </h1>
                  <span className="px-2 py-0.5 rounded text-[10px] uppercase font-bold bg-primary-fixed/10 text-primary-fixed tracking-wider">
                    Live Testing Sandbox
                  </span>
                </div>
                <div className="flex items-center gap-3 mt-1 flex-wrap">
                  <div className="flex items-center gap-1.5 text-xs text-on-surface-variant/70 font-mono">
                    <GitBranch className="w-3.5 h-3.5" />
                    <a
                      href={env.gitUrl || "#"}
                      target="_blank"
                      rel="noreferrer"
                      className="hover:text-primary-fixed transition-colors truncate max-w-[200px]"
                    >
                      {env.gitUrl?.replace("https://github.com/", "") || "Local Workspace"}
                    </a>
                    <span className="text-outline-variant">·</span>
                    {!isViewerRole ? (
                      <BranchPicker
                        envId={id}
                        currentBranch={env.branch || env.githubBranch || "master"}
                        onBranchChanged={() =>
                          mutate(`/api/environments/${id}`)
                        }
                      />
                    ) : (
                      <span className="bg-primary-fixed/10 text-primary-fixed px-2 py-0.5 rounded text-[10px]">
                        {env.branch || env.githubBranch || "master"}
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-1 text-[10px] text-on-surface-variant/50 font-mono">
                    <Clock className="w-3 h-3" />
                    <span>{env.id.slice(0, 8)}...</span>
                  </div>
                  {resolvedBuilder && (
                    <span className="bg-purple-500/10 text-purple-300 border border-purple-500/20 px-2 py-0.5 rounded text-[10px] font-mono capitalize">
                      {resolvedBuilder}
                    </span>
                  )}
                  {resolvedRuntime && (
                    <span className="bg-sky-500/10 text-sky-300 border border-sky-500/20 px-2 py-0.5 rounded text-[10px] font-mono capitalize">
                      {resolvedRuntime}
                    </span>
                  )}
                  {resolvedPort > 0 && (
                    <span className="bg-emerald-500/10 text-emerald-300 border border-emerald-500/20 px-2 py-0.5 rounded text-[10px] font-mono">
                      :{resolvedPort}
                    </span>
                  )}
                  {resolvedDbEngine && (
                    <span className="bg-amber-500/10 text-amber-300 border border-amber-500/20 px-2 py-0.5 rounded text-[10px] font-mono uppercase">
                      DB: {resolvedDbEngine}
                    </span>
                  )}
                  <ActiveEditors editors={activeEditors} />
                </div>
              </div>
            </div>

            {/* Right: status + actions */}
            <div className="flex items-center gap-3 shrink-0 flex-wrap">
              <div
                className={`px-3 py-1.5 rounded-full border text-xs font-bold tracking-wider uppercase flex items-center gap-2 ${statusColors[envStatus] || statusColors.idle}`}
              >
                {isBuilding && (
                  <Loader2 className="w-3.5 h-3.5 animate-spin text-blue-400" />
                )}
                {isRunning && (
                  <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse shadow-[0_0_6px_#34d399]" />
                )}
                {isFailed && (
                  <AlertCircle className="w-3.5 h-3.5 text-red-400" />
                )}
                {env.status}
              </div>
              {isFailed && (
                <div className="flex items-center gap-2 px-3 py-1.5 bg-red-500/10 border border-red-500/30 rounded-lg text-red-400 text-xs font-semibold">
                  <AlertCircle className="w-3.5 h-3.5 shrink-0" />
                  Build failed — check logs below
                  <button
                    onClick={handleRestart}
                    disabled={isRestarting || isViewerRole}
                    className="ml-2 px-2 py-0.5 rounded bg-red-500/20 hover:bg-red-500/30 border border-red-500/30 transition-colors disabled:opacity-50"
                  >
                    {isRestarting ? <Loader2 className="w-3 h-3 animate-spin" /> : "Retry"}
                  </button>
                </div>
              )}
              {previewUrl && !isDeleted && (
                <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-outline-variant bg-surface-container-high/40 text-xs font-mono">
                  <span className="text-on-surface-variant text-[11px] font-sans font-medium mr-0.5">Preview:</span>
                  <a
                    href={previewUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="text-primary-fixed hover:underline flex items-center gap-1 font-semibold truncate max-w-[200px] sm:max-w-[280px]"
                    title={previewUrl}
                  >
                    <span className="truncate">{previewUrl.replace(/^https?:\/\//, "")}</span>
                    <ExternalLink className="w-3.5 h-3.5 shrink-0" />
                  </a>
                  {isHibernated && (
                    <span className="px-1.5 py-0.5 rounded text-[10px] font-sans font-medium bg-amber-500/10 text-amber-500 dark:text-amber-400 border border-amber-500/20 whitespace-nowrap">
                      Wakes on visit
                    </span>
                  )}
                  <button
                    type="button"
                    onClick={() => {
                      navigator.clipboard.writeText(previewUrl);
                      toast.success("Preview URL copied to clipboard!");
                    }}
                    title="Copy preview URL"
                    className="p-1 rounded hover:bg-surface-container-highest text-on-surface-variant hover:text-on-surface transition-colors ml-0.5 cursor-pointer"
                  >
                    <Copy className="w-3.5 h-3.5" />
                  </button>
                </div>
              )}
              {isDeleted ? (
                <div className="flex items-center gap-2 px-3 py-1.5 bg-zinc-500/10 border border-zinc-500/30 rounded-lg text-zinc-400 text-xs font-semibold">
                  <AlertCircle className="w-3.5 h-3.5 shrink-0 text-zinc-400" />
                  Sandbox torn down
                  <Link
                    href="/upload"
                    className="ml-2 px-3 py-1 rounded bg-primary-container text-on-primary-fixed-variant hover:shadow-[0_0_12px_rgba(0,240,255,0.2)] transition-all font-bold"
                  >
                    Create New Sandbox
                  </Link>
                  <Link
                    href="/dashboard"
                    className="px-3 py-1 rounded bg-surface-container-high border border-outline-variant text-on-surface hover:text-primary-fixed transition-colors"
                  >
                    Dashboard
                  </Link>
                </div>
              ) : (
                <>
                  <button
                    onClick={handleRestart}
                    disabled={isRestarting || isBuilding || isViewerRole}
                    className={`px-4 py-1.5 rounded-lg border bg-primary-fixed/5 flex items-center gap-1.5 transition-colors text-xs font-semibold ${isViewerRole ? "border-outline-variant/30 text-on-surface-variant/30 cursor-not-allowed" : "border-primary-fixed/30 text-primary-fixed hover:bg-primary-fixed/15 disabled:opacity-50"}`}
                  >
                    {isRestarting ? (
                      <Loader2 className="w-3.5 h-3.5 animate-spin" />
                    ) : isHibernated ? (
                      <Play className="w-3.5 h-3.5" />
                    ) : (
                      <RefreshCw className="w-3.5 h-3.5" />
                    )}
                    {isFailed ? "Retry Build" : isHibernated ? "Wake Sandbox" : "Restart"}
                  </button>
                  {!isViewerRole && (
                    <button
                      onClick={() => setIsCommitModalOpen(true)}
                      className={`px-4 py-1.5 rounded-lg border flex items-center gap-1.5 transition-colors text-xs font-semibold ${
                        hasUncommittedChanges
                          ? "border-amber-500/30 bg-amber-500/10 text-amber-500 hover:bg-amber-500/20 shadow-[0_0_8px_rgba(245,158,11,0.2)]"
                          : "border-[#2ea44f]/30 bg-[#2ea44f]/10 text-[#2ea44f] hover:bg-[#2ea44f]/20"
                      }`}
                    >
                      <Save className="w-3.5 h-3.5" />
                      {hasUncommittedChanges ? "Review & Commit" : "Git Actions"}
                    </button>
                  )}
                  <button
                    onClick={handleFork}
                    disabled={isForking}
                    className="px-4 py-1.5 rounded-lg border border-purple-500/30 bg-purple-500/10 text-purple-400 hover:bg-purple-500/20 flex items-center gap-1.5 transition-colors disabled:opacity-50 text-xs font-semibold"
                  >
                    {isForking ? (
                      <Loader2 className="w-3.5 h-3.5 animate-spin" />
                    ) : (
                      <GitBranch className="w-3.5 h-3.5" />
                    )}
                    Fork
                  </button>
                  {isEnvOwner && (
                    <>
                      <button
                        onClick={() => setIsTransferModalOpen(true)}
                        className="px-4 py-1.5 rounded-lg border border-indigo-500/30 bg-indigo-500/10 text-indigo-400 hover:bg-indigo-500/20 flex items-center gap-1.5 transition-colors text-xs font-semibold"
                      >
                        <RefreshCw className="w-3.5 h-3.5" />
                        Transfer
                      </button>
                      <button
                        onClick={handleDelete}
                        disabled={isDeleting}
                        className="px-4 py-1.5 rounded-lg border border-red-500/30 bg-red-500/5 text-red-400 hover:bg-red-500/15 flex items-center gap-1.5 transition-colors disabled:opacity-50 text-xs font-semibold"
                      >
                        {isDeleting ? (
                          <Loader2 className="w-3.5 h-3.5 animate-spin" />
                        ) : (
                          <Trash2 className="w-3.5 h-3.5" />
                        )}
                        Delete
                      </button>
                    </>
                  )}
                </>
              )}
            </div>
          </div>
        </div>

        {/* Lifecycle & Hibernation Alerts */}
        <div className="space-y-3">
          {/* Grace Period Active Banner */}
          {lifecycleStatus?.in_grace_period && (
            <div className="bg-red-500/15 border-2 border-red-500/40 rounded-2xl p-4 flex flex-col sm:flex-row items-center justify-between gap-4 text-red-400 animate-pulse shadow-[0_0_20px_rgba(239,68,68,0.2)]">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-red-500/20 border border-red-500/30 flex items-center justify-center shrink-0">
                  <AlertTriangle className="w-5 h-5 text-red-400" />
                </div>
                <div>
                  <h4 className="font-bold text-sm text-red-300">
                    Grace Period Active: Automatic Hibernation Imminent
                  </h4>
                  <p className="text-xs text-red-400/90 mt-0.5">
                    Sandbox has reached its idle deadline. It will hibernate in less than 60 seconds unless active team presence is registered.
                  </p>
                </div>
              </div>
              <button
                onClick={handleTouchActivity}
                disabled={isExtendingActivity}
                className="px-4 py-2 rounded-xl bg-red-500 text-white font-bold text-xs uppercase tracking-wider hover:bg-red-600 transition-colors shrink-0 shadow-lg cursor-pointer disabled:opacity-50"
              >
                {isExtendingActivity ? "Resetting..." : "I'm Still Working (Keep Alive)"}
              </button>
            </div>
          )}

          {/* Hibernation Warning Banner */}
          {lifecycleStatus?.in_hibernation_warning && !lifecycleStatus?.in_grace_period && (
            <div className="bg-amber-500/15 border border-amber-500/30 rounded-2xl p-4 flex flex-col sm:flex-row items-center justify-between gap-4 text-amber-300">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl bg-amber-500/20 border border-amber-500/30 flex items-center justify-center shrink-0">
                  <Clock className="w-5 h-5 text-amber-400" />
                </div>
                <div>
                  <h4 className="font-bold text-sm text-amber-200">
                    Hibernation Warning (Idle Approaching)
                  </h4>
                  <p className="text-xs text-amber-300/80 mt-0.5">
                    No active authorized activity detected recently. The sandbox will enter grace hibernation to save platform resources.
                  </p>
                </div>
              </div>
              <button
                onClick={handleTouchActivity}
                disabled={isExtendingActivity}
                className="px-4 py-2 rounded-xl bg-amber-500/20 hover:bg-amber-500/30 border border-amber-500/40 text-amber-200 font-semibold text-xs transition-colors shrink-0 cursor-pointer disabled:opacity-50"
              >
                {isExtendingActivity ? "Confirming..." : "I'm Still Here"}
              </button>
            </div>
          )}

          {/* Active Operation Protection Banner */}
          {lifecycleStatus?.has_active_operation && (
            <div className="bg-blue-500/15 border border-blue-500/30 rounded-2xl p-3.5 flex items-center gap-3 text-blue-300 text-xs">
              <Loader2 className="w-4 h-4 animate-spin text-blue-400 shrink-0" />
              <span>
                <strong>Active Operation Protection:</strong> {lifecycleStatus.active_operation?.type || "Operation"} currently running. Automatic hibernation is temporarily deferred.
              </span>
            </div>
          )}

          {/* Expired Runtime & Data Preservation Notice */}
          {(env?.status === "EXPIRED" || lifecycleStatus?.status === "EXPIRED") && (
            <div className="bg-zinc-800/80 border border-zinc-700 rounded-2xl p-5 flex flex-col sm:flex-row items-center justify-between gap-4 text-zinc-300">
              <div className="flex items-center gap-3.5">
                <div className="w-11 h-11 rounded-xl bg-zinc-700/60 border border-zinc-600 flex items-center justify-center shrink-0">
                  <Box className="w-5 h-5 text-zinc-400" />
                </div>
                <div>
                  <div className="flex items-center gap-2">
                    <h4 className="font-bold text-sm text-zinc-100">
                      Sandbox Runtime Expired
                    </h4>
                    <span className="text-[10px] bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 px-2 py-0.5 rounded-full font-bold">
                      Workspace Files Preserved
                    </span>
                  </div>
                  <p className="text-xs text-zinc-400 mt-1 leading-relaxed">
                    Per security lifecycle policy, expired containers cannot be revived directly. However, your code workspace is retained in cold storage for 7 days. You can spin up a new active sandbox instance anytime from these preserved files.
                  </p>
                </div>
              </div>
              <button
                onClick={handleRestoreSandbox}
                disabled={isRestoring}
                className="px-5 py-2.5 rounded-xl bg-primary-fixed text-surface font-bold text-xs uppercase tracking-wider hover:opacity-95 transition-opacity shrink-0 flex items-center gap-2 cursor-pointer disabled:opacity-50"
              >
                {isRestoring ? <Loader2 className="w-4 h-4 animate-spin" /> : <Sparkles className="w-4 h-4" />}
                <span>Restore to New Sandbox</span>
              </button>
            </div>
          )}
        </div>

        {/* Tabs Header */}
        <div className="flex border-b border-outline-variant gap-6">
          <button
            onClick={() => setActiveTab("logs")}
            className={`pb-3 text-sm font-medium transition-all relative ${
              activeTab === "logs"
                ? "text-primary-fixed border-b-2 border-primary-fixed"
                : "text-on-surface-variant hover:text-on-surface"
            }`}
          >
            <span className="flex items-center gap-2">
              <TerminalIcon className="w-4 h-4" />
              Build Logs & Output
            </span>
          </button>
          <button
            onClick={() => setActiveTab("workspace")}
            className={`pb-3 text-sm font-medium transition-all relative ${
              activeTab === "workspace"
                ? "text-primary-fixed border-b-2 border-primary-fixed"
                : "text-on-surface-variant hover:text-on-surface"
            }`}
          >
            <span className="flex items-center gap-2">
              <Code className="w-4 h-4" />
              Code Workspace
            </span>
          </button>
          <button
            onClick={() => setActiveTab("collaborators")}
            className={`pb-3 text-sm font-medium transition-all relative ${
              activeTab === "collaborators"
                ? "text-primary-fixed border-b-2 border-primary-fixed"
                : "text-on-surface-variant hover:text-on-surface"
            }`}
          >
            <span className="flex items-center gap-2">
              <Users className="w-4 h-4" />
              Collaborators
            </span>
          </button>
          <button
            onClick={() => setActiveTab("team-activity")}
            className={`pb-3 text-sm font-medium transition-all relative ${
              activeTab === "team-activity"
                ? "text-primary-fixed border-b-2 border-primary-fixed"
                : "text-on-surface-variant hover:text-on-surface"
            }`}
          >
            <span className="flex items-center gap-2">
              <Activity className="w-4 h-4" />
              Team Activity
            </span>
          </button>
          <button
            onClick={() => setActiveTab("settings")}
            className={`pb-3 text-sm font-medium transition-all relative ${
              activeTab === "settings"
                ? "text-primary-fixed border-b-2 border-primary-fixed"
                : "text-on-surface-variant hover:text-on-surface"
            }`}
          >
            <span className="flex items-center gap-2">
              <SettingsIcon className="w-4 h-4" />
              Settings
            </span>
          </button>
        </div>

        {/* Logs View */}
        {activeTab === "logs" && (
          <div
            className="flex gap-4"
            style={{ height: "calc(100vh - 260px)", minHeight: "420px" }}
          >
            {/* Build Logs */}
            <div className="flex-1 bg-[#EAECEF] dark:bg-[#1c1f24] border border-[#CBD1D8] dark:border-[#31363f] rounded-2xl overflow-hidden shadow-sm dark:shadow-lg flex flex-col transition-colors">
              <div className="bg-[#DFE2E6] dark:bg-[#242830] border-b border-[#CBD1D8] dark:border-[#31363f] px-4 py-3 flex items-center justify-between transition-colors">
                <div className="flex items-center gap-2">
                  <TerminalIcon className="w-4 h-4 text-[#57606A] dark:text-[#abb2bf]" />
                  <h3 className="font-semibold text-[#1F2328] dark:text-[#e6edf3] text-xs font-mono uppercase tracking-wider">
                    Build Logs
                  </h3>
                </div>
                <span className="text-[10px] font-mono font-semibold px-2 py-0.5 rounded bg-[#CBD1D8] dark:bg-[#2c313a] text-[#24292F] dark:text-[#abb2bf] border border-[#B8BFC6] dark:border-[#3e4451]/60 transition-colors">
                  BuildKit
                </span>
              </div>

              {/* Step indicator strip — only shown during BUILDING */}
              {isBuilding && (() => {
                const latest = deploymentsData?.deployments?.[0];
                const lastLog = (latest?.log_excerpt || "").toLowerCase();
                const step = lastLog.includes("starting") || lastLog.includes("launch") ? 3
                  : lastLog.includes("install") || lastLog.includes("build") || lastLog.includes("mise") ? 2
                  : lastLog.includes("clon") ? 1
                  : 0;
                const steps = ["Cloning", "Installing", "Starting", "Live"];
                return (
                  <div className="flex items-center gap-0 px-4 py-2 border-b border-[#CBD1D8] dark:border-[#31363f] bg-[#E4E7EB] dark:bg-[#21252b] transition-colors">
                    {steps.map((s, i) => (
                      <div key={s} className="flex items-center">
                        <div className={`flex items-center gap-1.5 text-xs font-medium ${
                          i < step ? "text-[#1A7F37] dark:text-[#98c379]" : i === step ? "text-[#0969DA] dark:text-[#61afef] animate-pulse font-semibold" : "text-[#57606A] dark:text-[#5c6370]"
                        }`}>
                          {i < step ? (
                            <Check className="w-3 h-3 text-[#1A7F37] dark:text-[#98c379]" />
                          ) : i === step ? (
                            <Loader2 className="w-3 h-3 animate-spin text-[#0969DA] dark:text-[#61afef]" />
                          ) : (
                            <span className="w-2.5 h-2.5 rounded-full border border-[#8C959F] dark:border-[#5c6370] inline-block" />
                          )}
                          {s}
                        </div>
                        {i < steps.length - 1 && (
                          <span className="mx-2 text-[#8C959F] dark:text-[#5c6370] text-xs">→</span>
                        )}
                      </div>
                    ))}
                  </div>
                );
              })()}

              {/* Failure banner */}
              {isFailed && (
                <div className="flex items-center justify-between px-4 py-2 border-b border-[#CF222E]/30 dark:border-[#e06c75]/30 bg-[#FFEBE9] dark:bg-[#e06c75]/10 text-[#CF222E] dark:text-[#e06c75] text-xs transition-colors">
                  <div className="flex items-center gap-2">
                    <AlertCircle className="w-4 h-4 text-[#CF222E] dark:text-[#e06c75] shrink-0" />
                    <span>Build failed. Review build logs and error output below.</span>
                  </div>
                  <button
                    onClick={handleRestart}
                    disabled={isRestarting || isViewerRole}
                    className="px-2.5 py-1 bg-[#CF222E]/10 dark:bg-[#e06c75]/20 hover:bg-[#CF222E]/20 dark:hover:bg-[#e06c75]/30 border border-[#CF222E]/30 dark:border-[#e06c75]/40 rounded text-[#CF222E] dark:text-white text-[11px] font-medium transition-colors cursor-pointer"
                  >
                    Retry Build
                  </button>
                </div>
              )}

              <div
                ref={terminalRef}
                className="flex-1 w-full bg-[#EAECEF] dark:bg-[#1c1f24] overflow-hidden p-3 [&_.xterm-viewport]:!bg-[#EAECEF] dark:[&_.xterm-viewport]:!bg-[#1c1f24] [&_.xterm-viewport::-webkit-scrollbar]:w-2 [&_.xterm-viewport::-webkit-scrollbar-track]:!bg-[#EAECEF] dark:[&_.xterm-viewport::-webkit-scrollbar-track]:!bg-[#1c1f24] [&_.xterm-viewport::-webkit-scrollbar-thumb]:!bg-[#B8BFC6] hover:[&_.xterm-viewport::-webkit-scrollbar-thumb]:!bg-[#9EA7B0] dark:[&_.xterm-viewport::-webkit-scrollbar-thumb]:!bg-[#3e4451] transition-colors"
              />
            </div>

            {/* App Output */}
            <div className="flex-1 bg-[#EAECEF] dark:bg-[#1c1f24] border border-[#CBD1D8] dark:border-[#31363f] rounded-2xl overflow-hidden shadow-sm dark:shadow-lg flex flex-col transition-colors">
              <div className="bg-[#DFE2E6] dark:bg-[#242830] border-b border-[#CBD1D8] dark:border-[#31363f] px-4 py-3 flex items-center justify-between transition-colors">
                <div className="flex items-center gap-2">
                  <ScrollText className="w-4 h-4 text-[#57606A] dark:text-[#abb2bf]" />
                  <h3 className="font-semibold text-[#1F2328] dark:text-[#e6edf3] text-xs font-mono uppercase tracking-wider">
                    App Output
                  </h3>
                </div>
                <button
                  onClick={() => {
                    fetchDockerLogs();
                    mutateDeployments();
                    mutateEnv();
                  }}
                  disabled={isLoadingDockerLogs}
                  className="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-[#D8DCE2] hover:bg-[#CBD1D8] dark:bg-[#2c313a] dark:hover:bg-[#353b45] border border-[#CBD1D8] dark:border-[#3e4451] text-[#0969DA] dark:text-[#61afef] transition-colors text-[10px] font-semibold tracking-wide uppercase disabled:opacity-50 cursor-pointer"
                >
                  {isLoadingDockerLogs ? (
                    <Loader2 className="w-3 h-3 animate-spin" />
                  ) : (
                    <RefreshCw className="w-3 h-3" />
                  )}
                  Refresh
                </button>
              </div>
              <div className="flex-1 overflow-y-auto bg-[#EAECEF] dark:bg-[#1c1f24] p-4 [&::-webkit-scrollbar]:w-2 [&::-webkit-scrollbar-track]:bg-[#EAECEF] dark:[&::-webkit-scrollbar-track]:bg-[#1c1f24] [&::-webkit-scrollbar-thumb]:bg-[#B8BFC6] hover:[&::-webkit-scrollbar-thumb]:bg-[#9EA7B0] dark:[&::-webkit-scrollbar-thumb]:bg-[#3e4451] dark:[&::-webkit-scrollbar-thumb]:rounded-full transition-colors">
                <pre className="text-xs leading-6 text-[#1A7F37] dark:text-[#98c379] font-mono whitespace-pre-wrap break-all selection:bg-[#CBD1D8] dark:selection:bg-[#3e4451]">
                  {dockerLogs || "No output yet."}
                </pre>
              </div>
            </div>
          </div>
        )}

        {/* Collaborators View */}
        {activeTab === "collaborators" && (
          <div
            className="bg-surface-container-lowest border border-outline-variant rounded-xl overflow-hidden flex flex-col"
            style={{ height: "calc(100vh - 260px)", minHeight: "520px" }}
          >
            {/* Header & Action Bar */}
            <div className="bg-surface-container/60 border-b border-outline-variant px-5 py-3.5 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2">
                  <Users className="w-4 h-4 text-primary-fixed" />
                  <h3 className="font-semibold text-on-surface text-sm">
                    Project Collaborators & Access Control
                  </h3>
                </div>
                <p className="text-[11px] text-on-surface-variant/70 mt-0.5">
                  Manage team workspace permissions (Owner, Editor, Viewer) and invite collaborators.
                </p>
              </div>

              <div className="flex items-center gap-2">
                <button
                  onClick={() => {
                    mutateMembers();
                    mutateProject();
                    toast.success("Collaborator list refreshed");
                  }}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface-container hover:bg-surface-container-high border border-outline-variant text-on-surface-variant hover:text-on-surface transition-colors text-xs font-medium cursor-pointer"
                  title="Refresh list"
                >
                  <RefreshCw className="w-3.5 h-3.5" />
                  Refresh
                </button>

                {isOwnerOrAdmin && !isDefaultProject && (
                  <button
                    onClick={() => setIsInviteModalOpen(true)}
                    className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-primary-fixed/15 hover:bg-primary-fixed/25 border border-primary-fixed/30 text-primary-fixed transition-all text-xs font-semibold shadow-sm cursor-pointer"
                  >
                    <UserPlus className="w-3.5 h-3.5" />
                    Invite Collaborator
                  </button>
                )}
              </div>
            </div>

            {/* Scrollable Main Area */}
            <div className="flex-1 w-full bg-surface-container-lowest overflow-y-auto p-5 space-y-5">
              {/* Personal Space (Default Project) Alert Banner */}
              {isDefaultProject && (
                <div className="p-4 rounded-2xl bg-primary-fixed/10 border border-primary-fixed/30 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 shadow-sm">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-xl bg-primary-fixed/20 border border-primary-fixed/30 flex items-center justify-center text-primary-fixed shrink-0">
                      <Shield className="w-5 h-5 text-primary-fixed" />
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <h4 className="font-bold text-xs uppercase tracking-wider text-primary-fixed">
                          Default Project (Personal Space)
                        </h4>
                        <span className="text-[10px] bg-primary-fixed/20 text-primary-fixed px-2 py-0.5 rounded-full font-bold">
                          Private Space
                        </span>
                      </div>
                      <p className="text-xs text-on-surface-variant mt-0.5 leading-relaxed">
                        This environment is part of your private Default Project and is owned exclusively by you (<strong>{currentUser?.username || "You"}</strong>). It cannot be shared with collaborators. To share containers and work with teammates, create a Shareable Project from the Projects section.
                      </p>
                    </div>
                  </div>
                  <Link
                    href="/projects"
                    className="px-3.5 py-2 rounded-xl bg-surface-container border border-outline-variant hover:border-primary-fixed/40 text-xs font-semibold text-on-surface transition-colors shrink-0 whitespace-nowrap"
                  >
                    Go to Projects →
                  </Link>
                </div>
              )}
              {/* Summary Stats Row */}
              {(() => {
                const totalCount = collaborators.length;
                const ownerCount = collaborators.filter((c: any) => c.role === "OWNER").length;
                const editorCount = collaborators.filter((c: any) => c.role === "EDITOR" || c.role === "COLLABORATOR" || c.role === "ADMIN").length;
                const viewerCount = collaborators.filter((c: any) => c.role === "VIEWER").length;
                const pendingCount = collaborators.filter((c: any) => c.status === "PENDING" || !c.acceptedAt).length;

                return (
                  <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
                    <div className="bg-surface-container/50 border border-outline-variant/60 rounded-xl p-3 flex items-center gap-3">
                      <div className="w-9 h-9 rounded-lg bg-primary-fixed/10 border border-primary-fixed/20 flex items-center justify-center text-primary-fixed shrink-0">
                        <Users className="w-4 h-4" />
                      </div>
                      <div>
                        <div className="text-[11px] font-medium text-on-surface-variant uppercase tracking-wider">Total</div>
                        <div className="text-lg font-bold text-on-surface leading-tight">{totalCount}</div>
                      </div>
                    </div>

                    <div className="bg-surface-container/50 border border-outline-variant/60 rounded-xl p-3 flex items-center gap-3">
                      <div className="w-9 h-9 rounded-lg bg-amber-500/10 border border-amber-500/20 flex items-center justify-center text-amber-500 shrink-0">
                        <Crown className="w-4 h-4" />
                      </div>
                      <div>
                        <div className="text-[11px] font-medium text-on-surface-variant uppercase tracking-wider">Owners</div>
                        <div className="text-lg font-bold text-on-surface leading-tight">{ownerCount}</div>
                      </div>
                    </div>

                    <div className="bg-surface-container/50 border border-outline-variant/60 rounded-xl p-3 flex items-center gap-3">
                      <div className="w-9 h-9 rounded-lg bg-blue-500/10 border border-blue-500/20 flex items-center justify-center text-blue-500 shrink-0">
                        <ShieldCheck className="w-4 h-4" />
                      </div>
                      <div>
                        <div className="text-[11px] font-medium text-on-surface-variant uppercase tracking-wider">Editors</div>
                        <div className="text-lg font-bold text-on-surface leading-tight">{editorCount}</div>
                      </div>
                    </div>

                    <div className="bg-surface-container/50 border border-outline-variant/60 rounded-xl p-3 flex items-center gap-3">
                      <div className="w-9 h-9 rounded-lg bg-slate-500/10 border border-slate-500/20 flex items-center justify-center text-slate-400 shrink-0">
                        <Eye className="w-4 h-4" />
                      </div>
                      <div>
                        <div className="text-[11px] font-medium text-on-surface-variant uppercase tracking-wider">Viewers</div>
                        <div className="text-lg font-bold text-on-surface leading-tight">{viewerCount}</div>
                      </div>
                    </div>

                    <div className="bg-surface-container/50 border border-outline-variant/60 rounded-xl p-3 flex items-center gap-3">
                      <div className="w-9 h-9 rounded-lg bg-orange-500/10 border border-orange-500/20 flex items-center justify-center text-orange-500 shrink-0">
                        <Clock className="w-4 h-4" />
                      </div>
                      <div>
                        <div className="text-[11px] font-medium text-on-surface-variant uppercase tracking-wider">Pending</div>
                        <div className="text-lg font-bold text-on-surface leading-tight">{pendingCount}</div>
                      </div>
                    </div>
                  </div>
                );
              })()}

              {/* Search & Filter Toolbar */}
              <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 bg-surface-container/30 p-2.5 rounded-xl border border-outline-variant/50">
                <div className="relative flex-1 max-w-md">
                  <Search className="w-4 h-4 text-on-surface-variant/50 absolute left-3 top-1/2 -translate-y-1/2" />
                  <input
                    type="text"
                    value={collabSearch}
                    onChange={(e) => setCollabSearch(e.target.value)}
                    placeholder="Search by email, username or role..."
                    className="w-full pl-9 pr-3 py-1.5 text-xs rounded-lg bg-surface-container border border-outline-variant text-on-surface placeholder:text-on-surface-variant/40 focus:border-primary-fixed focus:outline-none focus:ring-1 focus:ring-primary-fixed transition-all"
                  />
                  {collabSearch && (
                    <button
                      onClick={() => setCollabSearch("")}
                      className="absolute right-2.5 top-1/2 -translate-y-1/2 text-[10px] text-on-surface-variant hover:text-on-surface"
                    >
                      Clear
                    </button>
                  )}
                </div>

                <div className="flex items-center gap-1.5 overflow-x-auto pb-1 sm:pb-0">
                  {(["ALL", "OWNER", "EDITOR", "VIEWER", "PENDING"] as const).map((filter) => {
                    const isActive = collabRoleFilter === filter;
                    return (
                      <button
                        key={filter}
                        onClick={() => setCollabRoleFilter(filter)}
                        className={`px-2.5 py-1 rounded-lg text-[11px] font-semibold transition-all whitespace-nowrap cursor-pointer ${
                          isActive
                            ? "bg-primary-fixed/20 text-primary-fixed border border-primary-fixed/30"
                            : "text-on-surface-variant hover:text-on-surface hover:bg-surface-container border border-transparent"
                        }`}
                      >
                        {filter === "ALL" ? "All Roles" : filter.charAt(0) + filter.slice(1).toLowerCase()}
                      </button>
                    );
                  })}
                </div>
              </div>

              {/* Collaborators List */}
              {(() => {
                const filtered = collaborators.filter((collab: any) => {
                  const email = (collab.user?.email || "").toLowerCase();
                  const username = (collab.user?.username || "").toLowerCase();
                  const role = (collab.role || "").toLowerCase();
                  const q = collabSearch.toLowerCase().trim();

                  const matchesSearch = !q || email.includes(q) || username.includes(q) || role.includes(q);

                  if (!matchesSearch) return false;

                  if (collabRoleFilter === "ALL") return true;
                  if (collabRoleFilter === "PENDING") {
                    return collab.status === "PENDING" || !collab.acceptedAt;
                  }
                  if (collabRoleFilter === "OWNER") return collab.role === "OWNER";
                  if (collabRoleFilter === "EDITOR") return collab.role === "EDITOR" || collab.role === "COLLABORATOR" || collab.role === "ADMIN";
                  if (collabRoleFilter === "VIEWER") return collab.role === "VIEWER";
                  return true;
                });

                if (filtered.length === 0) {
                  return (
                    <div className="text-center py-12 px-4 border border-dashed border-outline-variant rounded-xl bg-surface-container/20">
                      <Users className="w-8 h-8 text-on-surface-variant/30 mx-auto mb-2" />
                      <div className="text-sm font-semibold text-on-surface">No collaborators found</div>
                      <p className="text-xs text-on-surface-variant/70 mt-1 max-w-sm mx-auto">
                        {collabSearch || collabRoleFilter !== "ALL"
                          ? "Try clearing your search query or switching role filter tabs."
                          : "Invite team members to collaborate on this preview environment with role-based permissions."}
                      </p>
                      {(collabSearch || collabRoleFilter !== "ALL") && (
                        <button
                          onClick={() => {
                            setCollabSearch("");
                            setCollabRoleFilter("ALL");
                          }}
                          className="mt-3 px-3 py-1.5 text-xs rounded-lg bg-surface-container hover:bg-surface-container-high text-primary-fixed border border-outline-variant font-medium transition-colors cursor-pointer"
                        >
                          Reset Filters
                        </button>
                      )}
                    </div>
                  );
                }

                return (
                  <div className="space-y-3">
                    {filtered.map((collab: any) => {
                      const collabUserId = collab.userId || collab.id;
                      const isMe = currentUser && (currentUser.id === collabUserId || currentUser.email === collab.user?.email);
                      const isTargetOwner = collab.role === "OWNER";
                      const isPending = collab.status === "PENDING" || !collab.acceptedAt;

                      // Permission logic: Can remove if current user is owner (and not target owner) or if removing self
                      const canRemove = isMe || (isOwnerOrAdmin && !isTargetOwner);

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
                          badgeClass: "bg-blue-500/10 text-blue-500 border-blue-500/30",
                          avatarBorder: "border-blue-500/40 bg-blue-500/10 text-blue-500",
                        },
                        ADMIN: {
                          label: "Admin",
                          icon: ShieldCheck,
                          badgeClass: "bg-purple-500/10 text-purple-500 border-purple-500/30",
                          avatarBorder: "border-purple-500/40 bg-purple-500/10 text-purple-500",
                        },
                        COLLABORATOR: {
                          label: "Editor",
                          icon: ShieldCheck,
                          badgeClass: "bg-blue-500/10 text-blue-500 border-blue-500/30",
                          avatarBorder: "border-blue-500/40 bg-blue-500/10 text-blue-500",
                        },
                        VIEWER: {
                          label: "Viewer",
                          icon: Eye,
                          badgeClass: "bg-slate-500/10 text-slate-400 border-slate-500/30",
                          avatarBorder: "border-slate-500/40 bg-slate-500/10 text-slate-400",
                        },
                      };

                      const currentRoleCfg = roleConfig[collab.role] || roleConfig.VIEWER;
                      const RoleIcon = currentRoleCfg.icon;

                      return (
                        <div
                          key={collab.id || collabUserId}
                          className="flex flex-col sm:flex-row sm:items-center justify-between p-4 rounded-xl bg-surface-container/60 hover:bg-surface-container border border-outline-variant transition-all gap-4 shadow-sm"
                        >
                          {/* User Details */}
                          <div className="flex items-center gap-3.5 min-w-0">
                            <div
                              className={`w-11 h-11 rounded-full border-2 flex items-center justify-center font-bold text-sm shrink-0 shadow-inner ${currentRoleCfg.avatarBorder}`}
                            >
                              {(collab.user?.username || collab.user?.email || "?")[0]?.toUpperCase()}
                            </div>

                            <div className="min-w-0">
                              <div className="flex items-center gap-2 flex-wrap">
                                <span className="font-semibold text-on-surface text-sm truncate">
                                  {collab.user?.username || collab.user?.email || "Unknown User"}
                                </span>

                                {isMe && (
                                  <span className="text-[10px] bg-primary-fixed/20 text-primary-fixed px-2 py-0.5 rounded-full font-bold uppercase tracking-wider border border-primary-fixed/30">
                                    You
                                  </span>
                                )}

                                {isPending ? (
                                  <span className="inline-flex items-center gap-1 text-[10px] bg-orange-500/15 text-orange-500 px-2 py-0.5 rounded-full font-semibold border border-orange-500/30">
                                    <Clock className="w-2.5 h-2.5 animate-spin" />
                                    Pending Invite
                                  </span>
                                ) : (
                                  <span className="inline-flex items-center gap-1 text-[10px] bg-emerald-500/15 text-emerald-500 px-2 py-0.5 rounded-full font-semibold border border-emerald-500/30">
                                    <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse" />
                                    Active
                                  </span>
                                )}
                              </div>

                              {(() => {
                                const displayCollabEmail = collab.user?.email || "No email available";
                                return (
                                  <div className="flex items-center gap-2 text-xs text-on-surface-variant/80 mt-0.5">
                                    <span className="truncate">{displayCollabEmail}</span>
                                    {displayCollabEmail && displayCollabEmail !== "No email available" && (
                                      <button
                                        onClick={() => {
                                          navigator.clipboard.writeText(displayCollabEmail);
                                          toast.success("Email copied to clipboard");
                                        }}
                                        className="text-on-surface-variant/50 hover:text-on-surface transition-colors cursor-pointer"
                                        title="Copy email"
                                      >
                                        <Copy className="w-3 h-3" />
                                      </button>
                                    )}
                                  </div>
                                );
                              })()}
                            </div>
                          </div>

                          {/* Role Selector & Actions */}
                          <div className="flex items-center justify-between sm:justify-end gap-3 shrink-0 pt-2 sm:pt-0 border-t sm:border-t-0 border-outline-variant/30">
                            {isPending ? (
                              isOwnerOrAdmin && (
                                <div className="flex items-center gap-2">
                                  <button
                                    onClick={async () => {
                                      try {
                                        const inviteId = collab.inviteId || collab.id;
                                        const res = await fetch(`/api/projects/${env?.projectId}/invites/${inviteId}/resend`, {
                                          method: "POST",
                                        });
                                        if (res.ok) {
                                          toast.success("Invitation resent successfully");
                                          mutateMembers();
                                        } else {
                                          toast.error("Failed to resend invitation");
                                        }
                                      } catch {
                                        toast.error("Error resending invitation");
                                      }
                                    }}
                                    className="px-2.5 py-1.5 rounded-lg bg-surface-container hover:bg-surface-container-high border border-outline-variant text-on-surface-variant hover:text-on-surface text-xs font-semibold transition-colors flex items-center gap-1 cursor-pointer"
                                    title="Resend invitation"
                                  >
                                    <Send className="w-3 h-3" />
                                    <span>Resend</span>
                                  </button>
                                  <button
                                    onClick={async () => {
                                      if (!confirm(`Revoke invitation for ${collab.user?.username || collab.user?.email}?`)) return;
                                      try {
                                        const inviteId = collab.inviteId || collab.id;
                                        const res = await fetch(`/api/projects/${env?.projectId}/invites/${inviteId}/revoke`, {
                                          method: "POST",
                                        });
                                        if (res.ok) {
                                          toast.success("Invitation revoked");
                                          mutateMembers();
                                        } else {
                                          toast.error("Failed to revoke invitation");
                                        }
                                      } catch {
                                        toast.error("Error revoking invitation");
                                      }
                                    }}
                                    className="px-2.5 py-1.5 rounded-lg bg-error/10 hover:bg-error/20 border border-error/20 text-error text-xs font-semibold transition-colors flex items-center gap-1 cursor-pointer"
                                    title="Revoke invitation"
                                  >
                                    <X className="w-3 h-3" />
                                    <span>Revoke</span>
                                  </button>
                                </div>
                              )
                            ) : (
                              <>
                                {/* Role Dropdown (If Owner and not modifying self) */}
                                {isOwnerOrAdmin && !isMe ? (
                                  <div className="flex items-center gap-1.5">
                                    <span className="text-[11px] text-on-surface-variant/60 hidden sm:inline">Role:</span>
                                    <select
                                      value={collab.role === "COLLABORATOR" ? "EDITOR" : collab.role}
                                      onChange={(e) => {
                                        const newRole = e.target.value;
                                        if (confirm(`Change role for ${collab.user?.username || collab.user?.email} to ${newRole}?`)) {
                                          handleUpdateRole(collabUserId, newRole);
                                        }
                                      }}
                                      className="text-xs bg-surface-container font-semibold rounded-lg px-2.5 py-1.5 border border-outline-variant text-on-surface focus:border-primary-fixed focus:outline-none cursor-pointer"
                                    >
                                      <option value="EDITOR">Editor (Edit & Deploy)</option>
                                      <option value="VIEWER">Viewer (Read Only)</option>
                                    </select>
                                  </div>
                                ) : (
                                  <div
                                    className={`inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-semibold border ${currentRoleCfg.badgeClass}`}
                                  >
                                    <RoleIcon className="w-3.5 h-3.5" />
                                    <span>{currentRoleCfg.label}</span>
                                  </div>
                                )}

                                {/* Remove / Leave Action Button */}
                                {canRemove && (
                                  <button
                                    onClick={() => handleRemoveCollaborator(collabUserId, isMe)}
                                    className={`px-2.5 py-1.5 rounded-lg flex items-center gap-1.5 transition-colors text-xs font-medium cursor-pointer ${
                                      isMe
                                        ? "bg-amber-500/10 text-amber-500 hover:bg-amber-500/20 border border-amber-500/20"
                                        : "bg-error/10 text-error hover:bg-error/20 border border-error/20"
                                    }`}
                                    title={isMe ? "Leave this project" : "Remove collaborator"}
                                  >
                                    {isMe ? (
                                      <>
                                        <LogOut className="w-3.5 h-3.5" />
                                        <span>Leave</span>
                                      </>
                                    ) : (
                                      <>
                                        <Trash2 className="w-3.5 h-3.5" />
                                        <span className="hidden sm:inline">Remove</span>
                                      </>
                                    )}
                                  </button>
                                )}
                              </>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                );
              })()}

              {/* Role-Based Access Control (RBAC) Permissions Matrix */}
              <div className="border border-outline-variant/60 rounded-xl bg-surface-container/30 p-4 space-y-3 mt-4">
                <div className="flex items-center gap-2">
                  <Shield className="w-4 h-4 text-primary-fixed" />
                  <h4 className="text-xs font-bold text-on-surface uppercase tracking-wider">
                    Role-Based Access Control (RBAC) Permissions Matrix
                  </h4>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
                  <div className="p-3 rounded-lg bg-surface-container/60 border border-outline-variant/40 space-y-1.5">
                    <div className="flex items-center gap-1.5 font-bold text-amber-500">
                      <Crown className="w-3.5 h-3.5" />
                      <span>Owner</span>
                    </div>
                    <p className="text-[11px] text-on-surface-variant leading-relaxed">
                      Full administrative control. Can invite collaborators, assign/change member roles, transfer ownership, modify environment configurations, and delete sandboxes.
                    </p>
                  </div>

                  <div className="p-3 rounded-lg bg-surface-container/60 border border-outline-variant/40 space-y-1.5">
                    <div className="flex items-center gap-1.5 font-bold text-blue-500">
                      <ShieldCheck className="w-3.5 h-3.5" />
                      <span>Editor</span>
                    </div>
                    <p className="text-[11px] text-on-surface-variant leading-relaxed">
                      Read and write development access. Can create/edit code in Monaco editor, execute terminal commands via xterm.js, commit Git changes, and deploy preview containers.
                    </p>
                  </div>

                  <div className="p-3 rounded-lg bg-surface-container/60 border border-outline-variant/40 space-y-1.5">
                    <div className="flex items-center gap-1.5 font-bold text-slate-400">
                      <Eye className="w-3.5 h-3.5" />
                      <span>Viewer</span>
                    </div>
                    <p className="text-[11px] text-on-surface-variant leading-relaxed">
                      Read-only audit access. Can view live preview URLs, inspect real-time BuildKit logs, stream WebSocket outputs, and review project metrics without making changes.
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Code Workspace View */}
        {activeTab === "workspace" && (
          <div
            className="bg-surface-container-lowest border border-outline-variant rounded-xl overflow-hidden grid grid-cols-12"
            style={{ height: "calc(100vh - 260px)", minHeight: "500px" }}
          >
            {/* File Explorer Sidebar */}
            <div className="col-span-3 border-r border-outline-variant bg-surface-container/40 flex flex-col h-full min-h-0">
              <div className="px-4 py-2.5 border-b border-outline-variant flex items-center justify-between font-medium text-on-surface-variant text-xs tracking-wider uppercase">
                <span>Files</span>
                <div className="flex items-center gap-1.5 normal-case">
                  <button
                    onClick={() => handleCreateFileOrFolder(false)}
                    className="p-1 rounded text-on-surface-variant hover:text-on-surface hover:bg-surface-container transition-all cursor-pointer"
                    title="New File"
                  >
                    <FilePlus className="w-4 h-4" />
                  </button>
                  <button
                    onClick={() => handleCreateFileOrFolder(true)}
                    className="p-1 rounded text-on-surface-variant hover:text-on-surface hover:bg-surface-container transition-all cursor-pointer"
                    title="New Folder"
                  >
                    <FolderPlus className="w-4 h-4" />
                  </button>
                </div>
              </div>
              <div className="flex-1 overflow-y-auto p-2 space-y-1">
                {files && files.length > 0 ? (
                  files.map((node: FileNode) => (
                    <FileTreeItem
                      key={node.path}
                      node={node}
                      onFileSelect={setSelectedFilePath}
                      selectedPath={selectedFilePath}
                      onDelete={handleDeleteFileOrFolder}
                    />
                  ))
                ) : (
                  <div className="p-4 text-center text-on-surface-variant/50 text-xs">
                    {files ? "No files found." : "Loading files..."}
                  </div>
                )}
              </div>

              {/* Git Status Panel */}
              <div className="border-t border-outline-variant p-2 shrink-0 bg-surface-container/20 flex flex-col gap-2">
                <GitStatusPanel envId={id} />
                <CommitHistoryPanel envId={id} />
              </div>
            </div>

            {/* Editor Workspace */}
            <div className="col-span-9 flex flex-col bg-surface-container/10 h-full min-h-0">
              {selectedFilePath ? (
                <>
                  {/* Editor Header Toolbar */}
                  <div className="flex items-center justify-between px-4 py-2 bg-surface-container/50 border-b border-outline-variant shrink-0">
                    <div className="flex items-center gap-2 text-sm text-on-surface font-mono">
                      <File className="w-4 h-4 text-primary-fixed" />
                      <span>{selectedFilePath}</span>
                      {hasUnsavedChanges && (
                        <span className="text-amber-600 dark:text-amber-400 text-xs bg-amber-400/10 border border-amber-400/20 px-1.5 py-0.5 rounded font-sans">
                          Unsaved Changes
                        </span>
                      )}
                    </div>
                    <div className="flex gap-2">
                      {!isEditingFile ? (
                        <button
                          onClick={() => setIsEditingFile(true)}
                          className="flex items-center gap-1.5 px-3 py-1.5 rounded border border-outline-variant text-on-surface hover:bg-surface-container-high transition-colors text-xs font-semibold cursor-pointer"
                        >
                          <Code className="w-3.5 h-3.5" />
                          Edit File
                        </button>
                      ) : (
                        <button
                          onClick={handleSaveFile}
                          disabled={
                            (saveState !== "idle" &&
                              saveState !== "live" &&
                              saveState !== "failed") ||
                            !hasUnsavedChanges
                          }
                          className="flex items-center gap-1.5 px-3 py-1.5 rounded bg-primary/10 border border-primary/30 text-primary hover:bg-primary/20 disabled:opacity-30 disabled:hover:bg-primary/10 transition-colors text-xs font-semibold"
                        >
                          {saveState === "saving" ||
                          saveState === "reloading" ? (
                            <Loader2 className="w-3.5 h-3.5 animate-spin" />
                          ) : (
                            <Save className="w-3.5 h-3.5" />
                          )}
                          Save & Apply
                        </button>
                      )}

                      {/* Dev Loop Status Indicator */}
                      {saveState !== "idle" && (
                        <div
                          className={`flex items-center gap-1.5 px-3 py-1.5 rounded border text-xs font-semibold transition-colors ${
                            saveState === "live"
                              ? "bg-green-500/20 border-green-500/40 text-green-400 shadow-[0_0_10px_rgba(34,197,94,0.3)]"
                              : saveState === "failed"
                                ? "bg-red-500/20 border-red-500/40 text-red-400 shadow-[0_0_10px_rgba(239,68,68,0.3)]"
                                : "bg-amber-500/10 border-amber-500/20 text-amber-400"
                          }`}
                        >
                          {saveState === "saving" && (
                            <>
                              <span>Saving</span>
                              <Loader2 className="w-3.5 h-3.5 animate-spin ml-1" />
                            </>
                          )}
                          {saveState === "reloading" && (
                            <>
                              <span>Reloading</span>
                              <Loader2 className="w-3.5 h-3.5 animate-spin ml-1" />
                            </>
                          )}
                          {saveState === "live" && <span>Live ⚡</span>}
                          {saveState === "failed" && <span>Failed ❌</span>}
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Editor Workspace Input Area */}
                  <div className="flex-1 relative flex overflow-hidden min-h-0 bg-slate-950/90 font-mono">
                    {isLoadingFile ? (
                      <div className="absolute inset-0 flex items-center justify-center bg-slate-950/80 z-10">
                        <Loader2 className="w-8 h-8 text-primary animate-spin" />
                      </div>
                    ) : null}

                    {/* Line Numbers */}
                    <div
                      ref={lineNumbersRef}
                      className="w-12 text-right pr-3 select-none text-white/20 border-r border-white/5 py-4 overflow-hidden text-sm leading-6 shrink-0"
                    >
                      {Array.from({ length: lineCount }).map((_, i) => (
                        <div key={i}>{i + 1}</div>
                      ))}
                    </div>

                    {/* Textarea */}
                    <textarea
                      ref={textareaRef}
                      onScroll={handleScroll}
                      value={fileContent}
                      onChange={(e) => setFileContent(e.target.value)}
                      spellCheck="false"
                      readOnly={!isEditingFile || isViewerRole}
                      className={`flex-1 w-full h-full resize-none py-4 px-3 text-white/90 outline-none overflow-y-auto text-sm leading-6 select-text selection:bg-primary/30 selection:text-white ${!isEditingFile || isViewerRole ? "bg-transparent cursor-text" : "bg-slate-900/50"}`}
                    />
                  </div>
                </>
              ) : (
                <div className="flex-1 flex flex-col items-center justify-center p-8 text-center text-on-surface-variant">
                  <div className="w-16 h-16 rounded-2xl bg-surface-container flex items-center justify-center mb-4 text-on-surface-variant/40 border border-outline-variant">
                    <Code className="w-8 h-8" />
                  </div>
                  <h3 className="text-base font-semibold text-on-surface mb-1">
                    Live Editor Workspace
                  </h3>
                  <p className="text-xs max-w-sm text-on-surface-variant/70 leading-relaxed">
                    Select a file from the sidebar tree explorer to view or
                    modify its contents. Edits are local until you Commit and
                    Sync.
                  </p>
                </div>
              )}
            </div>
          </div>
        )}

        {/* Settings View */}
        {activeTab === "settings" && (
          <div className="pt-4">
            <EnvironmentSettings
              env={env}
              mutate={() => mutate(`/api/environments/${id}`)}
              isViewerRole={isViewerRole}
            />
          </div>
        )}
      </div>

      {/* Team Activity View */}
      {activeTab === "team-activity" && (
        <div
          className="bg-surface-container-lowest border border-outline-variant rounded-xl overflow-hidden flex flex-col"
          style={{ height: "calc(100vh - 260px)", minHeight: "500px" }}
        >
          <TeamCollaborationDashboard projectId={env?.projectId} />
        </div>
      )}
      {/* Invite Collaborator Modal */}
      <InviteCollaboratorModal
        isOpen={isInviteModalOpen}
        onClose={() => setIsInviteModalOpen(false)}
        onInvite={handleInvite}
        isInviting={isInviting}
        projectId={env?.projectId}
        projectName={project?.name || env?.name || "Environment Workspace"}
      />

      {/* Transfer Sandbox Modal */}
      <TransferSandboxModal
        isOpen={isTransferModalOpen}
        onClose={() => setIsTransferModalOpen(false)}
        onTransfer={handleTransfer}
        isTransferring={isTransferring}
        currentProjectId={env?.projectId}
        projects={projects}
        isProjectsLoading={isProjectsLoading}
      />
    </>
  );
}
