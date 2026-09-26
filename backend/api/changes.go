package api

import (
	"net/http"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
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
		status = models.ChangeRequestStatusMerged
		// In a full implementation, you would perform a git merge from the workspace branch to the canonical branch here.
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action. Must be APPROVE, REJECT, or MERGE"})
		return
	}

	cr.Status = status
	db.DB.Save(&cr)

	c.JSON(http.StatusOK, cr)
}
