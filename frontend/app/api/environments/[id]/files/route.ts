import { NextRequest, NextResponse } from "next/server";
import fs from "fs";
import path from "path";

export async function GET(
  req: NextRequest,
  props: { params: Promise<{ id: string }> }
) {
  const { id } = await props.params;
  const backend = process.env.BACKEND_URL || "http://localhost:8080";
  const cookie = req.headers.get("cookie") || "";
  const authHeader = req.headers.get("authorization") || "";

  const headers: Record<string, string> = {};
  if (cookie) headers["cookie"] = cookie;
  if (authHeader) headers["authorization"] = authHeader;

  let subdomain = "";
  let envName = "app";

  // 1. Fetch sandbox details from backend to discover subdomain and status
  try {
    const sbRes = await fetch(`${backend}/api/sandboxes/${id}`, {
      headers,
      cache: "no-store",
    });
    if (sbRes.ok) {
      const sbData = await sbRes.json();
      subdomain = sbData.subdomain || "";
      envName = sbData.name || "app";
    }
  } catch {
    // Ignore backend fetch error, fallback to disk checks
  }

  // 2. Try fetching files from backend /api/sandboxes/:id/files
  try {
    const res = await fetch(`${backend}/api/sandboxes/${id}/files`, {
      headers,
      cache: "no-store",
    });
    if (res.ok) {
      const data = await res.json();
      const filesList = data.files || data;
      if (Array.isArray(filesList) && filesList.length > 0) {
        return NextResponse.json({ files: filesList });
      }
    }
  } catch {
    // Backend files endpoint error
  }

  // 3. Fallback: inspect on-disk work directory
  const candidateDirs = [
    subdomain ? path.join("/tmp/api-sandbox-links/work", subdomain) : "",
    path.join("/tmp/api-sandbox-links/work", id),
    path.join("/tmp/api-sandbox/workspaces", id),
    subdomain ? path.join("/tmp/api-sandbox/workspaces", subdomain) : "",
  ].filter(Boolean);

  // Check general base directories for matching prefix
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

  let targetDir = candidateDirs.find((d) => fs.existsSync(d));

  // If work directory does not exist on disk, create and seed initial project files
  if (!targetDir) {
    const preferredWorkDir = subdomain
      ? path.join("/tmp/api-sandbox-links/work", subdomain)
      : path.join("/tmp/api-sandbox-links/work", id);

    try {
      fs.mkdirSync(preferredWorkDir, { recursive: true });
      // Seed default application files
      fs.writeFileSync(
        path.join(preferredWorkDir, "README.md"),
        `# ${subdomain || envName}\n\nLive testing sandbox workspace.\n`
      );
      fs.writeFileSync(
        path.join(preferredWorkDir, "app.py"),
        `# ${subdomain || envName} server\nfrom flask import Flask, jsonify\n\napp = Flask(__name__)\n\n@app.route('/')\ndef index():\n    return jsonify(status="ok", service="${subdomain || envName}")\n\nif __name__ == '__main__':\n    app.run(host='0.0.0.0', port=5000)\n`
      );
      fs.writeFileSync(
        path.join(preferredWorkDir, "requirements.txt"),
        `flask==3.0.0\ngunicorn==21.2.0\nmysql-connector-python==8.2.0\n`
      );
      fs.writeFileSync(
        path.join(preferredWorkDir, "Dockerfile"),
        `FROM python:3.11-slim\nWORKDIR /app\nCOPY requirements.txt .\nRUN pip install --no-cache-dir -r requirements.txt\nCOPY . .\nEXPOSE 5000\nCMD ["python", "app.py"]\n`
      );
      targetDir = preferredWorkDir;
    } catch {
      // Ignore seeding error
    }
  }

  // Walk directory and construct flat files list
  if (targetDir && fs.existsSync(targetDir)) {
    try {
      const files: { path: string; dir: boolean; size: number }[] = [];

      function walk(current: string, rel: string) {
        const items = fs.readdirSync(current, { withFileTypes: true });
        for (const item of items) {
          if (item.name === ".git" || item.name === "node_modules" || item.name === "__pycache__") {
            continue;
          }
          const itemRel = rel ? `${rel}/${item.name}` : item.name;
          const fullPath = path.join(current, item.name);
          const isDirectory = item.isDirectory();

          let size = 0;
          try {
            size = fs.statSync(fullPath).size;
          } catch {
            // Ignore
          }

          files.push({
            path: itemRel,
            dir: isDirectory,
            size,
          });

          if (isDirectory) {
            walk(fullPath, itemRel);
          }
        }
      }

      walk(targetDir, "");
      files.sort((a, b) => a.path.localeCompare(b.path));
      return NextResponse.json({ files });
    } catch {
      // Fallback
    }
  }

  return NextResponse.json({ files: [] });
}
