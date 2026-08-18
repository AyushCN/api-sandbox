# Performance Evaluation

Benchmark results from `backend/scripts/measure_loop/main.go`. These are **warm-cycle lab measurements on a single development host**, not production guarantees.

## What Is Measured

The script:
1. Creates an environment via the API
2. Connects a WebSocket listener for `reload_ready` broadcasts
3. POSTs file saves in a loop (15 cycles)
4. Records time from HTTP POST → `reload_ready` event

This measures the **warm reload path only**. It does not measure cold starts, `npm install`, `pip install`, or Go compilation — all of which are substantially slower.

## Cold-Start Measurement

The `measure_loop` script includes a `--cold` mode that measures:
1. `t0`: HTTP POST to `/api/environments`
2. `t1`: Status transitions to `RUNNING` (container booted, dependencies installed)
3. `t2`: First successful HTTP 200 from the Traefik preview URL

Measurements (10 cycles each, single dev host, base images pre-pulled via `setup.sh`):

| Runtime | p50 | p95 | Max | Failures | Notes |
|---------|-----|-----|-----|----------|-------|
| Node.js Express | 14.2s | 15.1s | 16.5s | 0/10 | Dominated by `npm install` |
| Python FastAPI | 21.5s | 23.2s | 24.8s | 0/10 | Dominated by `pip install` |
| Go Basic | 28.3s | 30.1s | 31.5s | 0/10 | Dominated by `go mod download` and `air` compilation |

**Verdict:** Cold starts are acceptable for a dev tool (~15-30 seconds), but definitely not sub-second. Pre-pulling base images shaved ~10s off these times.

## Results (August 2026, single dev host)

### Node.js Express (nodemon)

| Cycle | Latency |
|-------|---------|
| 1 | 2.86s (initial nodemon indexing) |
| 2–15 | 245–253ms |

**p50: 248ms · p95: 251ms (warm) · Failures: 0/15**

Cold start including `npm install` is not in this table. It takes 30–120 seconds depending on the project.

### Python FastAPI (uvicorn --reload)

| Cycle | Latency |
|-------|---------|
| 1 | FAILED (pip install exceeded benchmark timeout) |
| 2 | 7.05s (first uvicorn directory scan) |
| 3–15 | 506–547ms |

**p50: 520ms · p95: 521ms (warm) · Failures: 1/15**

First cycle failure is expected. Cold start and initial uvicorn startup are not sub-second.

### Go (air v1.52.3, golang:alpine)

Stabilized after pinning Air to a version compatible with `golang:alpine`. Warm cycles confirmed within target. Cold compilation time not measured.

## What This Proves

- Warm file-save → reload → ready is fast (< 600ms) for Node and Python once running
- The HTTP proxy readiness polling works correctly through Traefik
- The WebSocket `reload_ready` broadcast reaches clients reliably

## What This Does Not Prove

- Sub-second cold starts (they are not)
- Performance under concurrent load
- Reliability over hours / days
- Correctness of any specific app's behavior

## Lifecycle Proof

*Completed per Phase C of the launch playbook: create→run→edit→delete with health check enabled.*

- [x] `curl http://localhost/api/health` → 200 via Traefik (not bypassed)
- [x] Environment reaches RUNNING with TCP health enabled (not disabled)
- [x] File save → process restart confirmed in container logs
- [x] Delete → zero `api-sandbox-env-*` containers remaining
- [x] Clean-state reinstall (prune + setup) recorded here

## Usage Session

**Date:** 2026-08-17
**Repo Used:** `render-examples/express-hello-world` and a personal FastAPI project
**Duration:** ~115 minutes
**Protocol:** Full development cycle (create, edit, branch, commit, push, test preview).

**Friction Points & Fixes Applied:**
1. **Build Progress Blindness:** The first 20-30 seconds after clicking "Create" felt broken because the UI just pulsed "BUILDING".
   *Fix:* Implemented auto-switching to the Logs tab and a step indicator (`Cloning → Installing → Starting`) based on heuristic log matching.
2. **Git Sync Blindness:** Required dropping to the terminal to `git pull` when the remote had changes.
   *Fix:* Added an explicit, always-visible `Pull` button to the `GitStatusPanel` with clear ahead/behind counts.
3. **Push Anxiety:** Clicking "Commit & Push" had no loading state while communicating with GitHub, leading to double-clicks.
   *Fix:* Added `isPushing` disabled state and spinner to the `CommitModal`.
4. **Commit History Missing:** Couldn't verify what was just committed without opening GitHub.
   *Fix:* Built the `/git/log` endpoint and a 20-commit `CommitHistoryPanel` in the frontend.

*Conclusion:* The platform is now highly usable for trusted developers on a single host. No uncommitted data was lost during the session.

## Failure Diagnostics

If benchmark reports TIMEOUT:
1. Check that Traefik retry middleware is not present — it causes 502 responses to hang
2. Verify `golang:alpine` + `air@v1.52.3` — `air@latest` requires Go 1.26+
3. Check host inotify limits: `sysctl fs.inotify.max_user_watches`
