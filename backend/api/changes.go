package api

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/api-sandbox/backend/provider"
	"github.com/gin-gonic/gin"
)

type CreateChangeRequestPayload struct {
	WorkspaceID string `json:"workspaceId" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
}

func CreateChangeRequest(c *gin.Context) {
	projectID := c.Param("projectId")
	userID, _ := c.Get("userId")
	uid := userID.(string)

	var req CreateChangeRequestPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	// Verify project access
	var member models.ProjectMember
	if err := db.DB.Preload("Project").Where("project_id = ? AND user_id = ? AND status = ?", projectID, uid, models.ProjectMemberStatusAccepted).First(&member).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found or access denied"})
		return
	}

	// Viewers cannot submit changes
	if member.Role == models.ProjectMemberRoleViewer {
		c.JSON(http.StatusForbidden, gin.H{"error": "Viewers cannot submit change requests"})
		return
	}

	// Verify workspace ownership
	var workspace models.Workspace
	if err := db.DB.Where("id = ? AND project_id = ?", req.WorkspaceID, projectID).First(&workspace).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found"})
		return
	}

	if workspace.OwnerUserID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not own this workspace"})
		return
	}

	// Create change request
	cr := models.ChangeRequest{
		ProjectID:       projectID,
		WorkspaceID:     workspace.ID,
		CreatedByUserID: uid,
		Status:          models.ChangeRequestStatusOpen,
		Title:           req.Title,
		Description:     req.Description,
	}

	if err := db.DB.Create(&cr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create change request"})
		return
	}

	// Preload relationships for response
	db.DB.Preload("Workspace").Preload("CreatedByUser").First(&cr, "id = ?", cr.ID)

	c.JSON(http.StatusCreated, cr)
}

func GetChangeRequests(c *gin.Context) {
	projectID := c.Param("projectId")

	var requests []models.ChangeRequest
	if err := db.DB.Where("project_id = ?", projectID).Preload("Workspace").Preload("CreatedByUser").Find(&requests).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch change requests"})
		return
	}

	c.JSON(http.StatusOK, requests)
}

func GetChangeRequest(c *gin.Context) {
	projectID := c.Param("projectId")
	requestID := c.Param("requestId")

	var req models.ChangeRequest
	if err := db.DB.Where("id = ? AND project_id = ?", requestID, projectID).Preload("Workspace").Preload("CreatedByUser").First(&req).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Change request not found"})
		return
	}

	c.JSON(http.StatusOK, req)
}

type ReviewChangeRequestPayload struct {
	Action string `json:"action" binding:"required"` // APPROVED, REJECTED, MERGED
}

// mergeOp records a canonical repo directory that has been successfully staged
// (--no-commit merge) but not yet committed.
type mergeOp struct {
	RepoDir string
	CRepo   models.WorkspaceRepository
}

// abortStagedMerges runs "git merge --abort" on every entry in ops.
// Errors are logged but not returned – we are already in a failure path.
func abortStagedMerges(ops []mergeOp) {
	for _, op := range ops {
		exec.Command("git", "-C", op.RepoDir, "merge", "--abort").Run() //nolint:errcheck
	}
}

func ReviewChangeRequest(c *gin.Context) {
	projectID := c.Param("projectId")
	requestID := c.Param("requestId")
	userID, _ := c.Get("userId")
	uid := userID.(string)

	var req ReviewChangeRequestPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	// Only OWNER can review
	var member models.ProjectMember
	if err := db.DB.Where("project_id = ? AND user_id = ? AND status = ?", projectID, uid, models.ProjectMemberStatusAccepted).First(&member).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found or access denied"})
		return
	}
	if member.Role != models.ProjectMemberRoleOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the project owner can review change requests"})
		return
	}

	var cr models.ChangeRequest
	if err := db.DB.Where("id = ? AND project_id = ?", requestID, projectID).First(&cr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Change request not found"})
		return
	}

	status := models.ChangeRequestStatusOpen
	switch req.Action {
	case "APPROVE":
		status = models.ChangeRequestStatusApproved
	case "REJECT":
		status = models.ChangeRequestStatusRejected
	case "MERGE":
		newStatus, httpStatus, errMsg := performMerge(projectID, &cr)
		if errMsg != "" {
			cr.Status = newStatus
			db.DB.Save(&cr)
			c.JSON(httpStatus, gin.H{"error": errMsg})
			return
		}
		status = newStatus
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action. Must be APPROVE, REJECT, or MERGE"})
		return
	}

	cr.Status = status
	db.DB.Save(&cr)

	c.JSON(http.StatusOK, cr)
}

// performMerge executes the two-pass atomic multi-repo merge.
//
// Pass 1: for every (canonical, source) repository pair, fetch the editor
// branch into the canonical working tree and attempt "git merge --no-commit
// --no-ff".  If any fetch or merge fails every staged merge is aborted and
// the function returns immediately with an appropriate status.
//
// Pass 2: once every merge is clean, commit each repo in turn.  If a commit
// fails the already-committed repos are left (their commits are real and
// correct); the failed repo and the not-yet-committed repos are aborted/reset.
// The CR is left in CONFLICTED status so the owner can retry.
//
// Returns (newStatus, httpStatusCode, errorMessage).  errorMessage is empty on
// success.
func performMerge(projectID string, cr *models.ChangeRequest) (models.ChangeRequestStatus, int, string) {
	// ── Resolve workspaces ────────────────────────────────────────────────────
	var canonicalWorkspace models.Workspace
	if err := db.DB.Preload("Environment").Where("project_id = ? AND type = ?", projectID, models.WorkspaceTypeCanonical).First(&canonicalWorkspace).Error; err != nil {
		return models.ChangeRequestStatusOpen, http.StatusInternalServerError, "Canonical workspace not found"
	}

	var sourceWorkspace models.Workspace
	if err := db.DB.Preload("Environment").Where("id = ?", cr.WorkspaceID).First(&sourceWorkspace).Error; err != nil {
		return models.ChangeRequestStatusOpen, http.StatusInternalServerError, "Source workspace not found"
	}

	if canonicalWorkspace.EnvironmentID == nil {
		return models.ChangeRequestStatusOpen, http.StatusInternalServerError, "Canonical workspace has no environment"
	}
	if sourceWorkspace.EnvironmentID == nil {
		return models.ChangeRequestStatusOpen, http.StatusInternalServerError, "Source workspace has no environment"
	}

	var sourceRepos []models.WorkspaceRepository
	db.DB.Preload("ProjectRepository").Where("workspace_id = ?", sourceWorkspace.ID).Find(&sourceRepos)

	var canonicalRepos []models.WorkspaceRepository
	db.DB.Preload("ProjectRepository").Where("workspace_id = ?", canonicalWorkspace.ID).Find(&canonicalRepos)

	// ── Pass 1: fetch + --no-commit merge ────────────────────────────────────
	var staged []mergeOp

	for _, cRepo := range canonicalRepos {
		var sRepo *models.WorkspaceRepository
		for i := range sourceRepos {
			if sourceRepos[i].ProjectRepositoryID == cRepo.ProjectRepositoryID {
				sRepo = &sourceRepos[i]
				break
			}
		}
		if sRepo == nil {
			// No changes for this repo in the editor workspace – skip.
			continue
		}

		repoDir := provider.GetWorkspaceRepositoryPath(
			*canonicalWorkspace.EnvironmentID,
			len(canonicalRepos),
			cRepo.WorkingDirectory,
			cRepo.ProjectRepository.Name,
			cRepo.ID,
		)
		editorRepoDir := provider.GetWorkspaceRepositoryPath(
			*sourceWorkspace.EnvironmentID,
			len(sourceRepos),
			sRepo.WorkingDirectory,
			sRepo.ProjectRepository.Name,
			sRepo.ID,
		)

		// Fetch the editor branch as a local ref in the canonical repo.
		fetchCmd := exec.Command("git", "fetch", editorRepoDir, sRepo.Branch)
		fetchCmd.Dir = repoDir
		if out, err := fetchCmd.CombinedOutput(); err != nil {
			abortStagedMerges(staged)
			return models.ChangeRequestStatusOpen, http.StatusInternalServerError,
				fmt.Sprintf("Failed to fetch from editor workspace for %s: %s", cRepo.ProjectRepository.Name, strings.TrimSpace(string(out)))
		}

		// Attempt a dry-run merge.  --no-commit prevents any permanent state.
		mergeCmd := exec.Command("git", "merge", "FETCH_HEAD", "--no-commit", "--no-ff")
		mergeCmd.Dir = repoDir
		if out, err := mergeCmd.CombinedOutput(); err != nil {
			// Abort the failing repo before aborting the others.
			exec.Command("git", "-C", repoDir, "merge", "--abort").Run() //nolint:errcheck
			abortStagedMerges(staged)
			return models.ChangeRequestStatusConflicted, http.StatusConflict,
				fmt.Sprintf("Merge conflict in %s: %s", cRepo.ProjectRepository.Name, strings.TrimSpace(string(out)))
		}

		staged = append(staged, mergeOp{RepoDir: repoDir, CRepo: cRepo})
	}

	if len(staged) == 0 {
		// Nothing to merge (no matching repos between workspaces).
		return models.ChangeRequestStatusMerged, 0, ""
	}

	// ── Pass 2: commit each staged merge ─────────────────────────────────────
	var committed []mergeOp

	for _, op := range staged {
		commitMsg := fmt.Sprintf("Merge change request %s", cr.ID)
		commitCmd := exec.Command("git", "commit", "-m", commitMsg)
		commitCmd.Dir = op.RepoDir
		if out, err := commitCmd.CombinedOutput(); err != nil {
			// Abort this repo's staged merge.
			exec.Command("git", "-C", op.RepoDir, "merge", "--abort").Run() //nolint:errcheck

			// Abort remaining not-yet-committed staged merges.
			for _, remaining := range staged[len(committed)+1:] {
				exec.Command("git", "-C", remaining.RepoDir, "merge", "--abort").Run() //nolint:errcheck
			}

			// The already-committed repos cannot be reverted here without a
			// full revert commit – leave them and surface the error clearly.
			return models.ChangeRequestStatusConflicted, http.StatusInternalServerError,
				fmt.Sprintf("Commit failed for %s (already committed: %d/%d repos): %s",
					op.CRepo.ProjectRepository.Name, len(committed), len(staged),
					strings.TrimSpace(string(out)))
		}

		// Record the new HEAD commit hash for the canonical repo.
		hashCmd := exec.Command("git", "rev-parse", "HEAD")
		hashCmd.Dir = op.RepoDir
		if hashOut, err := hashCmd.Output(); err == nil {
			hashStr := strings.TrimSpace(string(hashOut))
			cRepoLocal := op.CRepo // copy to avoid loop variable capture
			db.DB.Model(&cRepoLocal).Update("current_commit", hashStr)
		}

		committed = append(committed, op)
	}

	return models.ChangeRequestStatusMerged, 0, ""
}
