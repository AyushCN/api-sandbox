# Performance Evaluation

Benchmark results from `backend/scripts/measure_loop/main.go`. These are **warm-cycle lab measurements on a single development host**, not production guarantees.

## What Is Measured

The script:
1. Creates an environment via the API
2. Connects a WebSocket listener for `reload_ready` broadcasts
3. POSTs file saves in a loop (15 cycles)
4. Records time from HTTP POST → `reload_ready` event

This measures the **warm reload path only**. It does not measure cold starts, `npm install`, `pip install`, or Go compilation — all of which are substantially slower.

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

*To be completed per Phase C of the launch playbook: create→run→edit→delete with health check enabled.*

- [ ] `curl http://localhost/api/health` → 200 via Traefik (not bypassed)
- [ ] Environment reaches RUNNING with TCP health enabled (not disabled)
- [ ] File save → process restart confirmed in container logs
- [ ] Delete → zero `api-sandbox-env-*` containers remaining
- [ ] Clean-state reinstall (prune + setup) recorded here

## Failure Diagnostics

If benchmark reports TIMEOUT:
1. Check that Traefik retry middleware is not present — it causes 502 responses to hang
2. Verify `golang:alpine` + `air@v1.52.3` — `air@latest` requires Go 1.26+
3. Check host inotify limits: `sysctl fs.inotify.max_user_watches`
