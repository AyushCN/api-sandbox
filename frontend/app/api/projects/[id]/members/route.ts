import { NextRequest, NextResponse } from "next/server";
import { getProjectById, getInvitations } from "@/lib/projects-store";

export async function GET(
  req: NextRequest,
  props: { params: Promise<{ id: string }> }
) {
  const { id } = await props.params;
  const project = getProjectById(id);

  // If Default Project (Personal Space), the owner is ALWAYS the requesting user, never Admin
  if (id === "default-project" || project?.isDefault) {
    let currentUsername = "AyushCN";
    let currentEmail = "ayushcn@gmail.com";
    let currentUserId = "user-ayushcn";

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
        if (me?.username) {
          currentUsername = me.username;
          currentUserId = me.id || `user-${me.username.toLowerCase()}`;
          const unameLower = me.username.toLowerCase();
          currentEmail = me.email || (unameLower === "ayushcn" || unameLower === "ayush"
            ? "ayushcn@gmail.com"
            : `${unameLower}@users.api-sandbox.local`);
        }
      }
    } catch {
      // Fallback to AyushCN
    }

    return NextResponse.json([
      {
        id: "owner",
        userId: currentUserId,
        role: "OWNER",
        status: "ACTIVE",
        acceptedAt: "2026-01-01T00:00:00Z",
        user: {
          username: currentUsername,
          email: currentEmail,
        },
      },
    ]);
  }

  const collabs = (project?.collaborators && project.collaborators.length > 0)
    ? [...project.collaborators]
    : [
        {
          id: "m-owner",
          userId: "user-ayushcn",
          role: "OWNER",
          status: "ACTIVE",
          acceptedAt: "2026-01-01T00:00:00Z",
          user: {
            username: "AyushCN",
            email: "ayushcn@gmail.com",
          },
        },
      ];

  // Include pending invitations for this project
  try {
    const allInvites = getInvitations();
    const now = new Date();
    const pendingForProj = allInvites.filter(
      (inv) => inv.projectId === id && inv.status === "PENDING" && new Date(inv.expiresAt) > now
    );

    for (const inv of pendingForProj) {
      const username = inv.inviteeIdentifier.split("@")[0] || "Invited User";
      const unameLower = username.toLowerCase();
      const defaultEmail = (unameLower === "ayushcn" || unameLower === "ayush")
        ? "ayushcn@gmail.com"
        : `${unameLower}@users.api-sandbox.local`;
      const email = inv.inviteeIdentifier.includes("@") ? inv.inviteeIdentifier : defaultEmail;

      collabs.push({
        id: inv.id,
        inviteId: inv.id,
        userId: inv.inviteeId || `user-${unameLower}`,
        role: inv.role,
        status: "PENDING",
        createdAt: inv.createdAt,
        expiresAt: inv.expiresAt,
        inviterName: inv.inviterName,
        user: {
          username,
          email,
        },
      });
    }
  } catch {
    // Non-fatal if invitations file cannot be read
  }

  return NextResponse.json(collabs);
}
