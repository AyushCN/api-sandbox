"use client";

import React, { useState, useEffect } from "react";
import { Loader2, Search, X, Check, ShieldCheck, Eye, AlertCircle, UserCheck, Clock } from "lucide-react";

export interface UserSearchResult {
  id: string;
  username: string;
  email: string;
  displayName?: string;
  avatarUrl?: string;
  isMember?: boolean;
  isPending?: boolean;
}

interface InviteCollaboratorModalProps {
  isOpen: boolean;
  onClose: () => void;
  onInvite: (identifier: string, role: string) => Promise<void>;
  isInviting: boolean;
  projectId?: string;
  projectName?: string;
}

export function InviteCollaboratorModal({
  isOpen,
  onClose,
  onInvite,
  isInviting,
  projectId,
  projectName = "Current Project",
}: InviteCollaboratorModalProps) {
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedUser, setSelectedUser] = useState<UserSearchResult | null>(null);
  const [inviteRole, setInviteRole] = useState<"EDITOR" | "VIEWER">("EDITOR");
  const [searchResults, setSearchResults] = useState<UserSearchResult[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [showDropdown, setShowDropdown] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // Debounced user search
  useEffect(() => {
    const trimmed = searchQuery.trim();
    if (!trimmed || trimmed.length < 2) {
      setSearchResults([]);
      setIsSearching(false);
      return;
    }

    const timer = setTimeout(async () => {
      setIsSearching(true);
      setErrorMessage(null);
      try {
        const token = typeof window !== "undefined" ? localStorage.getItem("token") : null;
        const projectParam = projectId ? `&projectId=${encodeURIComponent(projectId)}` : "";
        const res = await fetch(
          `/api/users/search?q=${encodeURIComponent(trimmed)}${projectParam}`,
          {
            headers: token ? { Authorization: `Bearer ${token}` } : {},
          }
        );
        if (res.ok) {
          const data = await res.json();
          setSearchResults(data.users || []);
          setShowDropdown(true);
        }
      } catch {
        // Fallback or ignore network error
      } finally {
        setIsSearching(false);
      }
    }, 300);

    return () => clearTimeout(timer);
  }, [searchQuery, projectId]);

  if (!isOpen) return null;

  const handleSelectUser = (user: UserSearchResult) => {
    if (user.isMember || user.isPending) return;
    setSelectedUser(user);
    setSearchQuery(user.username || user.email);
    setShowDropdown(false);
    setErrorMessage(null);
  };

  const handleClearSelection = () => {
    setSelectedUser(null);
    setSearchQuery("");
    setSearchResults([]);
    setShowDropdown(false);
    setErrorMessage(null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMessage(null);
    const identifier = selectedUser ? selectedUser.username || selectedUser.email : searchQuery.trim();
    if (!identifier) {
      setErrorMessage("Please select or enter a user to invite.");
      return;
    }

    try {
      await onInvite(identifier, inviteRole);
      // Reset state on success
      setSelectedUser(null);
      setSearchQuery("");
      setSearchResults([]);
      onClose();
    } catch (err: unknown) {
      setErrorMessage((err as Error).message || "Failed to send invitation.");
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm animate-in fade-in duration-200"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        className="w-full max-w-xl bg-surface-container-lowest rounded-2xl border border-outline-variant shadow-2xl overflow-hidden flex flex-col"
        style={{ maxHeight: "90vh" }}
      >
        {/* Modal Header */}
        <div className="flex items-start justify-between px-6 pt-6 pb-4 border-b border-outline-variant/60 bg-surface-container/20">
          <div>
            <h2 className="text-xl font-bold text-on-surface tracking-tight">Invite people</h2>
            <p className="text-xs text-on-surface-variant/80 mt-1">
              Share your project with other Coollab users.
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg text-on-surface-variant hover:text-on-surface hover:bg-surface-container transition-colors cursor-pointer"
            aria-label="Close"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Modal Form */}
        <form onSubmit={handleSubmit} className="p-6 space-y-5 overflow-y-auto flex-1">
          {errorMessage && (
            <div className="p-3.5 rounded-xl bg-error/10 border border-error/30 flex items-center gap-2.5 text-xs text-error">
              <AlertCircle className="w-4 h-4 shrink-0" />
              <span>{errorMessage}</span>
            </div>
          )}

          {/* User Search Section */}
          <div className="space-y-1.5">
            <label className="text-xs font-bold uppercase tracking-wider text-on-surface-variant block">
              Search users
            </label>

            {selectedUser ? (
              /* Selected User Chip */
              <div className="flex items-center justify-between p-3 rounded-xl bg-primary-fixed/10 border border-primary-fixed/30 shadow-sm">
                <div className="flex items-center gap-3">
                  <div className="w-9 h-9 rounded-full bg-primary-fixed/20 border border-primary-fixed/40 flex items-center justify-center text-primary-fixed font-bold text-sm shrink-0">
                    {(selectedUser.username || selectedUser.email || "?")[0]?.toUpperCase()}
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-semibold text-sm text-on-surface">
                        {selectedUser.username}
                      </span>
                      {selectedUser.displayName && selectedUser.displayName !== selectedUser.username && (
                        <span className="text-xs text-on-surface-variant/70">
                          ({selectedUser.displayName})
                        </span>
                      )}
                    </div>
                    <p className="text-xs text-on-surface-variant/80">{selectedUser.email}</p>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={handleClearSelection}
                  className="p-1.5 rounded-lg text-on-surface-variant hover:text-error hover:bg-error/10 transition-colors cursor-pointer"
                  title="Remove selection"
                >
                  <X className="w-4 h-4" />
                </button>
              </div>
            ) : (
              /* Search Input */
              <div className="relative">
                <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-4 h-4 text-on-surface-variant/50" />
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(e) => {
                    setSearchQuery(e.target.value);
                    setShowDropdown(true);
                  }}
                  onFocus={() => setShowDropdown(true)}
                  placeholder="Search by username or email..."
                  className="w-full bg-surface-container pl-10 pr-10 py-3 rounded-xl border border-outline-variant text-on-surface placeholder:text-on-surface-variant/40 focus:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition-all text-sm font-medium"
                />
                {isSearching ? (
                  <div className="absolute right-3.5 top-1/2 -translate-y-1/2">
                    <Loader2 className="w-4 h-4 animate-spin text-emerald-500" />
                  </div>
                ) : searchQuery ? (
                  <button
                    type="button"
                    onClick={() => {
                      setSearchQuery("");
                      setSearchResults([]);
                    }}
                    className="absolute right-3.5 top-1/2 -translate-y-1/2 text-on-surface-variant/50 hover:text-on-surface"
                  >
                    <X className="w-4 h-4" />
                  </button>
                ) : null}

                {/* Dropdown Results */}
                {showDropdown && searchResults.length > 0 && (
                  <div className="absolute z-20 w-full mt-1.5 bg-surface-container-lowest border border-outline-variant rounded-xl shadow-2xl max-h-56 overflow-y-auto divide-y divide-outline-variant/40">
                    {searchResults.map((user) => {
                      const isDisabled = user.isMember || user.isPending;
                      return (
                        <div
                          key={user.id}
                          onClick={() => {
                            if (!isDisabled) handleSelectUser(user);
                          }}
                          className={`px-4 py-3 flex items-center justify-between transition-colors ${
                            isDisabled
                              ? "opacity-60 bg-surface-container/30 cursor-not-allowed"
                              : "hover:bg-emerald-500/10 cursor-pointer"
                          }`}
                        >
                          <div className="flex items-center gap-3 min-w-0">
                            <div className="w-8 h-8 rounded-full bg-primary-fixed/20 border border-primary-fixed/30 flex items-center justify-center font-bold text-xs text-primary-fixed shrink-0">
                              {(user.username || user.email || "?")[0]?.toUpperCase()}
                            </div>
                            <div className="min-w-0">
                              <div className="flex items-center gap-2">
                                <span className="font-semibold text-xs text-on-surface truncate">
                                  {user.username}
                                </span>
                                {user.displayName && user.displayName !== user.username && (
                                  <span className="text-[11px] text-on-surface-variant/60 truncate">
                                    {user.displayName}
                                  </span>
                                )}
                              </div>
                              <p className="text-[11px] text-on-surface-variant/70 truncate">
                                {user.email}
                              </p>
                            </div>
                          </div>

                          <div className="shrink-0 ml-3">
                            {user.isMember ? (
                              <span className="inline-flex items-center gap-1 text-[10px] font-bold px-2 py-0.5 rounded-full bg-surface-container-high text-on-surface-variant border border-outline-variant">
                                <UserCheck className="w-3 h-3" />
                                Member
                              </span>
                            ) : user.isPending ? (
                              <span className="inline-flex items-center gap-1 text-[10px] font-bold px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-500 border border-amber-500/30">
                                <Clock className="w-3 h-3" />
                                Pending
                              </span>
                            ) : (
                              <span className="text-xs font-semibold text-emerald-500 opacity-0 group-hover:opacity-100">
                                Select
                              </span>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Role Selection Section matching Mockup */}
          <div className="space-y-2.5">
            <label className="text-xs font-bold uppercase tracking-wider text-on-surface-variant block">
              Select permission
            </label>

            {/* Editor Role Card */}
            <div
              onClick={() => setInviteRole("EDITOR")}
              className={`p-4 rounded-xl border-2 transition-all cursor-pointer relative ${
                inviteRole === "EDITOR"
                  ? "border-emerald-500 bg-emerald-500/[0.04] shadow-sm"
                  : "border-outline-variant/70 hover:border-outline-variant bg-surface-container/20"
              }`}
            >
              <div className="flex items-center justify-between">
                <span className="font-bold text-sm text-on-surface">Editor</span>
                {inviteRole === "EDITOR" && (
                  <span className="text-xs font-semibold px-2.5 py-0.5 rounded-full bg-emerald-500/15 text-emerald-500 border border-emerald-500/30">
                    Selected
                  </span>
                )}
              </div>
              <p className="text-xs text-on-surface-variant mt-1.5 leading-relaxed">
                Can edit files, execute commands and interact with permitted environments.
              </p>
              <div className="text-xs font-semibold text-emerald-500 mt-2">
                Read + Write + Execute
              </div>
            </div>

            {/* Viewer Role Card */}
            <div
              onClick={() => setInviteRole("VIEWER")}
              className={`p-4 rounded-xl border-2 transition-all cursor-pointer relative ${
                inviteRole === "VIEWER"
                  ? "border-emerald-500 bg-emerald-500/[0.04] shadow-sm"
                  : "border-outline-variant/70 hover:border-outline-variant bg-surface-container/20"
              }`}
            >
              <div className="flex items-center justify-between">
                <span className="font-bold text-sm text-on-surface">Viewer</span>
                {inviteRole === "VIEWER" && (
                  <span className="text-xs font-semibold px-2.5 py-0.5 rounded-full bg-emerald-500/15 text-emerald-500 border border-emerald-500/30">
                    Selected
                  </span>
                )}
              </div>
              <p className="text-xs text-on-surface-variant mt-1.5 leading-relaxed">
                Can view files, logs, project status and permitted previews.
              </p>
              <div className="text-xs font-medium text-on-surface-variant/70 mt-2">
                Read only
              </div>
            </div>
          </div>

          {/* Invitation Preview Summary */}
          {selectedUser && (
            <div className="p-3.5 rounded-xl bg-surface-container/50 border border-outline-variant/60 space-y-1.5 text-xs">
              <div className="flex items-center justify-between text-on-surface-variant">
                <span>Target Project:</span>
                <span className="font-semibold text-on-surface">{projectName}</span>
              </div>
              <div className="flex items-center justify-between text-on-surface-variant">
                <span>Inviting:</span>
                <span className="font-semibold text-on-surface">
                  {selectedUser.username} ({selectedUser.email})
                </span>
              </div>
              <div className="flex items-center justify-between text-on-surface-variant">
                <span>Assigned Role:</span>
                <span className="font-bold text-emerald-500">
                  {inviteRole === "EDITOR" ? "Editor (Read + Write + Execute)" : "Viewer (Read only)"}
                </span>
              </div>
            </div>
          )}

          {/* Action Buttons */}
          <div className="pt-3 border-t border-outline-variant/60 flex items-center justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              disabled={isInviting}
              className="px-4 py-2 text-xs font-semibold text-on-surface-variant hover:text-on-surface rounded-lg hover:bg-surface-container transition-colors cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isInviting || (!selectedUser && !searchQuery.trim())}
              className="px-5 py-2.5 rounded-xl font-bold text-xs bg-emerald-500 hover:bg-emerald-600 text-white shadow-md shadow-emerald-500/20 disabled:opacity-50 disabled:pointer-events-none transition-all flex items-center gap-2 cursor-pointer"
            >
              {isInviting && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
              Send Invitation
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
