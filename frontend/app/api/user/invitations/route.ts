import { NextRequest, NextResponse } from "next/server";
import { getUserInvitations } from "@/lib/projects-store";

export async function GET(req: NextRequest) {
  const url = new URL(req.url);
  const user = url.searchParams.get("user") || "";
  const invites = getUserInvitations(user);
  return NextResponse.json(invites);
}
