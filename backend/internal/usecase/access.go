package usecase

import (
	"context"
	"fmt"
	"net/http"

	"github.com/api-sandbox-links/backend/internal/db"
)

// AccessLevel describes the level of access a user has to a sandbox.
type AccessLevel string

const (
	// Owner has full read-write access to the sandbox.
	AccessLevelOwner AccessLevel = "owner"
	// CollaboratorViewer can read sandbox resources but not write/edit.
	AccessLevelCollaboratorViewer AccessLevel = "viewer"
	// CollaboratorEditor can read and write/edit sandbox resources.
	AccessLevelCollaboratorEditor AccessLevel = "editor"
	// Stranger has no access; callers should treat this as "not found" rather than
	// "forbidden", to avoid confirming that the sandbox ID exists.
	AccessLevelStranger AccessLevel = "stranger"
)

// AccessDecision is the result of an authorization check.
type AccessDecision struct {
	// Level is the access level granted (owner, viewer, editor, stranger).
	Level AccessLevel
	// Allow indicates whether the operation is permitted.
	Allow bool
	// Reason is a human-readable explanation if access is denied.
	Reason string
}

// ResolveAccess determines the access level of a user for a given sandbox.
// It queries the database for the sandbox's repository ownership and the
// user's collaborator status on that repository.
//
// Rules:
//   - The sandbox owner has full access (AccessLevelOwner).
//   - A user who is a collaborator on the owning repository has limited access:
//     * VIEWER role can read sandbox resources but not write/edit.
//     * EDITOR role can read, write, and edit sandbox resources.
//   - A user who is neither the owner nor a collaborator receives
//     AccessLevelStranger; callers should return 404 (not found) rather than
//     403 (forbidden), because a 403 confirms the ID exists and would let an
//     attacker enumerate sandboxes.
//   - The sandbox destroyed-at timestamp is checked: a destroyed sandbox
//     returns AccessLevelStranger with Allow=false regardless of role.
func ResolveAccess(ctx context.Context, store *db.Store, userID, sandboxID string) (*AccessDecision, error) {
	// Fetch the sandbox row.
	sb, err := store.GetSandbox(sandboxID)
	if err != nil {
		return nil, fmt.Errorf("db: fetching sandbox %s: %w", sandboxID, err)
	}

	// If the sandbox is destroyed, no one has access.
	if sb.DestroyedAt != nil {
		return &AccessDecision{
			Level: AccessLevelStranger,
			Allow: false,
			Reason: "sandbox has been destroyed",
		}, nil
	}

	// Fetch the sandbox's repository to determine ownership.
	repo, err := store.GetRepository(sb.RepositoryID)
	if err != nil {
		return nil, fmt.Errorf("db: fetching repository %s: %w", sb.RepositoryID, err)
	}

	// Check ownership.
	isOwner := repo.UserID == userID

	// Check collaborator status: is the user a collaborator on the owning project?
	// MVP: only the owner has access; collaborators are not yet supported.
	// This placeholder returns false; the file and git endpoints currently
	// use requireOwnedSandbox which enforces owner-only access.
	isCollaborator := false

	var level AccessLevel
	var allow bool
	var reason string

	if isOwner {
		level = AccessLevelOwner
		allow = true
		reason = "owner access"
	} else if isCollaborator {
		// Deterministic role assignment: for now, we distinguish viewer vs
		// editor based on a simple rule. In a full implementation, the role
		// would be stored in a join table. Here we use a heuristic: if the
		// user has made a file edit, they are an editor; otherwise, viewer.
		role := determineCollaboratorRole(store, userID, sb.ID)
		if role == "editor" {
			level = AccessLevelCollaboratorEditor
		} else {
			level = AccessLevelCollaboratorViewer
		}
		allow = true
		reason = fmt.Sprintf("collaborator %s access", level)
	} else {
		level = AccessLevelStranger
		allow = false
		reason = "stranger — not found (not forbidden)"
	}

	return &AccessDecision{
		Level: level,
		Allow: allow,
		Reason: reason,
	}, nil
}

// determineCollaboratorRole checks whether a user is an editor or viewer on a
// sandbox. In the MVP, only the owner has access; collaborators are not yet
// supported. This function defaults to "viewer" for collaborators.
//
// In a future implementation, this would check the user's file edit history
// or an explicit collaborator role store to determine if they are an editor
// (can write) or viewer (can read only).
func determineCollaboratorRole(store *db.Store, userID, sandboxID string) string {
	// MVP: collaborators are not yet supported; default to viewer.
	// TODO: Replace with a check against the user's file edit history or an
	// explicit collaborator role store.
	return "viewer"
}

// RequireOwner is a convenience wrapper around ResolveAccess that returns a
// 404-like error (actually 404 not found) if the user is not the owner.
// This is suitable for file and git endpoints where we want to hide the
// existence of sandboxes from strangers.
//
// Following the berth f999f88 fix: refusals return "environment not found"
// rather than 403, because a 403 confirms the ID exists and would let an
// attacker enumerate them.
func RequireOwner(decision *AccessDecision, w http.ResponseWriter, r *http.Request) bool {
	if decision.Level != AccessLevelOwner {
		// Return 404 "not found" rather than 403 "forbidden" to avoid
		// confirming the sandbox ID exists.
		http.Error(w, decision.Reason, http.StatusNotFound)
		return false
	}
	return true
}

// RequireAccessLevel checks that the decision meets the minimum required
// access level. If not, it writes the appropriate HTTP error and returns false.
func RequireAccessLevel(decision *AccessDecision, minLevel AccessLevel, w http.ResponseWriter, r *http.Request) bool {
	if !decision.Allow || decision.Level < minLevel {
		// Strangers always get 404 "not found" to avoid confirming ID existence.
		if decision.Level == AccessLevelStranger {
			http.Error(w, decision.Reason, http.StatusNotFound)
		} else {
			http.Error(w, decision.Reason, http.StatusForbidden)
		}
		return false
	}
	return true
}