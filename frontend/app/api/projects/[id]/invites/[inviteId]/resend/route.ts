import { NextRequest, NextResponse } from "next/server";
import { resendProjectInvitation } from "@/lib/projects-store";

export async function POST(
  req: NextRequest,
  props: { params: Promise<{ id: string; inviteId: string }> }
) {
  const { id, inviteId } = await props.params;
  try {
    const invite = resendProjectInvitation(id, inviteId);
    return NextResponse.json({
      success: true,
      message: "Invitation resent successfully",
      invitation: invite,
    });
  } catch (err: unknown) {
    return NextResponse.json({ error: (err as Error).message }, { status: 400 });
  }
}
