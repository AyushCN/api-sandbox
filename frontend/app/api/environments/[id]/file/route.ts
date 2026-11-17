import { NextRequest, NextResponse } from "next/server";
import fs from "fs";
import path from "path";

// Find on-disk work directory for sandbox
function resolveWorkDir(id: string, subdomain?: string): string {
  const candidateDirs = [
    subdomain ? path.join("/tmp/api-sandbox-links/work", subdomain) : "",
    path.join("/tmp/api-sandbox-links/work", id),
    path.join("/tmp/api-sandbox/workspaces", id),
    subdomain ? path.join("/tmp/api-sandbox/workspaces", subdomain) : "",
  ].filter(Boolean);

  for (const baseDir of ["/tmp/api-sandbox-links/work", "/tmp/api-sandbox/workspaces"]) {
    try {
      if (fs.existsSync(baseDir)) {
        const entries = fs.readdirSync(baseDir);
        for (const e of entries) {
          if (e.includes(id) || (subdomain && e.includes(subdomain))) {
            candidateDirs.push(path.join(baseDir, e));
          }
        }
      }
    } catch {
      // Ignore
    }
  }

  const existing = candidateDirs.find((d) => fs.existsSync(d));
  if (existing) return existing;

  const fallback = subdomain
    ? path.join("/tmp/api-sandbox-links/work", subdomain)
    : path.join("/tmp/api-sandbox-links/work", id);
  fs.mkdirSync(fallback, { recursive: true });
  return fallback;
}

export async function GET(
  req: NextRequest,
  props: { params: Promise<{ id: string }> }
) {
  const { id } = await props.params;
  const filePath = req.nextUrl.searchParams.get("path") || "";
  if (!filePath) {
    return NextResponse.json({ error: "Path parameter is required" }, { status: 400 });
  }

  const backend = process.env.BACKEND_URL || "http://localhost:8080";
  const cookie = req.headers.get("cookie") || "";
  const authHeader = req.headers.get("authorization") || "";

  const headers: Record<string, string> = {};
  if (cookie) headers["cookie"] = cookie;
  if (authHeader) headers["authorization"] = authHeader;

  // 1. Try backend endpoint /api/sandboxes/:id/file?path=...
  try {
    const res = await fetch(`${backend}/api/sandboxes/${id}/file?path=${encodeURIComponent(filePath)}`, {
      headers,
      cache: "no-store",
    });
    if (res.ok) {
      const data = await res.json();
      return NextResponse.json(data);
    }
  } catch {
    // Ignore backend failure, fallback to disk
  }

  // 2. Fallback to reading file directly from disk
  try {
    const workDir = resolveWorkDir(id);
    const cleanRelPath = path.normalize(filePath).replace(/^(\.\.[\/\\])+/, "");
    const fullPath = path.join(workDir, cleanRelPath);

    if (fs.existsSync(fullPath) && !fs.statSync(fullPath).isDirectory()) {
      const content = fs.readFileSync(fullPath, "utf-8");
      return NextResponse.json({
        path: cleanRelPath,
        content,
        binary: false,
        size: Buffer.byteLength(content),
      });
    }
  } catch (err: unknown) {
    return NextResponse.json({ error: (err as Error).message }, { status: 500 });
  }

  return NextResponse.json({ error: "File not found" }, { status: 404 });
}

export async function PUT(
  req: NextRequest,
  props: { params: Promise<{ id: string }> }
) {
  const { id } = await props.params;
  const backend = process.env.BACKEND_URL || "http://localhost:8080";
  const cookie = req.headers.get("cookie") || "";
  const authHeader = req.headers.get("authorization") || "";

  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (cookie) headers["cookie"] = cookie;
  if (authHeader) headers["authorization"] = authHeader;

  let body: any = {};
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ error: "Invalid JSON body" }, { status: 400 });
  }

  const { path: filePath, content } = body;
  if (!filePath) {
    return NextResponse.json({ error: "File path is required" }, { status: 400 });
  }

  // 1. Try backend save endpoint
  try {
    const res = await fetch(`${backend}/api/sandboxes/${id}/file`, {
      method: "PUT",
      headers,
      body: JSON.stringify({ path: filePath, content: content ?? "" }),
    });
    if (res.ok) {
      const data = await res.json();
      return NextResponse.json(data);
    }
  } catch {
    // Fallback to direct disk write
  }

  // 2. Fallback: write directly to disk workspace
  try {
    const workDir = resolveWorkDir(id);
    const cleanRelPath = path.normalize(filePath).replace(/^(\.\.[\/\\])+/, "");
    const fullPath = path.join(workDir, cleanRelPath);

    fs.mkdirSync(path.dirname(fullPath), { recursive: true });
    fs.writeFileSync(fullPath, content ?? "", "utf-8");

    return NextResponse.json({
      message: "File saved successfully",
      path: cleanRelPath,
      redeploying: false,
    });
  } catch (err: unknown) {
    return NextResponse.json({ error: (err as Error).message }, { status: 500 });
  }
}
