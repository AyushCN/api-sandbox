import { NextRequest, NextResponse } from "next/server";
import { resendProjectInvitation, revokeProjectInvitation } from "@/lib/projects-store";

export async function DELETE(
  req: NextRequest,
  props: { params: Promise<{ id: string; inviteId: string }> }
) {
  const { id, inviteId } = await props.params;
  try {
    const ok = revokeProjectInvitation(id, inviteId);
    if (!ok) {
      return NextResponse.json({ error: "Invitation not found or could not be revoked" }, { status: 404 });
    }
    return NextResponse.json({ success: true, message: "Invitation revoked successfully" });
  } catch (err: unknown) {
    return NextResponse.json({ error: (err as Error).message }, { status: 400 });
  }
}

export async function POST(
  req: NextRequest,
  props: { params: Promise<{ id: string; inviteId: string }> }
) {
  const { id, inviteId } = await props.params;
  try {
    const body = await req.json().catch(() => ({}));
    const action = body.action || new URL(req.url).searchParams.get("action") || "resend";

    if (action === "resend") {
      const invite = resendProjectInvitation(id, inviteId);
      return NextResponse.json({
        success: true,
        message: "Invitation resent successfully",
        invitation: invite,
      });
    } else if (action === "revoke") {
      const ok = revokeProjectInvitation(id, inviteId);
      if (!ok) {
        return NextResponse.json({ error: "Invitation not found or could not be revoked" }, { status: 404 });
      }
      return NextResponse.json({ success: true, message: "Invitation revoked successfully" });
    } else {
      return NextResponse.json({ error: "Invalid action. Supported: resend, revoke" }, { status: 400 });
    }
  } catch (err: unknown) {
    return NextResponse.json({ error: (err as Error).message }, { status: 400 });
  }
}
