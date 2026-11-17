import { NextRequest, NextResponse } from "next/server";
import fs from "fs";
import path from "path";

function resolveWorkDir(id: string): string {
  const candidateDirs = [
    path.join("/tmp/api-sandbox-links/work", id),
    path.join("/tmp/api-sandbox/workspaces", id),
  ];

  for (const baseDir of ["/tmp/api-sandbox-links/work", "/tmp/api-sandbox/workspaces"]) {
    try {
      if (fs.existsSync(baseDir)) {
        const entries = fs.readdirSync(baseDir);
        for (const e of entries) {
          if (e.includes(id)) {
            candidateDirs.push(path.join(baseDir, e));
          }
        }
      }
    } catch {
      // Ignore
    }
  }

  return candidateDirs.find((d) => fs.existsSync(d)) || path.join("/tmp/api-sandbox-links/work", id);
}

export async function POST(
  req: NextRequest,
  props: { params: Promise<{ id: string }> }
) {
  const { id } = await props.params;

  let body: any = {};
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ error: "Invalid JSON body" }, { status: 400 });
  }

  const filePath = body.path;
  if (!filePath) {
    return NextResponse.json({ error: "File path is required" }, { status: 400 });
  }

  try {
    const workDir = resolveWorkDir(id);
    const cleanRelPath = path.normalize(filePath).replace(/^(\.\.[\/\\])+/, "");
    if (!cleanRelPath || cleanRelPath === "." || cleanRelPath === "/") {
      return NextResponse.json({ error: "Cannot delete workspace root" }, { status: 400 });
    }

    const fullPath = path.join(workDir, cleanRelPath);
    if (fs.existsSync(fullPath)) {
      fs.rmSync(fullPath, { recursive: true, force: true });
      return NextResponse.json({ message: "Deleted successfully", path: cleanRelPath });
    }
    return NextResponse.json({ error: "Path does not exist" }, { status: 404 });
  } catch (err: unknown) {
    return NextResponse.json({ error: (err as Error).message }, { status: 500 });
  }
}
