# API Sandbox Evaluation Plan

This document outlines an experimental framework for validating the claims, performance, and security boundaries of the API Sandbox orchestration platform. 

These experiments are designed to produce measurable, reproducible data to support academic or engineering evaluations of the system's viability.

**Note: Procedures defined; results TBD until executed.**

## Experiment 1: Cold Start Latency
**Objective:** Measure the time delay from a user requesting an environment to the moment the container is successfully serving HTTP traffic.
**Methodology:**
1. Execute a script that loops 10+ times, calling `POST /api/environments` with a trivial Next.js or Go HTTP repository.
2. The script records `t0` (time of HTTP POST) and `t1` (the exact timestamp the status transitions to `RUNNING`).
3. The script continuously curls the `publicUrl` until it returns HTTP `200 OK`, recording `t2`.
**Metrics to capture:**
- p50 and p95 Cold Start Time ($t_2 - t_0$)
- List of sample repositories used (e.g., standard Next.js template, Express "Hello World").
**Success Criteria:**
- 95th percentile Cold Start Time should be under 45 seconds for cached OCI layers.

## Experiment 2: Organization Network Isolation
**Objective:** Prove that container networks are strictly isolated between tenants and that lateral movement is mathematically impossible at the `bridge` layer.
**Methodology:**
1. Create `Org A` and provision `Env A1` and `Env A2`.
2. Create `Org B` and provision `Env B1`.
3. Use the `/api/environments/:id/files/content` endpoint to inject a python ping/port-scan script into `Env A1`.
4. Trigger a `Commit` and `Sync`.
5. The python script inside `Env A1` will attempt to resolve and `curl` `Env A2`'s IP, `Env B1`'s IP, internal DNS names, and the host gateway IP.
**Metrics to capture:**
- HTTP/ICMP responses from `Env A2` (expected: Success).
- HTTP/ICMP responses or timeout metrics from `Env B1` and host gateway (expected: 100% Packet Loss / Connection Timeout).
**Success Criteria:**
- `Env A1` cannot route ICMP or TCP traffic to `Env B1` or the host gateway.

## Experiment 3: Resource Limit Enforcement (Host Protection)
**Objective:** Validate that a malicious or poorly written sandbox cannot compromise the host node's stability via resource exhaustion.
**Methodology:**
1. Provision a sandbox with a "Fork Bomb" script (`:(){ :|:& };:` in bash, or equivalent in C/Go).
2. Provision a sandbox with a Memory Hog script (allocates a continuous array of 2GB).
3. Monitor the host node's CPU, Memory, and `dmesg` via `htop` and `docker stats`.
**Metrics to capture:**
- Container `OOMKilled` events and `cgroup` outcome logs.
- Host node baseline vs peak memory usage.
- Maximum PIDs spawned by the sandbox.
**Success Criteria:**
- Memory Hog container must be killed by the Linux OOM Killer upon exceeding 512MB without impacting the host's RAM.
- Fork Bomb container must hit the `pids-limit=256` constraint and fail to spawn new processes, preventing host CPU lockup.

## Experiment 4: Queue Concurrency and Rate Limiting
**Objective:** Ensure the control plane survives a barrage of API requests and elegantly queues background work without dropping tasks.
**Methodology:**
1. Using `wrk` or `jmeter`, bombard the `/api/environments` creation endpoint with 50 concurrent requests (50 enqueues) from 5 different users.
2. Measure the API response codes (looking for `429 Too Many Requests`).
3. For the successful `201` requests, monitor the `asynq` Redis queue processing rate.
**Metrics to capture:**
- Ratio of `201 Created` vs `429 Too Many Requests`.
- Total time to drain the `asynq` queue and ensure all requests reach a terminal state (`RUNNING` or `FAILED`).
- Worker error rate (no stuck `BUILDING` states).
**Success Criteria:**
- 100% of the 50 enqueued jobs reach a terminal state.
- No containers stuck infinitely in `BUILDING`.

## Experiment 5: Authorization and IDOR Resistance
**Objective:** Validate that the strict `ProjectRole` checks mathematically prevent horizontal privilege escalation (Insecure Direct Object Reference) across the API.
**Methodology:**
1. Provision `Env A` owned by `User A`.
2. As `User B`, systematically execute `GET`, `POST`, and `DELETE` requests against all protected routes belonging to `Env A`:
   - `GET /api/environments/:id`
   - `POST /api/environments/:id/restart`
   - `DELETE /api/environments/:id`
   - `GET /api/environments/:id/files`
   - `GET /api/environments/:id/logs/stream`
   - `GET /api/ws/environments/:id`
3. Record the HTTP response code for each.
**Metrics to capture:**
- Pass/fail table of HTTP response codes per route for the unauthorized `User B`.
**Success Criteria:**
- 100% of requests from `User B` targeting `Env A`'s routes must return `403 Forbidden` or `404 Not Found`.
