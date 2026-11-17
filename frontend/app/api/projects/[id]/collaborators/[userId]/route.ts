import { NextRequest, NextResponse } from "next/server";
import { removeCollaborator, updateCollaboratorRole } from "@/lib/projects-store";

export async function DELETE(
  req: NextRequest,
  props: { params: Promise<{ id: string; userId: string }> }
) {
  const { id, userId } = await props.params;
  try {
    const ok = removeCollaborator(id, userId);
    if (!ok) {
      return NextResponse.json({ error: "Collaborator not found" }, { status: 404 });
    }
    return NextResponse.json({ success: true });
  } catch (err: unknown) {
    const msg = (err as Error).message;
    const status = msg.includes("owner cannot be removed") ? 400 : 500;
    return NextResponse.json({ error: msg }, { status });
  }
}

export async function PUT(
  req: NextRequest,
  props: { params: Promise<{ id: string; userId: string }> }
) {
  const { id, userId } = await props.params;
  try {
    const body = await req.json().catch(() => ({}));
    const { role } = body;
    if (!role || (role !== "EDITOR" && role !== "VIEWER")) {
      return NextResponse.json({ error: "Role must be EDITOR or VIEWER" }, { status: 400 });
    }
    const ok = updateCollaboratorRole(id, userId, role);
    if (!ok) {
      return NextResponse.json({ error: "Collaborator not found" }, { status: 404 });
    }
    return NextResponse.json({ success: true, role });
  } catch (err: unknown) {
    return NextResponse.json(
      { error: (err as Error).message },
      { status: 400 }
    );
  }
}
