import { NextRequest, NextResponse } from "next/server";
import { acceptProjectInvitation } from "@/lib/projects-store";

export async function POST(
  req: NextRequest,
  props: { params: Promise<{ id: string }> }
) {
  const { id } = await props.params;
  try {
    const body = await req.json().catch(() => ({}));
    const proj = acceptProjectInvitation(id, body.user);
    return NextResponse.json({
      success: true,
      message: "Invitation accepted successfully",
      project: proj,
      projectId: proj.id,
    });
  } catch (err: unknown) {
    const msg = (err as Error).message || "Failed to accept invitation";
    let status = 400;
    if (msg.includes("not found")) status = 404;
    return NextResponse.json({ error: msg }, { status });
  }
}
