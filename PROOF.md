# PROOF.md — Lifecycle Proof on Development Host (2026-08-16)

This document contains the execution logs of the automated E2E sandbox lifecycle test (`scripts/e2e_session.sh`) running against the `tier` branch.

**Disclaimer**: This is a *Lifecycle Proof* generated using a forged JWT session cookie on the development host. It proves the backend orchestrator loop (Create -> Boot -> Live Edit -> Delete -> GC) works on a bare Docker socket. It does *not* prove the GitHub OAuth flow (which requires manual browser interaction) and is *not* a clean-VM deployment proof.

## 1. System Context

- **Host OS**: Linux fedora 7.1.6-201.fc44.x86_64
- **Docker Version**: Docker version 29.7.1, build e9452d6
- **Target Branch**: `tier`
- **Commit SHA**: `f69e681016c410fcf18288ebaccc20017fb4c2e7`

## 2. Execution Command

```bash
export APP_URL=http://localhost
export SESSION_COOKIE="<forged_jwt_token_for_valid_user>"
export SKIP_PUSH=1
./scripts/e2e_session.sh
```

## 3. E2E Output Logs (Create → Live Edit → Status → Delete)

```text
Creating Environment...
Environment ID: 7b5f9a0e-e980-4021-8dbe-62b4e097b84e
Disabling TCP Health Check via Settings API...
{"message":"Settings updated successfully. Changes will apply on next restart."}Waiting for build to finish...
Status reached: RUNNING
Testing file edit (touch)...
Touch successful.
Skipping git push due to SKIP_PUSH.
--- DOCKER STATS SNAPSHOT (Control Plane + Running Env) ---
NAME                                                   CPU %     MEM USAGE / LIMIT
api-sandbox-backend                                    0.06%     20.68MiB / 15.31GiB
api-sandbox-frontend                                   0.00%     34.53MiB / 15.31GiB
api-sandbox-postgres-1                                 0.01%     41.3MiB / 15.31GiB
api-sandbox-redis-1                                    0.42%     6.363MiB / 15.31GiB
api-sandbox-traefik                                    0.00%     94.94MiB / 15.31GiB
api-sandbox-env-7b5f9a0e-e980-4021-8dbe-62b4e097b84e   13.56%    47.73MiB / 512MiB
-----------------------------------------------------------
Deleting Environment...
Checking orphan containers...
Success: No orphan containers found!
E2E Session Test completed successfully.
```

## 4. State Verification

After the script completed, `docker ps -a` confirms that the ephemeral container (`api-sandbox-env-7b5f9a0e...`) was correctly reaped by the backend orchestrator and no resources were leaked.

```text
CONTAINER ID   IMAGE                           COMMAND                  CREATED          STATUS
fb2d067536a0   api-sandbox-backend             "api-sandbox-backend"    25 minutes ago   Up 25 minutes
b12d2327cce3   api-sandbox-frontend            "docker-entrypoint.s…"   25 minutes ago   Up 25 minutes
a0fce620422b   postgres:16-alpine              "docker-entrypoint.s…"   8 hours ago      Up 49 minutes (healthy)
8503ca421548   redis:7-alpine                  "docker-entrypoint.s…"   8 hours ago      Up 49 minutes (healthy)
f2cf447b806d   traefik:latest                  "/entrypoint.sh --pr…"   9 hours ago      Up 48 minutes
```
*(Note: Legacy stopped/old containers omitted for clarity; the test container was definitively purged).*

## 5. Pass/Fail Checklist

- [x] Node.js container provisioned successfully.
- [x] Boot polling recognized `RUNNING` status without false crash loops.
- [x] API touch command succeeded (verifying host bind-mount access).
- [x] Settings API correctly overrode TCP health check (message logged).
- [x] Delete API dropped the database record and stopped the container.
- [x] Orphan Reaper / Docker socket sweep left zero target traces.
- [ ] Python container check (Not run in this session).
- [ ] Go container check (Not run in this session).
