import { NextRequest, NextResponse } from "next/server";
import { getProjectById, getInvitations } from "@/lib/projects-store";

export async function GET(
  req: NextRequest,
  props: { params: Promise<{ id: string }> }
) {
  const { id } = await props.params;
  const project = getProjectById(id);

  // Authenticate and fetch currently requesting user
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
      if (me?.username) {
        currentUsername = me.username;
        currentUserId = me.id || `user-${me.username.toLowerCase()}`;
        currentEmail = me.email || `${me.username.toLowerCase()}@users.api-sandbox.local`;
      }
    }
  } catch {
    // Session fetch error
  }

  // If Default Project (Personal Space), the owner is dynamically the requesting user
  if (id === "default-project" || project?.isDefault) {
    const ownerName = currentUsername || "Owner";
    const ownerEmail =
      currentEmail ||
      (currentUsername
        ? `${currentUsername.toLowerCase()}@users.api-sandbox.local`
        : "owner@api-sandbox.local");
    const ownerId = currentUserId || `user-${ownerName.toLowerCase()}`;

    return NextResponse.json([
      {
        id: "owner",
        userId: ownerId,
        role: "OWNER",
        status: "ACTIVE",
        acceptedAt: project?.createdAt || "2026-01-01T00:00:00Z",
        user: {
          username: ownerName,
          email: ownerEmail,
        },
      },
    ]);
  }

  const fallbackOwnerName =
    currentUsername ||
    (project?.createdBy ? project.createdBy.split("@")[0] : "Owner");
  const fallbackOwnerEmail =
    currentEmail ||
    project?.createdBy ||
    (currentUsername
      ? `${currentUsername.toLowerCase()}@users.api-sandbox.local`
      : "owner@api-sandbox.local");
  const fallbackOwnerId =
    currentUserId || `user-${fallbackOwnerName.toLowerCase()}`;

  const collabs =
    project?.collaborators && project.collaborators.length > 0
      ? [...project.collaborators]
      : [
          {
            id: "m-owner",
            userId: fallbackOwnerId,
            role: "OWNER",
            status: "ACTIVE",
            acceptedAt: project?.createdAt || "2026-01-01T00:00:00Z",
            user: {
              username: fallbackOwnerName,
              email: fallbackOwnerEmail,
            },
          },
        ];

  // Include pending invitations for this project
  try {
    const allInvites = getInvitations();
    const now = new Date();
    const pendingForProj = allInvites.filter(
      (inv) =>
        inv.projectId === id &&
        inv.status === "PENDING" &&
        new Date(inv.expiresAt) > now
    );

    for (const inv of pendingForProj) {
      const username = inv.inviteeIdentifier.split("@")[0] || "Invited User";
      const unameLower = username.toLowerCase();
      const email = inv.inviteeIdentifier.includes("@")
        ? inv.inviteeIdentifier
        : `${unameLower}@users.api-sandbox.local`;

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
