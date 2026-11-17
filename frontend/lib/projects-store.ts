import fs from "fs";
import path from "path";

const PROJECTS_FILE = "/tmp/api-sandbox-links/projects.json";
const INVITATIONS_FILE = "/tmp/api-sandbox-links/invitations.json";

export interface ProjectRepository {
  id: string;
  name: string;
  gitUrl: string;
  defaultBranch: string;
}

export interface StoredProject {
  id: string;
  name: string;
  description: string;
  createdAt: string;
  isDefault?: boolean;
  createdBy?: string;
  collaborators: any[];
  sandboxIds: string[];
  repositories?: ProjectRepository[];
}

export interface StoredInvitation {
  id: string;
  projectId: string;
  projectName: string;
  inviterId: string;
  inviterName: string;
  inviteeId: string;
  inviteeIdentifier: string;
  role: "EDITOR" | "VIEWER";
  status: "PENDING" | "ACCEPTED" | "REJECTED" | "REVOKED" | "EXPIRED";
  createdAt: string;
  expiresAt: string;
  acceptedAt?: string;
}

const DEFAULT_PROJECT: StoredProject = {
  id: "default-project",
  name: "Default Project (Personal)",
  description: "Personal private development environment",
  createdAt: "2026-01-01T00:00:00Z",
  isDefault: true,
  createdBy: "owner@api-sandbox.local",
  collaborators: [
    {
      id: "collab-owner",
      userId: "user-owner",
      role: "OWNER",
      status: "ACTIVE",
      acceptedAt: "2026-01-01T00:00:00Z",
      user: {
        username: "Owner",
        email: "owner@api-sandbox.local",
      },
    },
  ],
  sandboxIds: [],
  repositories: [],
};

export function getProjects(): StoredProject[] {
  try {
    if (!fs.existsSync(PROJECTS_FILE)) {
      fs.mkdirSync(path.dirname(PROJECTS_FILE), { recursive: true });
      fs.writeFileSync(PROJECTS_FILE, JSON.stringify([DEFAULT_PROJECT], null, 2));
      return [DEFAULT_PROJECT];
    }
    const raw = fs.readFileSync(PROJECTS_FILE, "utf-8");
    const list = JSON.parse(raw);
    if (!Array.isArray(list) || list.length === 0) {
      return [DEFAULT_PROJECT];
    }
    // Ensure default-project exists in the list and isDefault is true
    let foundDefault = false;
    for (const p of list) {
      if (p.id === "default-project") {
        p.isDefault = true;
        foundDefault = true;
      }
    }
    if (!foundDefault) {
      list.unshift(DEFAULT_PROJECT);
    }

    return list;
  } catch {
    return [DEFAULT_PROJECT];
  }
}

export function saveProjects(projects: StoredProject[]): void {
  try {
    fs.mkdirSync(path.dirname(PROJECTS_FILE), { recursive: true });
    fs.writeFileSync(PROJECTS_FILE, JSON.stringify(projects, null, 2));
  } catch (err) {
    console.error("Failed to save projects to disk:", err);
  }
}

export function getProjectById(id: string): StoredProject | null {
  const list = getProjects();
  const proj = list.find((p) => p.id === id);
  if (!proj && id === "default-project") {
    return DEFAULT_PROJECT;
  }
  return proj || null;
}

export function addProject(
  name: string,
  description: string,
  creator?: { userId?: string; username?: string; email?: string }
): StoredProject {
  const list = getProjects();
  const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
  const id = `proj-${slug || Date.now()}`;

  const username = creator?.username || "Owner";
  const unameLower = username.toLowerCase();
  const defaultEmail = creator?.email || `${unameLower}@users.api-sandbox.local`;
  const email = creator?.email || defaultEmail;
  const userId = creator?.userId || `user-${unameLower}`;

  const ownerCollab = {
    id: `collab-owner-${Date.now()}`,
    userId,
    role: "OWNER",
    status: "ACTIVE",
    acceptedAt: new Date().toISOString(),
    user: {
      username,
      email,
    },
  };

  const newProj: StoredProject = {
    id,
    name: name.trim(),
    description: description.trim(),
    createdAt: new Date().toISOString(),
    isDefault: false,
    createdBy: email,
    collaborators: [ownerCollab],
    sandboxIds: [],
    repositories: [],
  };
  list.push(newProj);
  saveProjects(list);
  return newProj;
}

export function associateSandboxWithProject(projectId: string, sandboxId: string): void {
  const list = getProjects();
  const proj = list.find((p) => p.id === projectId);
  if (proj) {
    if (!proj.sandboxIds) proj.sandboxIds = [];
    if (!proj.sandboxIds.includes(sandboxId)) {
      proj.sandboxIds.push(sandboxId);
      saveProjects(list);
    }
  }
}

// ---- Invitations Storage ----

export function getInvitations(): StoredInvitation[] {
  try {
    if (!fs.existsSync(INVITATIONS_FILE)) {
      return [];
    }
    const raw = fs.readFileSync(INVITATIONS_FILE, "utf-8");
    return JSON.parse(raw);
  } catch {
    return [];
  }
}

export function saveInvitations(invites: StoredInvitation[]): void {
  try {
    fs.mkdirSync(path.dirname(INVITATIONS_FILE), { recursive: true });
    fs.writeFileSync(INVITATIONS_FILE, JSON.stringify(invites, null, 2));
  } catch (err) {
    console.error("Failed to save invitations to disk:", err);
  }
}

export function createProjectInvitation(
  projectId: string,
  identifier: string,
  role: string = "EDITOR",
  inviter?: { userId?: string; username?: string; email?: string }
): StoredInvitation {
  const proj = getProjectById(projectId);
  if (!proj) {
    throw new Error("Project not found");
  }

  // Enforce Default Project privacy: CANNOT be shared
  if (proj.isDefault || proj.id === "default-project") {
    throw new Error("Default Project (Personal Space) cannot be shared or receive invitations");
  }

  const trimmed = identifier.trim().toLowerCase();
  const inviterEmail = (inviter?.email || "").toLowerCase();
  const inviterUsername = (inviter?.username || "").toLowerCase();

  // Edge case 1: Self invite
  if (trimmed === inviterEmail || trimmed === inviterUsername) {
    throw new Error("Cannot invite yourself to your own project");
  }

  // Edge case 2: Already a member
  if (Array.isArray(proj.collaborators)) {
    const isMember = proj.collaborators.some(
      (c) => c.status === "ACTIVE" && (
        c.user?.email?.toLowerCase() === trimmed ||
        c.user?.username?.toLowerCase() === trimmed
      )
    );
    if (isMember) {
      throw new Error("User is already a member of this project");
    }
  }

  // Edge case 3: Duplicate pending invitation
  const invites = getInvitations();
  const now = new Date();
  const duplicate = invites.find(
    (i) => i.projectId === projectId &&
      i.status === "PENDING" &&
      new Date(i.expiresAt) > now &&
      i.inviteeIdentifier.toLowerCase() === trimmed
  );
  if (duplicate) {
    throw new Error("An invitation is already pending for this user");
  }

  // Ownership guard
  if (role === "OWNER") {
    throw new Error("Project ownership cannot be assigned through ordinary invitations");
  }

  const validRole = role === "VIEWER" ? "VIEWER" : "EDITOR";
  const expiresAt = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString();

  const newInvite: StoredInvitation = {
    id: `invite-${Date.now()}`,
    projectId: proj.id,
    projectName: proj.name,
    inviterId: inviter?.userId || "user-owner",
    inviterName: inviter?.username || "Project Owner",
    inviteeId: `user-${trimmed.replace(/[^a-z0-9]/g, "")}`,
    inviteeIdentifier: trimmed,
    role: validRole,
    status: "PENDING",
    createdAt: new Date().toISOString(),
    expiresAt,
  };

  invites.push(newInvite);
  saveInvitations(invites);
  return newInvite;
}

export function getUserInvitations(userIdentifier?: string): StoredInvitation[] {
  const invites = getInvitations();
  const now = new Date();
  const idStr = (userIdentifier || "").toLowerCase();

  return invites.filter((i) => {
    if (i.status !== "PENDING") return false;
    if (new Date(i.expiresAt) <= now) {
      i.status = "EXPIRED";
      return false;
    }
    if (!idStr) return true; // return all pending if no user filter specified
    return i.inviteeIdentifier.toLowerCase() === idStr ||
      idStr.includes(i.inviteeIdentifier.toLowerCase()) ||
      i.inviteeIdentifier.toLowerCase().includes(idStr);
  });
}

export function acceptProjectInvitation(
  invitationId: string,
  user?: { userId?: string; username?: string; email?: string }
): StoredProject {
  const invites = getInvitations();
  const invite = invites.find((i) => i.id === invitationId);
  if (!invite) {
    throw new Error("Invitation not found");
  }

  if (invite.status === "REVOKED") {
    throw new Error("Invitation was revoked by the project owner");
  }
  if (invite.status === "REJECTED") {
    throw new Error("Invitation was previously rejected");
  }
  if (invite.status === "EXPIRED" || new Date(invite.expiresAt) <= new Date()) {
    invite.status = "EXPIRED";
    saveInvitations(invites);
    throw new Error("Invitation has expired");
  }

  const proj = getProjectById(invite.projectId);
  if (!proj) {
    throw new Error("Project not found");
  }

  // Update invitation status
  invite.status = "ACCEPTED";
  invite.acceptedAt = new Date().toISOString();
  saveInvitations(invites);

  // Add user to project collaborators with assigned role
  if (!proj.collaborators) proj.collaborators = [];
  const username = user?.username || invite.inviteeIdentifier.split("@")[0] || "Collaborator";
  const unameLower = username.toLowerCase();
  const defaultInviteEmail = `${unameLower}@users.api-sandbox.local`;
  const email = user?.email || (invite.inviteeIdentifier.includes("@") ? invite.inviteeIdentifier : defaultInviteEmail);
  const userId = user?.userId || `user-${unameLower}`;

  const existingCollab = proj.collaborators.find((c) => c.userId === userId || c.user?.email === email);
  if (existingCollab) {
    existingCollab.role = invite.role;
    existingCollab.status = "ACTIVE";
    existingCollab.acceptedAt = new Date().toISOString();
  } else {
    proj.collaborators.push({
      id: `collab-${Date.now()}`,
      userId,
      role: invite.role,
      status: "ACTIVE",
      acceptedAt: new Date().toISOString(),
      user: {
        username,
        email,
      },
    });
  }

  const list = getProjects();
  const idx = list.findIndex((p) => p.id === proj.id);
  if (idx !== -1) {
    list[idx] = proj;
    saveProjects(list);
  }

  return proj;
}

export function rejectProjectInvitation(invitationId: string): boolean {
  const invites = getInvitations();
  const invite = invites.find((i) => i.id === invitationId);
  if (!invite) return false;

  invite.status = "REJECTED";
  saveInvitations(invites);
  return true;
}

export function revokeProjectInvitation(projectId: string, invitationId: string): boolean {
  const invites = getInvitations();
  const invite = invites.find((i) => i.id === invitationId && i.projectId === projectId);
  if (!invite) return false;

  if (invite.status === "ACCEPTED") {
    throw new Error("Cannot revoke an already accepted invitation. Remove member instead.");
  }

  invite.status = "REVOKED";
  saveInvitations(invites);
  return true;
}

export function resendProjectInvitation(projectId: string, invitationId: string): StoredInvitation {
  const invites = getInvitations();
  const invite = invites.find((i) => i.id === invitationId && i.projectId === projectId);
  if (!invite) {
    throw new Error("Invitation not found");
  }

  if (invite.status !== "PENDING") {
    throw new Error("Only pending invitations can be resent");
  }

  // Extend expiration by 7 days from now
  invite.expiresAt = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString();
  saveInvitations(invites);
  return invite;
}

export function inviteCollaborator(projectId: string, identifier: string, role = "EDITOR"): any {
  // Backwards compatible wrapper calling createProjectInvitation
  return createProjectInvitation(projectId, identifier, role);
}

export function removeCollaborator(projectId: string, userId: string): boolean {
  const list = getProjects();
  const proj = list.find((p) => p.id === projectId);
  if (!proj || !proj.collaborators) return false;

  // Cannot remove owner from Default Project or remove the owner of any project
  const member = proj.collaborators.find((c: any) => c.userId === userId || c.id === userId);
  if (member && member.role === "OWNER") {
    throw new Error("Project owner cannot be removed");
  }

  proj.collaborators = proj.collaborators.filter((c: any) => c.userId !== userId && c.id !== userId);
  saveProjects(list);
  return true;
}

export function updateCollaboratorRole(projectId: string, userId: string, newRole: string): boolean {
  const list = getProjects();
  const proj = list.find((p) => p.id === projectId);
  if (!proj || !proj.collaborators) return false;

  if (proj.isDefault || proj.id === "default-project") {
    throw new Error("Default Project (Personal Space) roles cannot be modified");
  }

  const member = proj.collaborators.find((c: any) => c.userId === userId || c.id === userId);
  if (!member) return false;

  if (member.role === "OWNER") {
    throw new Error("Project owner role cannot be changed. Transfer ownership instead.");
  }

  member.role = newRole === "VIEWER" ? "VIEWER" : "EDITOR";
  saveProjects(list);
  return true;
}

export function addProjectRepository(
  projectId: string,
  repo: { name: string; gitUrl: string; defaultBranch?: string }
): ProjectRepository | null {
  const list = getProjects();
  let proj = list.find((p) => p.id === projectId);
  if (!proj) return null;
  if (!proj.repositories) proj.repositories = [];

  const newRepo: ProjectRepository = {
    id: `repo-${Date.now()}`,
    name: repo.name.trim(),
    gitUrl: repo.gitUrl.trim(),
    defaultBranch: repo.defaultBranch || "main",
  };
  proj.repositories.push(newRepo);
  saveProjects(list);
  return newRepo;
}

export function filterSandboxesForProject(projectId: string, allSandboxes: any[]): any[] {
  if (!Array.isArray(allSandboxes)) return [];
  const list = getProjects();

  if (!projectId || projectId === "default-project") {
    // Collect all sandboxIds assigned to other custom projects
    const otherProjectSandboxIds = new Set<string>();
    for (const p of list) {
      if (p.id !== "default-project" && Array.isArray(p.sandboxIds)) {
        for (const sid of p.sandboxIds) {
          otherProjectSandboxIds.add(sid);
        }
      }
    }
    const defaultProj = list.find((p) => p.id === "default-project");
    const explicitDefaultIds = new Set(defaultProj?.sandboxIds || []);
    return allSandboxes.filter((sb) => explicitDefaultIds.has(sb.id) || !otherProjectSandboxIds.has(sb.id));
  }

  const proj = list.find((p) => p.id === projectId);
  if (!proj || !Array.isArray(proj.sandboxIds) || proj.sandboxIds.length === 0) {
    return [];
  }
  const set = new Set(proj.sandboxIds);
  return allSandboxes.filter((sb) => set.has(sb.id));
}

