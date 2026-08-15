"use client";

import { useState, useEffect, useRef } from "react";
import { GitBranch, Loader2, Save, X, Plus, Check } from "lucide-react";
import toast from "react-hot-toast";

export function CommitModal({
  isOpen,
  onClose,
  onCommit,
  onCommitAndPush,
  isCommitting,
  isPushing,
  hasUncommittedChanges
}: {
  isOpen: boolean;
  onClose: () => void;
  onCommit: (msg: string) => Promise<void>;
  onCommitAndPush: (msg: string) => Promise<void>;
  isCommitting: boolean;
  isPushing: boolean;
  hasUncommittedChanges: boolean;
}) {
  const [message, setMessage] = useState("");

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div className="bg-[#1a1c23] border border-outline-variant rounded-xl shadow-2xl w-full max-w-md overflow-hidden flex flex-col">
        <div className="flex items-center justify-between p-4 border-b border-white/10">
          <h2 className="text-lg font-bold text-white flex items-center gap-2">
            <Save className="w-5 h-5 text-amber-500" />
            Commit Changes
          </h2>
          <button onClick={onClose} className="text-white/50 hover:text-white transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>
        
        <div className="p-4 flex flex-col gap-4">
          {!hasUncommittedChanges && (
            <div className="p-3 bg-amber-500/10 border border-amber-500/30 rounded-lg text-amber-500 text-sm">
              Warning: You have no uncommitted changes detected.
            </div>
          )}
          <div>
            <label className="block text-sm font-medium text-white/70 mb-1">Commit Message</label>
            <textarea
              autoFocus
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              placeholder="What did you change?"
              className="w-full h-24 bg-[#0d0e12] border border-white/10 rounded-lg p-3 text-sm text-white focus:outline-none focus:border-primary-fixed focus:ring-1 focus:ring-primary-fixed transition-all resize-none"
            />
          </div>
        </div>

        <div className="p-4 bg-white/5 border-t border-white/10 flex items-center justify-end gap-3">
          <button
            onClick={onClose}
            className="px-4 py-2 rounded-lg text-sm font-medium text-white/70 hover:text-white hover:bg-white/5 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={() => onCommit(message)}
            disabled={!message.trim() || isCommitting || isPushing}
            className="px-4 py-2 rounded-lg text-sm font-bold bg-white/10 text-white hover:bg-white/20 border border-white/10 transition-colors disabled:opacity-50 flex items-center gap-2"
          >
            {isCommitting ? <Loader2 className="w-4 h-4 animate-spin" /> : null}
            Commit Only
          </button>
          <button
            onClick={() => onCommitAndPush(message)}
            disabled={!message.trim() || isCommitting || isPushing}
            className="px-4 py-2 rounded-lg text-sm font-bold bg-[#2ea44f] text-white hover:bg-[#2ea44f]/90 transition-colors disabled:opacity-50 flex items-center gap-2 shadow-[0_0_10px_rgba(46,164,79,0.2)]"
          >
            {isPushing ? <Loader2 className="w-4 h-4 animate-spin" /> : null}
            Commit & Push
          </button>
        </div>
      </div>
    </div>
  );
}

export function BranchPicker({
  envId,
  currentBranch,
  onBranchChanged
}: {
  envId: string;
  currentBranch: string;
  onBranchChanged: () => void;
}) {
  const [isOpen, setIsOpen] = useState(false);
  const [branches, setBranches] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [isCheckingOut, setIsCheckingOut] = useState(false);
  const [newBranchName, setNewBranchName] = useState("");
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const fetchBranches = async () => {
    setIsLoading(true);
    try {
      const res = await fetch(`/api/environments/${envId}/git/branches`, {
        headers: { "Authorization": `Bearer ${localStorage.getItem("token")}` }
      });
      const data = await res.json();
      if (res.ok) {
        setBranches(data.branches || []);
      }
    } catch (e) {
      console.error(e);
    } finally {
      setIsLoading(false);
    }
  };

  const handleToggle = () => {
    const nextOpen = !isOpen;
    setIsOpen(nextOpen);
    if (nextOpen) {
      fetchBranches();
    }
  };

  const handleCheckout = async (branch: string) => {
    setIsCheckingOut(true);
    try {
      // Prompt for stash/discard if we have uncommitted changes (this could be improved to a real dialog, but prompt is okay for this edge case constraint)
      const res = await fetch(`/api/environments/${envId}/git/checkout`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${localStorage.getItem("token")}`
        },
        body: JSON.stringify({ ref: branch })
      });
      const data = await res.json();
      
      if (!res.ok) {
        if (data.error && data.error.includes("Working tree is dirty")) {
           const force = confirm("Working tree is dirty. Do you want to force checkout and discard your changes?");
           if (force) {
             const forceRes = await fetch(`/api/environments/${envId}/git/checkout`, {
               method: "POST",
               headers: {
                 "Content-Type": "application/json",
                 "Authorization": `Bearer ${localStorage.getItem("token")}`
               },
               body: JSON.stringify({ ref: branch, force: true })
             });
             if (!forceRes.ok) throw new Error((await forceRes.json()).error);
           } else {
             return;
           }
        } else {
          throw new Error(data.error || "Checkout failed");
        }
      }
      
      toast.success(`Checked out ${branch}`);
      setIsOpen(false);
      onBranchChanged();
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setIsCheckingOut(false);
    }
  };

  const handleCreateBranch = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newBranchName.trim()) return;
    setIsCreating(true);
    try {
      const res = await fetch(`/api/environments/${envId}/git/branch`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": `Bearer ${localStorage.getItem("token")}`
        },
        body: JSON.stringify({ branch: newBranchName.trim() })
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || "Failed to create branch");
      
      toast.success(`Created branch ${newBranchName}`);
      setNewBranchName("");
      setIsOpen(false);
      onBranchChanged();
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setIsCreating(false);
    }
  };

  // Clean branch name from remotes
  const displayBranch = currentBranch.replace("remotes/origin/", "");

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        onClick={handleToggle}
        className="flex items-center gap-1.5 px-2 py-1 rounded bg-white/5 hover:bg-white/10 text-xs font-mono font-bold text-on-surface border border-white/10 transition-colors"
      >
        <GitBranch className="w-3.5 h-3.5 text-primary-fixed" />
        {displayBranch}
      </button>

      {isOpen && (
        <div className="absolute top-full left-0 mt-1 w-64 bg-[#1a1c23] border border-outline-variant rounded-lg shadow-xl overflow-hidden z-50">
          <div className="p-2 border-b border-white/10">
            <form onSubmit={handleCreateBranch} className="relative">
              <input
                type="text"
                placeholder="Create new branch..."
                value={newBranchName}
                onChange={(e) => setNewBranchName(e.target.value)}
                className="w-full bg-[#0d0e12] border border-white/10 rounded px-2 py-1.5 text-xs text-white placeholder-white/30 focus:outline-none focus:border-primary-fixed pr-8"
              />
              <button
                type="submit"
                disabled={!newBranchName.trim() || isCreating}
                className="absolute right-1 top-1/2 -translate-y-1/2 p-1 text-white/50 hover:text-white disabled:opacity-50"
              >
                {isCreating ? <Loader2 className="w-3 h-3 animate-spin" /> : <Plus className="w-3 h-3" />}
              </button>
            </form>
          </div>
          <div className="max-h-48 overflow-y-auto p-1">
            {isLoading ? (
              <div className="flex justify-center p-4">
                <Loader2 className="w-4 h-4 animate-spin text-white/30" />
              </div>
            ) : branches.length === 0 ? (
              <div className="text-xs text-white/30 p-2 text-center">No branches found</div>
            ) : (
              branches.map((b) => {
                const isCurrent = b === currentBranch || b === `remotes/origin/${currentBranch}`;
                return (
                  <button
                    key={b}
                    onClick={() => !isCurrent && handleCheckout(b)}
                    disabled={isCheckingOut}
                    className={`w-full text-left px-2 py-1.5 rounded text-xs font-mono flex items-center justify-between ${
                      isCurrent ? "bg-primary-fixed/10 text-primary-fixed" : "text-white/70 hover:bg-white/5 hover:text-white"
                    }`}
                  >
                    <span className="truncate">{b.replace("remotes/origin/", "")}</span>
                    {isCurrent && <Check className="w-3.5 h-3.5" />}
                  </button>
                );
              })
            )}
          </div>
        </div>
      )}
    </div>
  );
}
