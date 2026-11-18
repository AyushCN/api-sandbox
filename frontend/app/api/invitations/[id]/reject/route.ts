import { NextRequest, NextResponse } from "next/server";
import { rejectProjectInvitation } from "@/lib/projects-store";

export async function POST(
  req: NextRequest,
  props: { params: Promise<{ id: string }> }
) {
  const { id } = await props.params;
  try {
    const success = rejectProjectInvitation(id);
    if (!success) {
      return NextResponse.json({ error: "Invitation not found" }, { status: 404 });
    }
    return NextResponse.json({
      success: true,
      message: "Invitation rejected successfully",
    });
  } catch (err: unknown) {
    return NextResponse.json(
      { error: (err as Error).message || "Failed to reject invitation" },
      { status: 400 }
    );
  }
}
