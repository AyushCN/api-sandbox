"use client";

import { useEffect, useRef, useState } from "react";
import useSWR, { mutate } from "swr";
import { useParams, useRouter } from "next/navigation";
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
  Folder,
  FolderOpen,
  File,
  ChevronRight,
  ChevronDown,
  Save,
  Code,
  Check,
  AlertCircle,
  FilePlus,
  FolderPlus,
  ScrollText,
  X,
  Users,
  DownloadCloud,
  Search,
  Plus,
  Settings as SettingsIcon,
} from "lucide-react";

import { fetchWithAuth } from "@/lib/auth";
import TeamCollaborationDashboard from "@/components/TeamCollaborationDashboard";
import EnvironmentSettings from "@/components/EnvironmentSettings";
import { useEnvironmentChanges } from "@/hooks/useEnvironmentChanges";
import ActiveEditors from "@/components/ActiveEditors";
import { CommitModal, BranchPicker, GitStatusPanel, CommitHistoryPanel } from "@/components/GitUI";

const fetcher = async (url: string) => {
  return fetchWithAuth(url);
};

const statusColors: Record<string, string> = {
  IDLE: "text-gray-400 bg-gray-400/10 border-gray-400/20",
  BUILDING: "text-blue-400 bg-blue-400/10 border-blue-400/20 animate-pulse",
  RUNNING: "text-emerald-400 bg-emerald-400/10 border-emerald-400/20",
  STOPPED: "text-orange-400 bg-orange-400/10 border-orange-400/20",
  FAILED: "text-red-400 bg-red-400/10 border-red-400/20",
};

interface FileNode {
  name: string;
  path: string;
  isDir: boolean;
  children?: FileNode[];
}

function FileTreeItem({
  node,
  onFileSelect,
  selectedPath,
  onDelete,
}: {
  node: FileNode;
  onFileSelect: (path: string) => void;
  selectedPath: string;
  onDelete: (path: string, e: React.MouseEvent) => void;
}) {
  const [isOpen, setIsOpen] = useState(false);

  if (node.isDir) {
    return (
      <div className="pl-1">
        <div className="group flex items-center justify-between hover:bg-white/5 rounded px-2">
          <button
            onClick={() => setIsOpen(!isOpen)}
            className="flex items-center gap-1.5 py-1.5 text-white/70 hover:text-white text-sm flex-1 text-left min-w-0 transition-colors"
          >
            {isOpen ? (
              <ChevronDown className="w-3.5 h-3.5 text-white/40 shrink-0" />
            ) : (
              <ChevronRight className="w-3.5 h-3.5 text-white/40 shrink-0" />
            )}
            {isOpen ? (
              <FolderOpen className="w-4 h-4 text-sky-400 shrink-0" />
            ) : (
              <Folder className="w-4 h-4 text-sky-400 shrink-0" />
            )}
            <span className="truncate">{node.name}</span>
          </button>
          <button
            onClick={(e) => onDelete(node.path, e)}
            className="opacity-0 group-hover:opacity-100 text-white/40 hover:text-red-400 p-0.5 rounded transition-opacity shrink-0 ml-1"
            title="Delete Folder"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        </div>
        {isOpen && node.children && (
          <div className="border-l border-white/5 ml-3.5 pl-1.5">
            {node.children.map((child) => (
              <FileTreeItem
                key={child.path}
                node={child}
                onFileSelect={onFileSelect}
                selectedPath={selectedPath}
                onDelete={onDelete}
              />
            ))}
          </div>
        )}
      </div>
    );
  }

  const isSelected = selectedPath === node.path;
  return (
    <div className="group flex items-center justify-between hover:bg-white/5 rounded transition-all">
      <button
        onClick={() => onFileSelect(node.path)}
        className={`flex items-center gap-2 py-1.5 pl-6 flex-1 text-sm text-left min-w-0 transition-all ${
          isSelected
            ? "text-primary font-semibold border-l-2 border-primary"
            : "text-white/60 hover:text-white"
        }`}
      >
        <File
          className={`w-3.5 h-3.5 shrink-0 ${isSelected ? "text-primary" : "text-white/40"}`}
        />
        <span className="truncate">{node.name}</span>
      </button>
      <button
        onClick={(e) => onDelete(node.path, e)}
        className="opacity-0 group-hover:opacity-100 text-white/40 hover:text-red-400 p-0.5 rounded transition-opacity shrink-0 mr-2 ml-1"
        title="Delete File"
      >
        <Trash2 className="w-3.5 h-3.5" />
      </button>
    </div>
  );
}

export default function EnvironmentDetail() {
  const params = useParams();
  const id = params.id as string;
  const router = useRouter();
  const [isDeleting, setIsDeleting] = useState(false);
  const [isRestarting, setIsRestarting] = useState(false);
  const terminalRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<unknown>(null);

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
      await handlePush();
      setIsCommitModalOpen(false);
    } catch (e) {
      // Errors are toasted in the individual handlers
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

  // Docker Logs Modal state
  const [isDockerLogsOpen, setIsDockerLogsOpen] = useState<boolean>(false);
  const [dockerLogs, setDockerLogs] = useState<string>("");
  const [isLoadingDockerLogs, setIsLoadingDockerLogs] =
    useState<boolean>(false);

  // Invite Modal state
  const [isInviteModalOpen, setIsInviteModalOpen] = useState(false);
  const [inviteIdentifier, setInviteIdentifier] = useState("");
  const [inviteRole, setInviteRole] = useState("COLLABORATOR");
  const [isInviting, setIsInviting] = useState(false);

  // Transfer Modal state
  const [isTransferModalOpen, setIsTransferModalOpen] = useState(false);
  const [transferProjectId, setTransferProjectId] = useState("");
  const [isTransferring, setIsTransferring] = useState(false);
  const [isForking, setIsForking] = useState(false);
  const { data: projects, isLoading: isProjectsLoading } = useSWR(
    "/api/projects",
    fetcher,
  );

  const [userSearchResults, setUserSearchResults] = useState<
    { id: string; username: string; email: string }[]
  >([]);
  const [isSearchingUsers, setIsSearchingUsers] = useState(false);
  const [showUserDropdown, setShowUserDropdown] = useState(false);

  useEffect(() => {
    if (!inviteIdentifier.trim() || inviteIdentifier.trim().length < 2) {
      setTimeout(() => {
        setUserSearchResults([]);
        setShowUserDropdown(false);
      }, 0);
      return;
    }

    const timer = setTimeout(async () => {
      setIsSearchingUsers(true);
      try {
        const token = localStorage.getItem("token");
        const res = await fetch(
          `/api/users/search?q=${encodeURIComponent(inviteIdentifier.trim())}`,
          {
            headers: {
              Authorization: `Bearer ${token}`,
            },
          },
        );
        if (res.ok) {
          const data = await res.json();
          if (data.length === 1 && data[0].email === inviteIdentifier.trim()) {
            setShowUserDropdown(false);
          } else {
            setUserSearchResults(data || []);
            setShowUserDropdown(true);
          }
        }
      } catch (err) {
        // fail silently for search
      } finally {
        setIsSearchingUsers(false);
      }
    }, 300); // 300ms debounce

    return () => clearTimeout(timer);
  }, [inviteIdentifier]);

  const fetchDockerLogs = async () => {
    setIsLoadingDockerLogs(true);
    try {
      const res = await fetch(`/api/environments/${id}/docker-logs`, {
        credentials: "include",
      });
      if (!res.ok) {
        const data = await res
          .json()
          .catch(() => ({ error: "Failed to fetch logs" }));
        throw new Error(data.error || "Failed to fetch container logs");
      }
      const text = await res.text();
      setDockerLogs(
        text || "(No output yet. The container might still be starting.)",
      );
    } catch (e: unknown) {
      setDockerLogs(`Error: ${(e as Error).message}`);
    } finally {
      setIsLoadingDockerLogs(false);
    }
  };

  useEffect(() => {
    if (activeTab === "logs") {
      setTimeout(() => {
        fetchDockerLogs();
      }, 0);
      const interval = setInterval(fetchDockerLogs, 3000);
      return () => clearInterval(interval);
    }
  }, [activeTab]);


  const handleTransfer = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!transferProjectId) return;
    setIsTransferring(true);
    try {
      await fetchWithAuth(`/api/environments/${id}/transfer`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ projectId: transferProjectId }),
      });
      toast.success("Sandbox transferred successfully!");
      setIsTransferModalOpen(false);
      mutate(`/api/environments/${id}`);
      mutate(`/api/environments?projectId=${env?.projectId}`); // old project
      mutate(`/api/environments?projectId=${transferProjectId}`); // new project
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

  const handleInvite = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!inviteIdentifier.trim() || !env?.projectId) return;
    setIsInviting(true);
    try {
      const token = localStorage.getItem("token");
      const res = await fetch(`/api/projects/${env.projectId}/invite`, {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          identifier: inviteIdentifier.trim(),
          role: inviteRole,
        }),
      });

      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || "Failed to invite collaborator");
      }

      toast.success("Collaborator invited successfully!");
      setIsInviteModalOpen(false);
      setInviteIdentifier("");
      setInviteRole("COLLABORATOR");
      mutate(`/api/projects/${env.projectId}`);
    } catch (e: unknown) {
      toast.error((e as Error).message);
    } finally {
      setIsInviting(false);
    }
  };

  const { data: env, error } = useSWR(`/api/environments/${id}`, fetcher, {
    refreshInterval: (data) => (data?.status === "BUILDING" ? 1000 : 5000),
  });

  // Auto-switch to logs tab when environment is BUILDING or FAILED
  useEffect(() => {
    if (env?.status === "BUILDING" || env?.status === "FAILED") {
      setActiveTab("logs");
    }
  }, [env?.status]);

  const { data: files } = useSWR(
    activeTab === "workspace" ? `/api/environments/${id}/files` : null,
    fetcher,
  );

  const { data: project } = useSWR(
    env?.projectId ? `/api/projects/${env.projectId}` : null,
    fetcher,
  );

  const { data: currentUser } = useSWR("/api/user/me", fetcher);

  const isEnvOwner = currentUser && env && currentUser.id === env.userId;
  const myCollab = project?.collaborators?.find(
    (c: { userId: string; role: string }) => c.userId === currentUser?.id,
  );
  const isViewerRole = !isEnvOwner && myCollab && myCollab.role === "VIEWER";

  const handleRemoveCollaborator = async (userId: string) => {
    if (!env?.projectId) return;
    if (!confirm("Are you sure you want to remove this collaborator?")) return;

    try {
      const token = localStorage.getItem("token");
      const res = await fetch(
        `/api/projects/${env.projectId}/collaborators/${userId}`,
        {
          method: "DELETE",
          credentials: "include",
        },
      );
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || "Failed to remove collaborator");
      }
      toast.success("Collaborator removed successfully");
      mutate(`/api/projects/${env.projectId}`);
    } catch (e: unknown) {
      toast.error((e as Error).message);
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
        const token = localStorage.getItem("token");
        const res = await fetch(
          `/api/environments/${id}/files/content?path=${encodeURIComponent(selectedFilePath)}`,
          {
            credentials: "include",
          },
        );
        if (!res.ok) throw new Error("Failed to load file content");
        const data = await res.json();
        setFileContent(data.content);
        setOriginalFileContent(data.content);
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
      const res = await fetch(`/api/environments/${id}/files/content`, {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          path: selectedFilePath,
          content: fileContent,
        }),
      });
      if (!res.ok) throw new Error("Failed to save changes");
      const data = await res.json();
      if (data.reloadSignaled) {
        setSaveState("reloading");
      } else {
        toast.success(
          data.message ?? "Saved — runtime not running (no reload signal)",
        );
        setSaveState("idle");
      }
      setOriginalFileContent(fileContent);
      setIsEditingFile(false);

      // Mutate env cache to update environment status immediately
      mutate(`/api/environments/${id}`);
    } catch (e: unknown) {
      toast.error((e as Error).message);
      setSaveState("idle");
    }
  };

  const handleCreateFileOrFolder = async (isDir: boolean) => {
    const typeStr = isDir ? "Folder" : "File";
    const name = prompt(`Enter path/name of new ${typeStr}:`);
    if (!name || !name.trim()) return;

    try {
      const token = localStorage.getItem("token");
      const res = await fetch(`/api/environments/${id}/files/create`, {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          path: name.trim(),
          isDir,
        }),
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || `Failed to create ${typeStr}`);
      }

      toast.success(`${typeStr} created successfully!`);
      // Re-fetch files list
      mutate(`/api/environments/${id}/files`);
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

  // Initialize Terminal and SSE
  useEffect(() => {
    if (!terminalRef.current || xtermRef.current || activeTab !== "logs")
      return;

    let term: {
      writeln: (data: string) => void;
      loadAddon: (addon: import("xterm").ITerminalAddon) => void;
      open: (parent: HTMLElement) => void;
      dispose: () => void;
      clear: () => void;
      _logCount?: number;
    };
    let fitAddon: { fit: () => void } & import("xterm").ITerminalAddon;

    const initTerminal = async () => {
      const { Terminal } = await import("xterm");
      const { FitAddon } = await import("xterm-addon-fit");

      term = new Terminal({
        theme: {
          background: "#0f172a", // Tailwind slate-900
          foreground: "#f8fafc",
          cursor: "#3b82f6",
        },
        fontFamily: 'Menlo, Monaco, "Courier New", monospace',
        fontSize: 14,
        convertEol: true,
        cursorBlink: true,
      });

      fitAddon = new FitAddon();
      term.loadAddon(fitAddon);
      term.open(terminalRef.current!);
      fitAddon.fit();
      xtermRef.current = term;

      if (env?.logs) {
        const sortedLogs = [...env.logs].reverse();
        sortedLogs.forEach((l: { message: string; level: string }) => {
          const msg = l.message.replace(/\n$/, "");
          term.writeln(`[${l.level.toUpperCase()}] ${msg}`);
        });
        term._logCount = env.logs.length;
      } else {
        term.writeln("\x1b[36mWaiting for logs...\x1b[0m");
      }
    };
    initTerminal();

    const handleResize = () => {
      if (fitAddon) fitAddon.fit();
    };
    window.addEventListener("resize", handleResize);

    return () => {
      if (term) term.dispose();
      xtermRef.current = null;
      window.removeEventListener("resize", handleResize);
    };
  }, [env?.id, activeTab]);

  // Update terminal when new logs arrive
  useEffect(() => {
    if (!env?.logs || !xtermRef.current || activeTab !== "logs") return;

    const term = xtermRef.current as {
      writeln: (data: string) => void;
      loadAddon: (addon: import("xterm").ITerminalAddon) => void;
      open: (parent: HTMLElement) => void;
      dispose: () => void;
      clear: () => void;
      _logCount?: number;
    };
    const currentLength = term._logCount || 0;

    if (env.logs.length > currentLength) {
      const sortedLogs = [...env.logs].reverse();
      const newLogs = sortedLogs.slice(currentLength);
      newLogs.forEach((l: { message: string; level: string }) => {
        const msg = l.message.replace(/\n$/, "");
        term.writeln(`[${l.level.toUpperCase()}] ${msg}`);
      });
      term._logCount = env.logs.length;
    }
  }, [env?.logs?.length, activeTab]);

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
      const token = localStorage.getItem("token");
      const res = await fetch(`/api/environments/${id}`, {
        method: "DELETE",
        credentials: "include",
      });
      if (!res.ok) throw new Error("Failed to delete sandbox");
      toast.success("Sandbox deleted successfully");
      mutate(`/api/environments`);
      mutate(`/api/environments?projectId=${env?.projectId}`);
      mutate(`/api/projects/${env?.projectId}`);
      router.push("/dashboard");
    } catch (e: unknown) {
      toast.error((e as Error).message);
      setIsDeleting(false);
    }
  };

  const handleRestart = async () => {
    if (
      !confirm(
        "Are you sure you want to restart this sandbox? It will pull the latest code and start the dev runtime.",
      )
    )
      return;
    setIsRestarting(true);
    try {
      const token = localStorage.getItem("token");
      const res = await fetch(`/api/environments/${id}/restart`, {
        method: "POST",
        credentials: "include",
      });
      if (!res.ok) throw new Error("Failed to restart sandbox");
      toast.success("Sandbox restart initiated");
      mutate(`/api/environments/${id}`);
      if (xtermRef.current) {
        (xtermRef.current as { clear: () => void }).clear();
        (xtermRef.current as { _logCount?: number })._logCount = 0;
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
      <div className="p-8 text-center text-white/50 animate-pulse">
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
                      href={env.gitUrl}
                      target="_blank"
                      rel="noreferrer"
                      className="hover:text-primary-fixed transition-colors truncate max-w-[200px]"
                    >
                      {env.gitUrl.replace("https://github.com/", "")}
                    </a>
                    <span className="text-outline-variant">·</span>
                    {!isViewerRole ? (
                      <BranchPicker
                        envId={id}
                        currentBranch={env.githubBranch}
                        onBranchChanged={() =>
                          mutate(`/api/environments/${id}`)
                        }
                      />
                    ) : (
                      <span className="bg-primary-fixed/10 text-primary-fixed px-2 py-0.5 rounded text-[10px]">
                        {env.githubBranch}
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-1 text-[10px] text-on-surface-variant/50 font-mono">
                    <Clock className="w-3 h-3" />
                    <span>{env.id.slice(0, 8)}...</span>
                  </div>
                  <ActiveEditors editors={activeEditors} />
                </div>
              </div>
            </div>

            {/* Right: status + actions */}
            <div className="flex items-center gap-3 shrink-0 flex-wrap">
              <div
                className={`px-3 py-1.5 rounded-full border text-xs font-bold tracking-wider uppercase flex items-center gap-2 ${statusColors[env.status] || statusColors.IDLE}`}
              >
                {env.status === "BUILDING" && (
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                )}
                {env.status === "RUNNING" && (
                  <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse shadow-[0_0_6px_#34d399]" />
                )}
                {env.status}
              </div>
              {env.status === "FAILED" && (
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
              {env.publicUrl && env.status === "RUNNING" && (
                <a
                  href={env.publicUrl}
                  target="_blank"
                  rel="noreferrer"
                  className="px-4 py-1.5 rounded-lg border border-outline-variant text-on-surface-variant hover:text-on-surface hover:border-primary-fixed/30 flex items-center gap-1.5 text-xs font-semibold transition-colors"
                >
                  Ephemeral Preview URL <ExternalLink className="w-3.5 h-3.5" />
                </a>
              )}
              <button
                onClick={handleRestart}
                disabled={
                  isRestarting || env.status === "BUILDING" || isViewerRole
                }
                className={`px-4 py-1.5 rounded-lg border bg-primary-fixed/5 flex items-center gap-1.5 transition-colors text-xs font-semibold ${isViewerRole ? "border-outline-variant/30 text-on-surface-variant/30 cursor-not-allowed" : "border-primary-fixed/30 text-primary-fixed hover:bg-primary-fixed/15 disabled:opacity-50"}`}
              >
                {isRestarting ? (
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                ) : (
                  <RefreshCw className="w-3.5 h-3.5" />
                )}
                {env.status === "STOPPED" ? "Re-clone & Restart" : "Restart"}
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
            </div>
          </div>
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
            <div className="flex-1 bg-surface-container-lowest border border-outline-variant rounded-xl overflow-hidden flex flex-col">
              <div className="bg-surface-container/60 border-b border-outline-variant px-4 py-3 flex items-center gap-2">
                <TerminalIcon className="w-4 h-4 text-on-surface-variant/50" />
                <h3 className="font-medium text-on-surface-variant text-sm">
                  Build Logs
                </h3>
              </div>

              {/* Step indicator strip — only shown during BUILDING */}
              {env?.status === "BUILDING" && (() => {
                const lastLog = (env.logs && env.logs.length > 0)
                  ? (env.logs[env.logs.length - 1]?.message || "").toLowerCase()
                  : "";
                const step = lastLog.includes("starting") ? 3
                  : lastLog.includes("install") ? 2
                  : lastLog.includes("clon") ? 1
                  : 0;
                const steps = ["Cloning", "Installing", "Starting", "Live"];
                return (
                  <div className="flex items-center gap-0 px-4 py-2 border-b border-white/5 bg-[#0d1020]">
                    {steps.map((s, i) => (
                      <div key={s} className="flex items-center">
                        <div className={`flex items-center gap-1.5 text-xs font-medium ${
                          i < step ? "text-emerald-400" : i === step ? "text-blue-400 animate-pulse" : "text-white/20"
                        }`}>
                          {i < step ? (
                            <Check className="w-3 h-3" />
                          ) : i === step ? (
                            <Loader2 className="w-3 h-3 animate-spin" />
                          ) : (
                            <span className="w-3 h-3 rounded-full border border-white/20 inline-block" />
                          )}
                          {s}
                        </div>
                        {i < steps.length - 1 && (
                          <span className="mx-2 text-white/15 text-xs">→</span>
                        )}
                      </div>
                    ))}
                  </div>
                );
              })()}

              <div
                ref={terminalRef}
                className="flex-1 w-full bg-[#0a0e17] overflow-hidden p-2"
              />
            </div>

            {/* App Output */}
            <div className="flex-1 bg-surface-container-lowest border border-outline-variant rounded-xl overflow-hidden flex flex-col">
              <div className="bg-surface-container/60 border-b border-outline-variant px-4 py-3 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <ScrollText className="w-4 h-4 text-on-surface-variant/50" />
                  <h3 className="font-medium text-on-surface-variant text-sm">
                    App Output
                  </h3>
                </div>
                <button
                  onClick={fetchDockerLogs}
                  disabled={isLoadingDockerLogs}
                  className="flex items-center gap-1.5 px-3 py-1 rounded bg-blue-600/10 border border-blue-500/20 text-blue-400 hover:bg-blue-600/20 transition-colors text-[10px] font-semibold tracking-wide uppercase disabled:opacity-50"
                >
                  {isLoadingDockerLogs ? (
                    <Loader2 className="w-3 h-3 animate-spin" />
                  ) : (
                    <RefreshCw className="w-3 h-3" />
                  )}
                  Refresh
                </button>
              </div>
              <div className="flex-1 overflow-y-auto bg-slate-950/90 p-4">
                <pre className="text-xs leading-6 text-green-300/90 font-mono whitespace-pre-wrap break-all">
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
            style={{ height: "calc(100vh - 260px)", minHeight: "420px" }}
          >
            <div className="bg-surface-container/60 border-b border-outline-variant px-4 py-3 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Users className="w-4 h-4 text-on-surface-variant/50" />
                <h3 className="font-medium text-on-surface-variant text-sm">
                  Project Collaborators
                </h3>
              </div>
              <button
                onClick={() => setIsInviteModalOpen(true)}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-primary-fixed/10 border border-primary-fixed/20 text-primary-fixed hover:bg-primary-fixed/20 transition-colors text-xs font-semibold"
              >
                <Users className="w-3.5 h-3.5" />
                Invite Collaborator
              </button>
            </div>
            <div className="flex-1 w-full bg-[#0a0e17] overflow-y-auto p-6">
              {!project ? (
                <div className="flex justify-center items-center h-full text-white/50 animate-pulse">
                  Loading collaborators...
                </div>
              ) : (
                <div className="max-w-3xl mx-auto space-y-4">
                  {project.collaborators?.map(
                    (collab: {
                      id: string;
                      userId: string;
                      role: string;
                      acceptedAt?: string;
                      user?: {
                        email: string;
                        username?: string;
                        avatarUrl?: string;
                      };
                    }) => {
                      const isMe =
                        currentUser && currentUser.id === collab.userId;
                      const myCollab = project.collaborators.find(
                        (c: { userId: string; role: string }) =>
                          c.userId === currentUser?.id,
                      );
                      const iAmAdminOrOwner =
                        myCollab &&
                        (myCollab.role === "ADMIN" ||
                          myCollab.role === "OWNER");
                      const canRemove =
                        isMe || (iAmAdminOrOwner && collab.role !== "OWNER");

                      return (
                        <div
                          key={collab.id}
                          className="flex items-center justify-between p-4 rounded-xl bg-surface-container border border-outline-variant"
                        >
                          <div className="flex items-center gap-3">
                            <div className="w-10 h-10 rounded-full bg-primary/20 flex items-center justify-center text-primary font-bold">
                              {collab.user?.email?.[0]?.toUpperCase() || "?"}
                            </div>
                            <div>
                              <div className="font-semibold text-white flex items-center gap-2">
                                {collab.user?.email || "Unknown User"}
                                {isMe && (
                                  <span className="text-[10px] bg-primary-fixed/20 text-primary-fixed px-1.5 py-0.5 rounded font-bold uppercase">
                                    You
                                  </span>
                                )}
                                {!collab.acceptedAt && (
                                  <span className="text-[10px] bg-orange-500/20 text-orange-400 px-1.5 py-0.5 rounded font-bold uppercase">
                                    Pending Invite
                                  </span>
                                )}
                              </div>
                              <div className="text-xs text-white/50">
                                Role: {collab.role}
                              </div>
                            </div>
                          </div>
                          {canRemove && (
                            <button
                              onClick={() =>
                                handleRemoveCollaborator(collab.userId)
                              }
                              className="w-8 h-8 rounded-lg flex items-center justify-center text-error hover:bg-error/10 transition-colors"
                              title={
                                isMe ? "Leave Project" : "Remove Collaborator"
                              }
                            >
                              <Trash2 className="w-4 h-4" />
                            </button>
                          )}
                        </div>
                      );
                    },
                  )}
                  {(!project.collaborators ||
                    project.collaborators.length === 0) && (
                    <div className="text-center text-white/40">
                      No collaborators found.
                    </div>
                  )}
                </div>
              )}
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
                    className="p-1 rounded text-white/40 hover:text-white hover:bg-white/5 transition-all"
                    title="New File"
                  >
                    <FilePlus className="w-4 h-4" />
                  </button>
                  <button
                    onClick={() => handleCreateFileOrFolder(true)}
                    className="p-1 rounded text-white/40 hover:text-white hover:bg-white/5 transition-all"
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
                  <div className="p-4 text-center text-white/40 text-xs">
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
            <div className="col-span-9 flex flex-col bg-slate-950/20 h-full min-h-0">
              {selectedFilePath ? (
                <>
                  {/* Editor Header Toolbar */}
                  <div className="flex items-center justify-between px-4 py-2 bg-slate-950/50 border-b border-white/10 shrink-0">
                    <div className="flex items-center gap-2 text-sm text-white/80 font-mono">
                      <File className="w-4 h-4 text-primary" />
                      <span>{selectedFilePath}</span>
                      {hasUnsavedChanges && (
                        <span className="text-amber-400 text-xs bg-amber-400/10 border border-amber-400/20 px-1.5 py-0.5 rounded font-sans">
                          Unsaved Changes
                        </span>
                      )}
                    </div>
                    <div className="flex gap-2">
                      {!isEditingFile ? (
                        <button
                          onClick={() => setIsEditingFile(true)}
                          className="flex items-center gap-1.5 px-3 py-1.5 rounded border border-white/20 text-white/80 hover:bg-white/10 transition-colors text-xs font-semibold"
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
                <div className="flex-1 flex flex-col items-center justify-center p-8 text-center text-white/40">
                  <Code className="w-12 h-12 mb-4 text-white/10" />
                  <h3 className="text-base font-semibold text-white/60 mb-1">
                    Live Editor Workspace
                  </h3>
                  <p className="text-xs max-w-sm text-white/30">
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
      {isInviteModalOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4"
          style={{ backgroundColor: "rgba(0,0,0,0.75)" }}
        >
          <div className="w-full max-w-md bg-surface-container-lowest rounded-2xl border border-outline-variant shadow-2xl overflow-hidden">
            <div className="flex items-center justify-between px-5 py-4 border-b border-outline-variant bg-surface-container/30">
              <h3 className="font-semibold text-on-surface">
                Invite Collaborator
              </h3>
              <button
                onClick={() => setIsInviteModalOpen(false)}
                className="text-on-surface-variant hover:text-on-surface"
              >
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleInvite} className="p-5 space-y-4">
              <div className="relative">
                <label className="text-sm font-bold tracking-wide text-on-surface-variant uppercase mb-1.5 block">
                  Search User
                </label>
                <div className="relative">
                  <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-on-surface-variant/50" />
                  <input
                    type="text"
                    value={inviteIdentifier}
                    onChange={(e) => {
                      setInviteIdentifier(e.target.value);
                    }}
                    onFocus={() => setShowUserDropdown(true)}
                    placeholder="Email or username"
                    className="w-full bg-surface-container pl-9 pr-4 py-3 rounded-lg border border-outline-variant text-on-surface focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed transition-all"
                  />
                  {isSearchingUsers && (
                    <div className="absolute right-3 top-1/2 -translate-y-1/2">
                      <Loader2 className="w-4 h-4 animate-spin text-primary-fixed" />
                    </div>
                  )}
                </div>

                {/* Dropdown for search results */}
                {showUserDropdown && userSearchResults.length > 0 && (
                  <div className="absolute z-10 w-full mt-1 bg-surface-container-lowest border border-outline-variant rounded-lg shadow-xl max-h-48 overflow-y-auto">
                    {userSearchResults.map(
                      (user: {
                        id: string;
                        username: string;
                        email: string;
                      }) => (
                        <button
                          key={user.id}
                          type="button"
                          onClick={() => {
                            setInviteIdentifier(user.username || user.email);
                            setShowUserDropdown(false);
                          }}
                          className="w-full text-left px-4 py-2.5 hover:bg-primary-fixed/10 transition-colors flex items-center justify-between group"
                        >
                          <div className="flex items-center gap-2">
                            <div className="w-6 h-6 rounded bg-primary-container text-on-primary-fixed flex items-center justify-center text-xs font-bold shrink-0">
                              {(user.username ||
                                user.email ||
                                "?")[0].toUpperCase()}
                            </div>
                            <div className="min-w-0">
                              <p className="text-sm font-medium text-on-surface truncate group-hover:text-primary-fixed transition-colors">
                                {user.username || "No Username"}
                              </p>
                              <p className="text-xs text-on-surface-variant truncate">
                                {user.email}
                              </p>
                            </div>
                          </div>
                          <Plus className="w-4 h-4 text-on-surface-variant group-hover:text-primary-fixed opacity-0 group-hover:opacity-100 transition-all shrink-0 ml-2" />
                        </button>
                      ),
                    )}
                  </div>
                )}
              </div>

              <div>
                <label className="text-sm font-bold tracking-wide text-on-surface-variant uppercase mb-1.5 block">
                  Role
                </label>
                <select
                  value={inviteRole}
                  onChange={(e) => setInviteRole(e.target.value)}
                  className="w-full bg-surface-container px-4 py-3 rounded-lg border border-outline-variant text-on-surface focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed transition-all"
                >
                  <option value="COLLABORATOR">
                    Collaborator (Edit & Push)
                  </option>
                  <option value="VIEWER">Viewer (Read Only)</option>
                  <option value="ADMIN">Admin (Manage Team)</option>
                </select>
              </div>
              <div className="pt-2 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setIsInviteModalOpen(false)}
                  className="px-4 py-2 text-sm font-medium text-on-surface-variant hover:text-on-surface"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isInviting || !inviteIdentifier.trim()}
                  className="px-5 py-2 bg-primary-container text-on-primary-fixed-variant rounded-lg font-bold hover:shadow-[0_0_15px_rgba(0,240,255,0.2)] disabled:opacity-50 disabled:pointer-events-none flex items-center gap-2"
                >
                  {isInviting && <Loader2 className="w-4 h-4 animate-spin" />}
                  Send Invite
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Transfer Sandbox Modal */}
      {isTransferModalOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4"
          style={{ backgroundColor: "rgba(0,0,0,0.75)" }}
        >
          <div className="w-full max-w-md bg-surface-container-lowest rounded-2xl border border-outline-variant shadow-2xl overflow-hidden">
            <div className="flex items-center justify-between px-5 py-4 border-b border-outline-variant bg-surface-container/30">
              <h3 className="font-semibold text-on-surface">
                Transfer Sandbox
              </h3>
              <button
                onClick={() => setIsTransferModalOpen(false)}
                className="text-on-surface-variant hover:text-on-surface"
              >
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleTransfer} className="p-5 space-y-4">
              <div>
                <label className="text-sm font-bold tracking-wide text-on-surface-variant uppercase mb-1.5 block">
                  Select Destination Project
                </label>
                {isProjectsLoading ? (
                  <div className="w-full bg-surface-container px-4 py-3 rounded-lg border border-outline-variant text-on-surface opacity-50 flex items-center">
                    <Loader2 className="w-4 h-4 mr-2 animate-spin" /> Loading
                    projects...
                  </div>
                ) : (
                  <select
                    value={transferProjectId}
                    onChange={(e) => setTransferProjectId(e.target.value)}
                    className="w-full bg-surface-container px-4 py-3 rounded-lg border border-outline-variant text-on-surface focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed transition-all"
                  >
                    <option value="" disabled>
                      -- Select a Project --
                    </option>
                    {projects
                      ?.filter((p: { id: string }) => p.id !== env?.projectId)
                      .map((p: { id: string; name: string }) => (
                        <option key={p.id} value={p.id}>
                          {p.name}
                        </option>
                      ))}
                  </select>
                )}
                <p className="text-xs text-on-surface-variant mt-2">
                  Transferring this sandbox will instantly grant access to all
                  collaborators in the destination project.
                </p>
              </div>
              <div className="pt-2 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setIsTransferModalOpen(false)}
                  className="px-4 py-2 text-sm font-medium text-on-surface-variant hover:text-on-surface"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isTransferring || !transferProjectId}
                  className="px-5 py-2 bg-indigo-500/10 text-indigo-400 border border-indigo-500/30 rounded-lg font-bold hover:bg-indigo-500/20 disabled:opacity-50 disabled:pointer-events-none flex items-center gap-2"
                >
                  {isTransferring && (
                    <Loader2 className="w-4 h-4 animate-spin" />
                  )}
                  Confirm Transfer
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  );
}
