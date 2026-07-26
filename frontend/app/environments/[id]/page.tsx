"use client";

import { useEffect, useRef, useState } from "react";
import useSWR from "swr";
import { useParams, useRouter } from "next/navigation";
import toast from "react-hot-toast";
import "xterm/css/xterm.css";
import { Activity, Box, Clock, ExternalLink, GitBranch, Terminal as TerminalIcon, Loader2, Trash2 } from "lucide-react";

import { fetchWithAuth } from "@/lib/auth";

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

export default function EnvironmentDetail() {
  const params = useParams();
  const id = params.id as string;
  const router = useRouter();
  const [isDeleting, setIsDeleting] = useState(false);
  const terminalRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<any>(null);
  
  const { data: env, error } = useSWR(`/api/environments/${id}`, fetcher, {
    refreshInterval: (data) => (data?.status === 'BUILDING' ? 1000 : 5000),
  });

  // Initialize Terminal and SSE
  useEffect(() => {
    if (!terminalRef.current || xtermRef.current) return;

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
        padding: 16,
      });

      fitAddon = new FitAddon();
      term.loadAddon(fitAddon);
      term.open(terminalRef.current!);
      fitAddon.fit();
      xtermRef.current = term;

      if (env?.logs) {
        env.logs.forEach((l: any) => {
          let msg = l.message.replace(/\n$/, '');
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
  }, [env?.id]);

  // Update terminal when new logs arrive via SWR polling
  useEffect(() => {
    if (!env?.logs || !xtermRef.current) return;
    
    const term = xtermRef.current;
    const currentLength = term._logCount || 0;
    
    if (env.logs.length > currentLength) {
      const newLogs = env.logs.slice(currentLength);
      newLogs.forEach((l: any) => {
        let msg = l.message.replace(/\n$/, '');
        term.writeln(`[${l.level.toUpperCase()}] ${msg}`);
      });
      term._logCount = env.logs.length;
    }
  }, [env?.logs?.length]);

  const handleDelete = async () => {
    if (!confirm("Are you sure you want to delete this sandbox? This cannot be undone.")) return;
    setIsDeleting(true);
    try {
      const token = localStorage.getItem("token");
      const res = await fetch(`/api/environments/${id}`, { 
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` }
      });
      if (!res.ok) throw new Error("Failed to delete sandbox");
      toast.success("Sandbox deleted successfully");
      router.push("/");
    } catch (e: any) {
      toast.error(e.message);
      setIsDeleting(false);
    }
  };

  if (error) return <div className="p-8 text-center text-red-400">Failed to load environment</div>;
  if (!env) return <div className="p-8 text-center text-white/50 animate-pulse">Loading environment details...</div>;

  return (
    <div className="max-w-6xl mx-auto space-y-6">
      {/* Header Card */}
      <div className="glass-panel p-8">
        <div className="flex flex-col md:flex-row md:items-start justify-between gap-6">
          <div className="space-y-4">
            <div className="flex items-center gap-4">
              <div className="w-12 h-12 rounded-xl bg-primary/10 flex items-center justify-center ring-1 ring-primary/20">
                <Box className="w-6 h-6 text-primary" />
              </div>
              <div>
                <h1 className="text-3xl font-bold text-white tracking-tight">{env.name}</h1>
                <div className="flex items-center mt-2 text-white/60 text-sm">
                  <Clock className="w-4 h-4 mr-1.5" />
                  ID: <span className="font-mono ml-1 text-white/80">{env.id}</span>
                </div>
              </div>
            </div>
            
            <div className="flex flex-wrap gap-4 mt-4">
              <div className="flex items-center text-sm bg-white/5 px-3 py-1.5 rounded-lg border border-white/10">
                <GitBranch className="w-4 h-4 mr-2 text-white/50" />
                <a href={env.gitUrl} target="_blank" rel="noreferrer" className="hover:text-primary transition-colors">
                  {env.gitUrl.replace('https://github.com/', '')}
                </a>
                <span className="mx-2 text-white/20">/</span>
                <span className="text-white/80">{env.githubBranch}</span>
              </div>
            </div>
          </div>

          <div className="flex flex-col items-end gap-4">
            <div className={`px-4 py-2 rounded-full border text-sm font-semibold tracking-wide flex items-center gap-2 ${statusColors[env.status] || statusColors.IDLE}`}>
              {env.status === 'BUILDING' && <Loader2 className="w-4 h-4 animate-spin" />}
              {env.status}
            </div>
            {env.publicUrl && (
              <a 
                href={env.publicUrl} 
                target="_blank" 
                rel="noreferrer"
                className="glass-button px-5 py-2.5 flex items-center gap-2"
              >
                Open App <ExternalLink className="w-4 h-4" />
              </a>
            )}
            <button
              onClick={handleDelete}
              disabled={isDeleting}
              className="px-5 py-2.5 rounded-lg border border-red-500/30 bg-red-500/10 text-red-400 hover:bg-red-500/20 flex items-center gap-2 transition-colors"
            >
              {isDeleting ? <Loader2 className="w-4 h-4 animate-spin" /> : <Trash2 className="w-4 h-4" />}
              Delete
            </button>
          </div>
        </div>
      </div>

      {/* Terminal View */}
      <div className="glass-panel overflow-hidden flex flex-col">
        <div className="bg-slate-950/50 border-b border-white/10 px-4 py-3 flex items-center gap-2">
          <TerminalIcon className="w-5 h-5 text-white/50" />
          <h3 className="font-medium text-white/80">Build Logs & Output</h3>
        </div>
        <div 
          ref={terminalRef} 
          className="h-[500px] w-full bg-[#0f172a] rounded-b-xl overflow-hidden p-2"
        />
      </div>
    </div>
  );
}
