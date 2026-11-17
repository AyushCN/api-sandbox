import { NextRequest, NextResponse } from "next/server";
import { createProjectInvitation, getProjectById } from "@/lib/projects-store";

export async function POST(
  req: NextRequest,
  props: { params: Promise<{ id: string }> }
) {
  const { id } = await props.params;
  try {
    const proj = getProjectById(id);
    if (!proj) {
      return NextResponse.json({ error: "Project not found" }, { status: 404 });
    }

    if (proj.isDefault || proj.id === "default-project") {
      return NextResponse.json(
        { error: "Default Project (Personal Space) cannot be shared or receive invitations" },
        { status: 403 }
      );
    }

    const body = await req.json().catch(() => ({}));
    const { identifier, role, inviter } = body;
    if (!identifier) {
      return NextResponse.json(
        { error: "Username or email is required" },
        { status: 400 }
      );
    }

    const invitation = createProjectInvitation(id, identifier, role || "EDITOR", inviter);
    return NextResponse.json({
      success: true,
      message: "Collaboration invitation sent successfully",
      invitation,
      collaborator: invitation,
    });
  } catch (err: unknown) {
    const msg = (err as Error).message || "Failed to invite collaborator";
    let status = 500;
    if (msg.includes("Default Project")) status = 403;
    else if (msg.includes("already")) status = 409;
    else if (msg.includes("Cannot invite yourself") || msg.includes("Invalid")) status = 400;
    else if (msg.includes("not found")) status = 404;

    return NextResponse.json({ error: msg }, { status });
  }
}
