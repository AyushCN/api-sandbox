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
		// Find Canonical Workspace
		var canonicalWorkspace models.Workspace
		if err := db.DB.Preload("Environment").Where("project_id = ? AND type = ?", projectID, models.WorkspaceTypeCanonical).First(&canonicalWorkspace).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Canonical workspace not found"})
			return
		}
		
		var sourceWorkspace models.Workspace
		if err := db.DB.Preload("Environment").Where("id = ?", cr.WorkspaceID).First(&sourceWorkspace).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Source workspace not found"})
			return
		}

		var sourceRepos []models.WorkspaceRepository
		db.DB.Preload("ProjectRepository").Where("workspace_id = ?", sourceWorkspace.ID).Find(&sourceRepos)

		var canonicalRepos []models.WorkspaceRepository
		db.DB.Preload("ProjectRepository").Where("workspace_id = ?", canonicalWorkspace.ID).Find(&canonicalRepos)

		if canonicalWorkspace.EnvironmentID == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Canonical workspace has no environment"})
			return
		}
		if sourceWorkspace.EnvironmentID == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Source workspace has no environment"})
			return
		}


		
		type mergeOp struct {
			RepoDir string
			CRepo   models.WorkspaceRepository
		}
		var successfulMerges []mergeOp

		for _, cRepo := range canonicalRepos {
			var sRepo *models.WorkspaceRepository
			for _, sr := range sourceRepos {
				if sr.ProjectRepositoryID == cRepo.ProjectRepositoryID {
					sRepo = &sr
					break
				}
			}
			
			if sRepo != nil {
				repoDir := provider.GetWorkspaceRepositoryPath(*canonicalWorkspace.EnvironmentID, len(canonicalRepos), cRepo.WorkingDirectory, cRepo.ProjectRepository.Name, cRepo.ID)
				editorRepoDir := provider.GetWorkspaceRepositoryPath(*sourceWorkspace.EnvironmentID, len(sourceRepos), sRepo.WorkingDirectory, sRepo.ProjectRepository.Name, sRepo.ID)
				
				fetchCmd := exec.Command("git", "fetch", editorRepoDir, sRepo.Branch)
				fetchCmd.Dir = repoDir
				if err := fetchCmd.Run(); err != nil {
					// Rollback any successful merges
					for _, sm := range successfulMerges {
						exec.Command("git", "-C", sm.RepoDir, "merge", "--abort").Run()
					}
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch from source workspace"})
					return
				}
				
				mergeCmd := exec.Command("git", "merge", "FETCH_HEAD", "--no-commit", "--no-ff")
				mergeCmd.Dir = repoDir
				if err := mergeCmd.Run(); err != nil {
					// Rollback this failed merge
					exec.Command("git", "-C", repoDir, "merge", "--abort").Run()
					// Rollback all previous successful merges
					for _, sm := range successfulMerges {
						exec.Command("git", "-C", sm.RepoDir, "merge", "--abort").Run()
					}

					status = models.ChangeRequestStatusConflicted
					cr.Status = status
					db.DB.Save(&cr)
					c.JSON(http.StatusConflict, gin.H{"error": "Merge conflict detected in repository " + cRepo.ProjectRepository.Name})
					return
				}

				successfulMerges = append(successfulMerges, mergeOp{RepoDir: repoDir, CRepo: cRepo})
			}
		}

		// Phase 2: Commit all successful merges
		for _, sm := range successfulMerges {
			commitCmd := exec.Command("git", "commit", "-m", fmt.Sprintf("Merge change request %s", cr.ID))
			commitCmd.Dir = sm.RepoDir
			commitCmd.Run() // Assuming this succeeds since the merge was clean

			hashCmd := exec.Command("git", "rev-parse", "HEAD")
			hashCmd.Dir = sm.RepoDir
			if hashOut, err := hashCmd.Output(); err == nil {
				hashStr := strings.TrimSpace(string(hashOut))
				db.DB.Model(&sm.CRepo).Update("current_commit", hashStr)
			}
		}

		status = models.ChangeRequestStatusMerged
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action. Must be APPROVE, REJECT, or MERGE"})
		return
	}

	cr.Status = status
	db.DB.Save(&cr)

	c.JSON(http.StatusOK, cr)
}
