import { NextRequest, NextResponse } from "next/server";
import { revokeProjectInvitation } from "@/lib/projects-store";

export async function POST(
  req: NextRequest,
  props: { params: Promise<{ id: string; inviteId: string }> }
) {
  const { id, inviteId } = await props.params;
  try {
    const ok = revokeProjectInvitation(id, inviteId);
    if (!ok) {
      return NextResponse.json({ error: "Invitation not found or could not be revoked" }, { status: 404 });
    }
    return NextResponse.json({
      success: true,
      message: "Invitation revoked successfully",
    });
  } catch (err: unknown) {
    return NextResponse.json({ error: (err as Error).message }, { status: 400 });
  }
}
