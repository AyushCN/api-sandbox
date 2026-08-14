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
