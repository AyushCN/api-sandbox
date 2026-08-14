# API Sandbox Architecture

## Tier 1: Single Coherent Model

**Decision**: The `Deployment` model and all related tables have been removed. `Environment` is now the single abstraction for both interactive workspaces and production deployments.

### Context
Previously, the system maintained parallel paths for `Deployments` (immutable production containers) and `Environments` (interactive sandbox code workspaces). This led to:
- Duplicated API routing and handlers (`/deployments` vs `/environments`).
- Dual container-provisioning pipelines in the background worker.
- Conflicting UI patterns (separate "Deployments" and "Sandboxes" dashboards).
- Authorization drift (e.g. IDOR fixed on Deployments but missing in Environments).

### Implementation
- `Environment` acts as the universal entity.
- The UI refers to them universally as "Environments" or "Sandboxes" under a unified `/dashboard`.
- Database addons (like PostgreSQL, MongoDB, etc.) are provisioned automatically via sidecars if detected in the project, or linked manually via an external `DATABASE_URL`. The legacy manual Addon table UI and endpoints are deprecated to favor this zero-config approach.
- **Breaking Change**: Existing deployment data, process types, legacy addons, and provider configs were discarded.

### Deprecated Tables
The following tables were dropped from PostgreSQL:
- `deployments`
- `process_types`
- `addons`
- `provider_configs`

## Editor Strategy: GitHub-First (Non-Live)

**Decision**: The web editor acts as a view/preparation environment. To run new code, users must commit and sync (push) to GitHub, which triggers a rebuild and restart of the sandbox.

### Context
Since the API Sandbox leverages Nixpacks to build immutable OCI Docker images, all dependencies and build steps are baked into the image. Implementing real-time "live" edits via bind-mounting host directories into these containers contradicts the immutable buildpack architecture and introduces significant fragility. 

### Implementation
- **Non-Live Editor**: Do not imply "save and it runs live" in the UI.
- **Workflow**: Edits -> Commit/Push -> Rebuild -> Restart.
- If true live-reloading is required in the future, it should be built as a separate "Dev Mode" (e.g., using Devcontainers or simple volume mounts with process managers) rather than compromising the current robust Nixpacks workflow.
