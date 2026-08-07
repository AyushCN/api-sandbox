"use client";

import { useEffect, useRef, useState } from "react";
import useSWR, { mutate } from "swr";
import { useParams, useRouter } from "next/navigation";
import toast from "react-hot-toast";
import "xterm/css/xterm.css";
import { 
  Activity, Box, Clock, ExternalLink, GitBranch, 
  Terminal as TerminalIcon, Loader2, Trash2, RefreshCw,
  Folder, FolderOpen, File, ChevronRight, ChevronDown, 
  Save, Code, Check, AlertCircle, FilePlus, FolderPlus, ScrollText, X, Users
} from "lucide-react";

import { fetchWithAuth } from "@/lib/auth";
import GitTreeVisualizer from "@/components/GitTreeVisualizer";

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
  onDelete
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
            {isOpen ? <ChevronDown className="w-3.5 h-3.5 text-white/40 shrink-0" /> : <ChevronRight className="w-3.5 h-3.5 text-white/40 shrink-0" />}
            {isOpen ? <FolderOpen className="w-4 h-4 text-sky-400 shrink-0" /> : <Folder className="w-4 h-4 text-sky-400 shrink-0" />}
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
        <File className={`w-3.5 h-3.5 shrink-0 ${isSelected ? "text-primary" : "text-white/40"}`} />
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
  const xtermRef = useRef<any>(null);
  
  // Tab control
  const [activeTab, setActiveTab] = useState<"logs" | "workspace" | "collaborators" | "git-tree">("logs");
  
  // File explorer states
  const [selectedFilePath, setSelectedFilePath] = useState<string>("");
  const [fileContent, setFileContent] = useState<string>("");
  const [originalFileContent, setOriginalFileContent] = useState<string>("");
  const [isLoadingFile, setIsLoadingFile] = useState<boolean>(false);
  const [isSavingFile, setIsSavingFile] = useState<boolean>(false);
  const [isEditingFile, setIsEditingFile] = useState<boolean>(false);

  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const lineNumbersRef = useRef<HTMLDivElement>(null);

  // Docker Logs Modal state
  const [isDockerLogsOpen, setIsDockerLogsOpen] = useState<boolean>(false);
  const [dockerLogs, setDockerLogs] = useState<string>("");
  const [isLoadingDockerLogs, setIsLoadingDockerLogs] = useState<boolean>(false);

  // Invite Modal state
  const [isInviteModalOpen, setIsInviteModalOpen] = useState(false);
  const [inviteIdentifier, setInviteIdentifier] = useState("");
  const [inviteRole, setInviteRole] = useState("COLLABORATOR");
  const [isInviting, setIsInviting] = useState(false);

  const fetchDockerLogs = async () => {
    setIsLoadingDockerLogs(true);
    setIsDockerLogsOpen(true);
    try {
      const token = localStorage.getItem("token");
      const res = await fetch(`/api/environments/${id}/docker-logs`, {
        credentials: "include"
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({ error: "Failed to fetch logs" }));
        throw new Error(data.error || "Failed to fetch container logs");
      }
      const text = await res.text();
      setDockerLogs(text || "(No output yet. The container might still be starting.)");
    } catch (e: any) {
      setDockerLogs(`Error: ${e.message}`);
    } finally {
      setIsLoadingDockerLogs(false);
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
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          identifier: inviteIdentifier.trim(),
          role: inviteRole
        })
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
    } catch (e: any) {
      toast.error(e.message);
    } finally {
      setIsInviting(false);
    }
  };

  const { data: env, error } = useSWR(`/api/environments/${id}`, fetcher, {
    refreshInterval: (data) => (data?.status === 'BUILDING' ? 1000 : 5000),
  });

  const { data: files } = useSWR(
    activeTab === "workspace" ? `/api/environments/${id}/files` : null,
    fetcher
  );

  const { data: project } = useSWR(
    activeTab === "collaborators" && env?.projectId ? `/api/projects/${env.projectId}` : null,
    fetcher
  );

  const { data: currentUser } = useSWR("/api/user/me", fetcher);

  const handleRemoveCollaborator = async (userId: string) => {
    if (!env?.projectId) return;
    if (!confirm("Are you sure you want to remove this collaborator?")) return;
    
    try {
      const token = localStorage.getItem("token");
      const res = await fetch(`/api/projects/${env.projectId}/collaborators/${userId}`, {
        method: "DELETE",
        credentials: "include"
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || "Failed to remove collaborator");
      }
      toast.success("Collaborator removed successfully");
      mutate(`/api/projects/${env.projectId}`);
    } catch (e: any) {
      toast.error(e.message);
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
        const res = await fetch(`/api/environments/${id}/files/content?path=${encodeURIComponent(selectedFilePath)}`, {
          credentials: "include"
        });
        if (!res.ok) throw new Error("Failed to load file content");
        const data = await res.json();
        setFileContent(data.content);
        setOriginalFileContent(data.content);
        setIsEditingFile(false);
      } catch (e: any) {
        toast.error(e.message);
        setSelectedFilePath("");
      } finally {
        setIsLoadingFile(false);
      }
    };

    fetchFile();
  }, [selectedFilePath, id]);

  const handleSaveFile = async () => {
    if (!selectedFilePath) return;
    setIsSavingFile(true);
    try {
      const token = localStorage.getItem("token");
      const res = await fetch(`/api/environments/${id}/files/content`, {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          path: selectedFilePath,
          content: fileContent
        })
      });
      if (!res.ok) throw new Error("Failed to save changes");
      
      toast.success("File saved and container reloaded!");
      setOriginalFileContent(fileContent);
      setIsEditingFile(false);
      
      // Clear logs to reflect container restart log sequence
      if (xtermRef.current) {
        xtermRef.current.clear();
        xtermRef.current._logCount = 0;
      }
      
      // Mutate env cache to update environment status immediately
      mutate(`/api/environments/${id}`);
    } catch (e: any) {
      toast.error(e.message);
    } finally {
      setIsSavingFile(false);
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
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          path: name.trim(),
          isDir
        })
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || `Failed to create ${typeStr}`);
      }

      toast.success(`${typeStr} created successfully!`);
      // Re-fetch files list
      mutate(`/api/environments/${id}/files`);
    } catch (e: any) {
      toast.error(e.message);
    }
  };

  const handleDeleteFileOrFolder = async (pathToDelete: string, e: React.MouseEvent) => {
    e.stopPropagation();
    if (!confirm(`Are you sure you want to delete "${pathToDelete}"? This will restart the container.`)) return;

    try {
      const token = localStorage.getItem("token");
      const res = await fetch(`/api/environments/${id}/files/delete`, {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          path: pathToDelete
        })
      });

      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.error || "Failed to delete file or folder");
      }

      toast.success("Deleted successfully!");
      // If we deleted the currently active file (or its parent directory), clear editor
      if (selectedFilePath === pathToDelete || selectedFilePath.startsWith(pathToDelete + "/")) {
        setSelectedFilePath("");
        setFileContent("");
        setOriginalFileContent("");
      }
      
      // Refresh files list
      mutate(`/api/environments/${id}/files`);
      // Refresh logs
      mutate(`/api/environments/${id}`);
    } catch (e: any) {
      toast.error(e.message);
    }
  };

  // Initialize Terminal and SSE
  useEffect(() => {
    if (!terminalRef.current || xtermRef.current || activeTab !== "logs") return;

    let term: any;
    let fitAddon: any;

    const initTerminal = async () => {
      const { Terminal } = await import("xterm");
      const { FitAddon } = await import("xterm-addon-fit");
      
      term = new Terminal({
        theme: {
          background: '#0f172a', // Tailwind slate-900
          foreground: '#f8fafc',
          cursor: '#3b82f6',
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
        env.logs.forEach((l: any) => {
          const msg = l.message.replace(/\n$/, '');
          term.writeln(`[${l.level.toUpperCase()}] ${msg}`);
        });
        term._logCount = env.logs.length;
      } else {
        term.writeln("\x1b[36mWaiting for logs...\x1b[0m");
      }
    };
    initTerminal();

    const handleResize = () => { if (fitAddon) fitAddon.fit(); };
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
    
    const term = xtermRef.current;
    const currentLength = term._logCount || 0;
    
    if (env.logs.length > currentLength) {
      const newLogs = env.logs.slice(currentLength);
      newLogs.forEach((l: any) => {
        const msg = l.message.replace(/\n$/, '');
        term.writeln(`[${l.level.toUpperCase()}] ${msg}`);
      });
      term._logCount = env.logs.length;
    }
  }, [env?.logs?.length, activeTab]);

  const handleDelete = async () => {
    if (!confirm("Are you sure you want to delete this sandbox? This cannot be undone.")) return;
    setIsDeleting(true);
    try {
      const token = localStorage.getItem("token");
      const res = await fetch(`/api/environments/${id}`, { 
        method: 'DELETE',
        credentials: "include"
      });
      if (!res.ok) throw new Error("Failed to delete sandbox");
      toast.success("Sandbox deleted successfully");
      router.push("/dashboard");
    } catch (e: any) {
      toast.error(e.message);
      setIsDeleting(false);
    }
  };

  const handleRestart = async () => {
    if (!confirm("Are you sure you want to restart this sandbox? It will pull the latest code and rebuild the image.")) return;
    setIsRestarting(true);
    try {
      const token = localStorage.getItem("token");
      const res = await fetch(`/api/environments/${id}/restart`, { 
        method: 'POST',
        credentials: "include"
      });
      if (!res.ok) throw new Error("Failed to restart sandbox");
      toast.success("Sandbox restart initiated");
      if (xtermRef.current) {
        xtermRef.current.clear();
        xtermRef.current._logCount = 0;
      }
    } catch (e: any) {
      toast.error(e.message);
    } finally {
      setIsRestarting(false);
    }
  };

  const lineCount = fileContent.split("\n").length;
  const hasUnsavedChanges = fileContent !== originalFileContent;

  if (error) return <div className="p-8 text-center text-red-400">Failed to load environment</div>;
  if (!env) return <div className="p-8 text-center text-white/50 animate-pulse">Loading environment details...</div>;

  return (
    <>
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
              <h1 className="text-xl font-bold text-on-surface tracking-tight truncate">{env.name}</h1>
              <div className="flex items-center gap-3 mt-1 flex-wrap">
                <div className="flex items-center gap-1.5 text-xs text-on-surface-variant/70 font-mono">
                  <GitBranch className="w-3.5 h-3.5" />
                  <a href={env.gitUrl} target="_blank" rel="noreferrer" className="hover:text-primary-fixed transition-colors truncate max-w-[200px]">
                    {env.gitUrl.replace('https://github.com/', '')}
                  </a>
                  <span className="text-outline-variant">·</span>
                  <span className="font-bold text-on-surface">{env.githubBranch}</span>
                </div>
                <div className="flex items-center gap-1 text-[10px] text-on-surface-variant/50 font-mono">
                  <Clock className="w-3 h-3" />
                  <span>{env.id.slice(0, 8)}...</span>
                </div>
              </div>
            </div>
          </div>

          {/* Right: status + actions */}
          <div className="flex items-center gap-3 shrink-0 flex-wrap">
            <div className={`px-3 py-1.5 rounded-full border text-xs font-bold tracking-wider uppercase flex items-center gap-2 ${statusColors[env.status] || statusColors.IDLE}`}>
              {env.status === 'BUILDING' && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
              {env.status === 'RUNNING' && <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse shadow-[0_0_6px_#34d399]" />}
              {env.status}
            </div>
            {env.publicUrl && env.status === 'RUNNING' && (
              <a href={env.publicUrl} target="_blank" rel="noreferrer" className="px-4 py-1.5 rounded-lg border border-outline-variant text-on-surface-variant hover:text-on-surface hover:border-primary-fixed/30 flex items-center gap-1.5 text-xs font-semibold transition-colors">
                Open App <ExternalLink className="w-3.5 h-3.5" />
              </a>
            )}
            <button
              onClick={handleRestart}
              disabled={isRestarting || env.status === 'BUILDING'}
              className="px-4 py-1.5 rounded-lg border border-primary-fixed/30 bg-primary-fixed/5 text-primary-fixed hover:bg-primary-fixed/15 flex items-center gap-1.5 transition-colors disabled:opacity-50 text-xs font-semibold"
            >
              {isRestarting ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
              Restart
            </button>
            <button
              onClick={handleDelete}
              disabled={isDeleting}
              className="px-4 py-1.5 rounded-lg border border-red-500/30 bg-red-500/5 text-red-400 hover:bg-red-500/15 flex items-center gap-1.5 transition-colors disabled:opacity-50 text-xs font-semibold"
            >
              {isDeleting ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Trash2 className="w-3.5 h-3.5" />}
              Delete
            </button>
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
          onClick={() => setActiveTab("git-tree")}
          className={`pb-3 text-sm font-medium transition-all relative ${
            activeTab === "git-tree" 
              ? "text-primary-fixed border-b-2 border-primary-fixed" 
              : "text-on-surface-variant hover:text-on-surface"
          }`}
        >
          <span className="flex items-center gap-2">
            <GitBranch className="w-4 h-4" />
            Git Tree
          </span>
        </button>
      </div>

      {/* Logs View */}
      {activeTab === "logs" && (
        <div className="bg-surface-container-lowest border border-outline-variant rounded-xl overflow-hidden flex flex-col" style={{ height: 'calc(100vh - 260px)', minHeight: '420px' }}>
          <div className="bg-surface-container/60 border-b border-outline-variant px-4 py-3 flex items-center gap-2">
            <TerminalIcon className="w-4 h-4 text-on-surface-variant/50" />
            <h3 className="font-medium text-on-surface-variant text-sm">Build Logs & Output</h3>
          </div>
          <div 
            ref={terminalRef} 
            className="flex-1 w-full bg-[#0a0e17] overflow-hidden p-2"
          />
        </div>
      )}

      {/* Collaborators View */}
      {activeTab === "collaborators" && (
        <div className="bg-surface-container-lowest border border-outline-variant rounded-xl overflow-hidden flex flex-col" style={{ height: 'calc(100vh - 260px)', minHeight: '420px' }}>
          <div className="bg-surface-container/60 border-b border-outline-variant px-4 py-3 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Users className="w-4 h-4 text-on-surface-variant/50" />
              <h3 className="font-medium text-on-surface-variant text-sm">Project Collaborators</h3>
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
              <div className="flex justify-center items-center h-full text-white/50 animate-pulse">Loading collaborators...</div>
            ) : (
              <div className="max-w-3xl mx-auto space-y-4">
                {project.collaborators?.map((collab: any) => {
                  const isMe = currentUser && currentUser.id === collab.userId;
                  const myCollab = project.collaborators.find((c: any) => c.userId === currentUser?.id);
                  const iAmAdminOrOwner = myCollab && (myCollab.role === 'ADMIN' || myCollab.role === 'OWNER');
                  const canRemove = isMe || (iAmAdminOrOwner && collab.role !== 'OWNER');

                  return (
                    <div key={collab.id} className="flex items-center justify-between p-4 rounded-xl bg-surface-container border border-outline-variant">
                      <div className="flex items-center gap-3">
                        <div className="w-10 h-10 rounded-full bg-primary/20 flex items-center justify-center text-primary font-bold">
                          {collab.user?.email?.[0]?.toUpperCase() || '?'}
                        </div>
                        <div>
                          <div className="font-semibold text-white flex items-center gap-2">
                            {collab.user?.email || 'Unknown User'}
                            {isMe && <span className="text-[10px] bg-primary-fixed/20 text-primary-fixed px-1.5 py-0.5 rounded font-bold uppercase">You</span>}
                            {!collab.acceptedAt && <span className="text-[10px] bg-orange-500/20 text-orange-400 px-1.5 py-0.5 rounded font-bold uppercase">Pending Invite</span>}
                          </div>
                          <div className="text-xs text-white/50">Role: {collab.role}</div>
                        </div>
                      </div>
                      {canRemove && (
                        <button
                          onClick={() => handleRemoveCollaborator(collab.userId)}
                          className="w-8 h-8 rounded-lg flex items-center justify-center text-error hover:bg-error/10 transition-colors"
                          title={isMe ? "Leave Project" : "Remove Collaborator"}
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      )}
                    </div>
                  );
                })}
                {(!project.collaborators || project.collaborators.length === 0) && (
                  <div className="text-center text-white/40">No collaborators found.</div>
                )}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Code Workspace View */}
      {activeTab === "workspace" && (
        <div className="bg-surface-container-lowest border border-outline-variant rounded-xl overflow-hidden grid grid-cols-12" style={{ height: 'calc(100vh - 260px)', minHeight: '500px' }}>
          {/* File Explorer Sidebar */}
          <div className="col-span-3 border-r border-outline-variant bg-surface-container/40 flex flex-col h-full">
            <div className="px-4 py-2.5 border-b border-outline-variant flex items-center justify-between font-medium text-on-surface-variant text-xs tracking-wider uppercase">
              <span>Files</span>
              <div className="flex items-center gap-1.5 normal-case">
                <button
                  onClick={fetchDockerLogs}
                  className="flex items-center gap-1 px-2 py-0.5 rounded bg-blue-600/20 border border-blue-500/40 text-blue-400 hover:bg-blue-600/40 hover:text-blue-300 transition-all text-xs font-semibold tracking-normal normal-case"
                  title="View App Logs"
                >
                  <ScrollText className="w-3.5 h-3.5" />
                  App Logs
                </button>
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
          </div>

          {/* Editor Workspace */}
          <div className="col-span-9 flex flex-col bg-slate-950/20 h-full">
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
                        disabled={isSavingFile || !hasUnsavedChanges}
                        className="flex items-center gap-1.5 px-3 py-1.5 rounded bg-primary/10 border border-primary/30 text-primary hover:bg-primary/20 disabled:opacity-30 disabled:hover:bg-primary/10 transition-colors text-xs font-semibold"
                      >
                        {isSavingFile ? (
                          <Loader2 className="w-3.5 h-3.5 animate-spin" />
                        ) : (
                          <Save className="w-3.5 h-3.5" />
                        )}
                        Save & Apply
                      </button>
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
                    className="w-12 text-right pr-3 select-none text-white/20 border-r border-white/5 py-4 overflow-hidden text-sm leading-6"
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
                    readOnly={!isEditingFile}
                    className={`flex-1 resize-none py-4 px-3 text-white/90 outline-none overflow-y-auto text-sm leading-6 select-text selection:bg-primary/30 selection:text-white ${!isEditingFile ? 'bg-transparent cursor-text' : 'bg-slate-900/50'}`}
                  />
                </div>
              </>
            ) : (
              <div className="flex-1 flex flex-col items-center justify-center p-8 text-center text-white/40">
                <Code className="w-12 h-12 mb-4 text-white/10" />
                <h3 className="text-base font-semibold text-white/60 mb-1">Live Editor Workspace</h3>
                <p className="text-xs max-w-sm text-white/30">
                  Select a file from the sidebar tree explorer to view or modify its contents inside the running environment container.
                </p>
              </div>
            )}
          </div>
        </div>
      )}
    </div>

      {/* Docker Logs Modal */}
      {isDockerLogsOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ backgroundColor: 'rgba(0,0,0,0.75)' }}>
          <div className="w-full max-w-4xl max-h-[85vh] flex flex-col rounded-2xl border border-white/10 bg-slate-900 shadow-2xl overflow-hidden">
            {/* Modal Header */}
            <div className="flex items-center justify-between px-5 py-3.5 bg-slate-950/80 border-b border-white/10 shrink-0">
              <div className="flex items-center gap-2.5">
                <ScrollText className="w-5 h-5 text-blue-400" />
                <h2 className="text-sm font-semibold text-white">Container App Logs</h2>
                <span className="text-xs text-white/40 font-mono">api-sandbox-env-{id}</span>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={fetchDockerLogs}
                  disabled={isLoadingDockerLogs}
                  className="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-blue-600/20 border border-blue-500/30 text-blue-400 hover:bg-blue-600/30 transition-colors text-xs font-semibold disabled:opacity-50"
                >
                  {isLoadingDockerLogs ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
                  Refresh
                </button>
                <button
                  onClick={() => setIsDockerLogsOpen(false)}
                  className="p-1.5 rounded-lg text-white/40 hover:text-white hover:bg-white/10 transition-colors"
                >
                  <X className="w-4 h-4" />
                </button>
              </div>
            </div>

            {/* Modal Body - Log output */}
            <div className="flex-1 overflow-y-auto bg-slate-950/90">
              {isLoadingDockerLogs ? (
                <div className="flex items-center justify-center h-48 gap-3 text-white/50">
                  <Loader2 className="w-6 h-6 animate-spin text-blue-400" />
                  <span className="text-sm">Fetching logs...</span>
                </div>
              ) : (
                <pre className="p-5 text-xs leading-6 text-green-300/90 font-mono whitespace-pre-wrap break-all">
                  {dockerLogs}
                </pre>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Git Tree View */}
      {activeTab === "git-tree" && (
        <div className="bg-surface-container-lowest border border-outline-variant rounded-xl p-6">
          <div className="mb-4 flex items-center justify-between">
            <h3 className="font-semibold text-lg text-on-surface">Git Tree Visualizer</h3>
            <div className="text-xs text-on-surface-variant flex gap-4">
              <span className="flex items-center gap-1.5"><div className="w-3 h-3 bg-teal-700 border border-teal-500 rounded-sm"></div> Github Commits</span>
              <span className="flex items-center gap-1.5"><div className="w-3 h-3 bg-cyan-700 border border-cyan-400 rounded-sm"></div> Local Commits</span>
              <span className="flex items-center gap-1.5"><div className="w-3 h-3 bg-cyan-900/50 border border-dashed border-cyan-400 rounded-sm"></div> Uncommitted Edits</span>
            </div>
          </div>
          <GitTreeVisualizer environmentId={id} />
        </div>
      )}

      {/* Invite Collaborator Modal */}
      {isInviteModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ backgroundColor: 'rgba(0,0,0,0.75)' }}>
          <div className="w-full max-w-md bg-surface-container-lowest rounded-2xl border border-outline-variant shadow-2xl overflow-hidden">
            <div className="flex items-center justify-between px-5 py-4 border-b border-outline-variant bg-surface-container/30">
              <h3 className="font-semibold text-on-surface">Invite Collaborator</h3>
              <button onClick={() => setIsInviteModalOpen(false)} className="text-on-surface-variant hover:text-on-surface">
                <X className="w-5 h-5" />
              </button>
            </div>
            <form onSubmit={handleInvite} className="p-5 space-y-4">
              <div>
                <label className="block text-xs font-medium text-on-surface-variant mb-1.5">Email or Username</label>
                <input
                  type="text"
                  required
                  value={inviteIdentifier}
                  onChange={(e) => setInviteIdentifier(e.target.value)}
                  placeholder="e.g. user@example.com or username"
                  className="w-full bg-surface-container border border-outline-variant rounded-lg px-3 py-2 text-sm text-on-surface placeholder:text-on-surface-variant/50 focus:outline-none focus:border-primary-fixed/50"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-on-surface-variant mb-1.5">Role</label>
                <select
                  value={inviteRole}
                  onChange={(e) => setInviteRole(e.target.value)}
                  className="w-full bg-surface-container border border-outline-variant rounded-lg px-3 py-2 text-sm text-on-surface focus:outline-none focus:border-primary-fixed/50 appearance-none"
                >
                  <option value="ADMIN">Admin</option>
                  <option value="COLLABORATOR">Collaborator</option>
                  <option value="VIEWER">Viewer</option>
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
                  className="flex items-center gap-2 px-4 py-2 rounded-lg bg-primary-fixed text-on-primary-fixed hover:bg-primary-fixed/90 font-medium text-sm transition-colors disabled:opacity-50"
                >
                  {isInviting && <Loader2 className="w-4 h-4 animate-spin" />}
                  Send Invite
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  );
}
