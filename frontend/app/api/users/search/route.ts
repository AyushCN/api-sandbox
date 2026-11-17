import { NextRequest, NextResponse } from "next/server";
import { getProjectById, getInvitations } from "@/lib/projects-store";

interface RegisteredUser {
  id: string;
  username: string;
  email: string;
  displayName: string;
  avatarUrl?: string;
}

export async function GET(req: NextRequest) {
  const url = new URL(req.url);
  const q = (url.searchParams.get("q") || "").trim().toLowerCase();
  const projectId = url.searchParams.get("projectId") || "";

  if (q.length < 2) {
    return NextResponse.json({ users: [] });
  }

  // 1. Identify current authenticated user to exclude them from search results
  let currentUsername = "";
  let currentEmail = "";
  let currentUserId = "";

  const backend = process.env.BACKEND_URL || "http://localhost:8080";
  const cookie = req.headers.get("cookie") || "";
  const authHeader = req.headers.get("authorization") || "";

  try {
    const meRes = await fetch(`${backend}/api/me`, {
      headers: {
        cookie,
        ...(authHeader ? { authorization: authHeader } : {}),
      },
      cache: "no-store",
    });
    if (meRes.ok) {
      const me = await meRes.json();
      currentUsername = (me.username || "").toLowerCase();
      currentEmail = (me.email || "").toLowerCase();
      currentUserId = me.id || "";
    }
  } catch {
    // Fallback if backend offline
  }

  // 2. Fetch registered DB users from backend
  const allUsersMap = new Map<string, RegisteredUser>();

  try {
    const remoteRes = await fetch(`${backend}/api/users/search?q=${encodeURIComponent(q)}`, {
      headers: {
        cookie,
        ...(authHeader ? { authorization: authHeader } : {}),
      },
      cache: "no-store",
    });
    if (remoteRes.ok) {
      const remoteData = await remoteRes.json();
      const remoteUsers: any[] = Array.isArray(remoteData)
        ? remoteData
        : remoteData?.users || [];
      for (const ru of remoteUsers) {
        if (ru && (ru.username || ru.email)) {
          const key = (ru.username || ru.email).toLowerCase();
          if (!allUsersMap.has(key)) {
            allUsersMap.set(key, {
              id: ru.id || `user-${key}`,
              username: ru.username || key,
              email: ru.email || `${key}@users.api-sandbox.local`,
              displayName: ru.displayName || ru.username || key,
              avatarUrl: ru.avatarUrl || "",
            });
          }
        }
      }
    }
  } catch {
    // Remote backend search failed or endpoint not ready
  }

  // 3. Filter by query string
  const matchedUsers: RegisteredUser[] = [];
  for (const u of allUsersMap.values()) {
    const uName = (u.username || "").toLowerCase();
    const uEmail = (u.email || "").toLowerCase();
    const uDisplay = (u.displayName || "").toLowerCase();

    // Exclude current user
    if (
      uName === currentUsername ||
      uEmail === currentEmail ||
      (currentUserId && u.id === currentUserId)
    ) {
      continue;
    }

    if (uName.includes(q) || uEmail.includes(q) || uDisplay.includes(q)) {
      matchedUsers.push(u);
    }
  }

  // 4. If projectId provided, check for existing membership and pending invitations
  let memberIdentifiers = new Set<string>();
  let pendingIdentifiers = new Set<string>();

  if (projectId) {
    const proj = getProjectById(projectId);
    if (proj && Array.isArray(proj.collaborators)) {
      for (const c of proj.collaborators) {
        if (c.user?.username) memberIdentifiers.add(c.user.username.toLowerCase());
        if (c.user?.email) memberIdentifiers.add(c.user.email.toLowerCase());
        if (c.userId) memberIdentifiers.add(c.userId.toLowerCase());
      }
    }

    const invites = getInvitations();
    const now = new Date();
    for (const inv of invites) {
      if (
        inv.projectId === projectId &&
        inv.status === "PENDING" &&
        new Date(inv.expiresAt) > now
      ) {
        const iden = inv.inviteeIdentifier.toLowerCase();
        pendingIdentifiers.add(iden);
        if (inv.inviteeId) pendingIdentifiers.add(inv.inviteeId.toLowerCase());
      }
    }
  }

  // 5. Sanitize results (exclude any sensitive fields)
  const results = matchedUsers.slice(0, 10).map((u) => {
    const uName = u.username.toLowerCase();
    const uEmail = u.email.toLowerCase();
    const uId = u.id.toLowerCase();

    const isMember =
      memberIdentifiers.has(uName) ||
      memberIdentifiers.has(uEmail) ||
      memberIdentifiers.has(uId);

    const isPending =
      pendingIdentifiers.has(uName) ||
      pendingIdentifiers.has(uEmail) ||
      pendingIdentifiers.has(uId);

    return {
      id: u.id,
      username: u.username,
      email: u.email,
      displayName: u.displayName || u.username,
      avatarUrl: u.avatarUrl || "",
      isMember,
      isPending,
    };
  });

  return NextResponse.json({ users: results });
}

