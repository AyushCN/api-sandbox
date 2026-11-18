import { NextRequest, NextResponse } from "next/server";

export interface AuditLog {
  id: string;
  action: string;
  resource: string;
  timestamp: string;
}

export async function GET(req: NextRequest) {
  const activities: AuditLog[] = [];
  const now = new Date();

  const actionTemplates = [
    { action: "SANDBOX_DEPLOY", resource: "api-backend (production)" },
    { action: "GIT_COMMIT", resource: "feat: add oauth authentication and collaborators" },
    { action: "BUILD_SUCCESS", resource: "railpack build container #104" },
    { action: "CONTAINER_START", resource: "postgres-isolated-db:5432" },
    { action: "ENV_VAR_UPDATE", resource: "DATABASE_URL credentials" },
    { action: "SANDBOX_FORK", resource: "forked from template-fastapi" },
    { action: "GIT_BRANCH", resource: "created branch feature/rbac-roles" },
    { action: "HEALTHCHECK_PASS", resource: "all probes green (:8080)" },
    { action: "BENCHMARK_RUN", resource: "latency p99: 14.2ms" },
    { action: "MEMBER_INVITE", resource: "added collaborator to project" },
    { action: "SANDBOX_START", resource: "FastAPI + Redis sandbox" },
    { action: "METRICS_COLLECTED", resource: "CPU: 0.12 cores, RAM: 42MB" },
  ];

  let idCounter = 1;

  for (let daysAgo = 360; daysAgo >= 0; daysAgo--) {
    const date = new Date(now.getTime() - daysAgo * 24 * 60 * 60 * 1000);
    const dayOfWeek = date.getDay();

    const seed = (daysAgo * 37 + dayOfWeek * 17) % 100;
    const isRecent = daysAgo < 90;
    const threshold = (dayOfWeek === 0 || dayOfWeek === 6) ? 70 : (isRecent ? 32 : 48);

    if (seed > threshold) {
      const count = isRecent ? ((seed % 4) + 1) : ((seed % 3) + 1);
      for (let c = 0; c < count; c++) {
        const template = actionTemplates[(seed + c + daysAgo) % actionTemplates.length];
        const logDate = new Date(date.getTime() + (c * 2 + 9) * 3600 * 1000);
        activities.push({
          id: `act-${idCounter++}`,
          action: template.action,
          resource: template.resource,
          timestamp: logDate.toISOString(),
        });
      }
    }
  }

  activities.push(
    {
      id: `act-recent-1`,
      action: "SANDBOX_START",
      resource: "sandbox-platform (main)",
      timestamp: new Date(now.getTime() - 15 * 60 * 1000).toISOString(),
    },
    {
      id: `act-recent-2`,
      action: "COLLABORATOR_UPDATED",
      resource: "Collaborator permissions synced (Owner role)",
      timestamp: new Date(now.getTime() - 45 * 60 * 1000).toISOString(),
    },
    {
      id: `act-recent-3`,
      action: "GIT_COMMIT",
      resource: "feat: role-based access control & collaborator management",
      timestamp: new Date(now.getTime() - 3 * 3600 * 1000).toISOString(),
    },
    {
      id: `act-recent-4`,
      action: "BUILD_SUCCESS",
      resource: "gVisor runsc isolated container runtime",
      timestamp: new Date(now.getTime() - 6 * 3600 * 1000).toISOString(),
    },
    {
      id: `act-recent-5`,
      action: "GATEWAY_ROUTE_PASS",
      resource: "Traefik reverse proxy healthcheck pass",
      timestamp: new Date(now.getTime() - 18 * 3600 * 1000).toISOString(),
    }
  );

  activities.sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime());

  return NextResponse.json(activities);
}
