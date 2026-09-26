package api

import (
	"net/http"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
)

type CreateProjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

func CreateProject(c *gin.Context) {
	userID, _ := c.Get("userId")
	uid := userID.(string)

	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	project := models.Project{
		Name:            req.Name,
		Description:     req.Description,
		CreatedByUserID: uid,
	}

	if err := db.DB.Create(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project"})
		return
	}

	now := time.Now()
	
	// Add new ProjectMember
	db.DB.Create(&models.ProjectMember{
		ProjectID:       project.ID,
		UserID:          uid,
		Role:            models.ProjectMemberRoleOwner,
		Status:          models.ProjectMemberStatusAccepted,
		InvitedByUserID: &uid,
		AcceptedAt:      &now,
	})

	// Create Canonical Workspace
	canonicalWorkspace := models.Workspace{
		ProjectID:   project.ID,
		OwnerUserID: uid,
		Type:        models.WorkspaceTypeCanonical,
		Status:      models.WorkspaceStatusActive,
	}
	db.DB.Create(&canonicalWorkspace)

	c.JSON(http.StatusCreated, gin.H{"project": project})
}

func GetUserProjects(c *gin.Context) {
	userID, _ := c.Get("userId")

	var members []models.ProjectMember
	db.DB.Preload("Project").Where("user_id = ? AND status = ?", userID, models.ProjectMemberStatusAccepted).Find(&members)

	var projects []models.Project
	for _, m := range members {
		if m.Project != nil {
			projects = append(projects, *m.Project)
		}
	}

	c.JSON(http.StatusOK, projects)
}

func GetProject(c *gin.Context) {
	projectID := c.Param("projectId")

	var project models.Project
	if err := db.DB.Preload("Members").Preload("Members.User").First(&project, "id = ?", projectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	c.JSON(http.StatusOK, project)
}

type InviteToProjectRequest struct {
	Identifier string                   `json:"identifier" binding:"required"` // Email or Username
	Role       models.ProjectMemberRole `json:"role" binding:"required"`
}

func InviteToProject(c *gin.Context) {
	projectID := c.Param("projectId")
	userIDVal, _ := c.Get("userId")
	inviterID := userIDVal.(string)

	var req InviteToProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	// Make sure role is valid
	if req.Role != models.ProjectMemberRoleOwner && req.Role != models.ProjectMemberRoleEditor && req.Role != models.ProjectMemberRoleViewer {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role specified"})
		return
	}

	// Fetch the project to ensure we don't invite to Default Workspace
	var project models.Project
	if err := db.DB.First(&project, "id = ?", projectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.Name == "Default Workspace" {
		c.JSON(http.StatusForbidden, gin.H{"error": "You cannot invite members to your Default Workspace. Please create a new project to share with others."})
		return
	}

	// Find the user to invite
	var userToInvite models.User
	if err := db.DB.Where("email = ? OR username = ?", req.Identifier, req.Identifier).First(&userToInvite).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found with the provided email or username"})
		return
	}

	// Check if user is already a member
	var existingMember models.ProjectMember
	if err := db.DB.Where("project_id = ? AND user_id = ?", projectID, userToInvite.ID).First(&existingMember).Error; err == nil {
		if existingMember.Status == models.ProjectMemberStatusAccepted {
			c.JSON(http.StatusConflict, gin.H{"error": "User is already a member of this project"})
		} else {
			c.JSON(http.StatusConflict, gin.H{"error": "User already has a pending invite for this project"})
		}
		return
	}

	// Add the member
	newMember := models.ProjectMember{
		ProjectID:       projectID,
		UserID:          userToInvite.ID,
		Role:            req.Role,
		Status:          models.ProjectMemberStatusPending,
		InvitedByUserID: &inviterID,
	}

	if err := db.DB.Create(&newMember).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add member"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member invited successfully", "member": newMember})
}

func GetUserInvites(c *gin.Context) {
	userID, _ := c.Get("userId")

	var invites []models.ProjectMember
	if err := db.DB.Preload("Project").Where("user_id = ? AND status = ?", userID, models.ProjectMemberStatusPending).Find(&invites).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch invites"})
		return
	}

	c.JSON(http.StatusOK, invites)
}

func AcceptProjectInvite(c *gin.Context) {
	projectID := c.Param("projectId")
	userID, _ := c.Get("userId")

	var member models.ProjectMember
	if err := db.DB.Where("project_id = ? AND user_id = ? AND status = ?", projectID, userID, models.ProjectMemberStatusPending).First(&member).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invite not found or already accepted"})
		return
	}

	now := time.Now()
	member.AcceptedAt = &now
	member.Status = models.ProjectMemberStatusAccepted
	if err := db.DB.Save(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to accept invite"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Invite accepted successfully"})
}

func DeclineProjectInvite(c *gin.Context) {
	projectID := c.Param("projectId")
	userID, _ := c.Get("userId")

	if err := db.DB.Where("project_id = ? AND user_id = ? AND status = ?", projectID, userID, models.ProjectMemberStatusPending).Delete(&models.ProjectMember{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decline invite"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Invite declined successfully"})
}

func RemoveCollaborator(c *gin.Context) {
	projectID := c.Param("projectId")
	targetUserID := c.Param("userId")

	currentUserIDVal, _ := c.Get("userId")
	currentUserID := currentUserIDVal.(string)

	currentRoleVal, _ := c.Get("projectMemberRole")
	currentRole := currentRoleVal.(models.ProjectMemberRole)

	var targetMember models.ProjectMember
	if err := db.DB.Where("project_id = ? AND user_id = ?", projectID, targetUserID).First(&targetMember).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Member not found"})
		return
	}

	// Permission logic
	if currentUserID != targetUserID {
		// Removing someone else
		if currentRole != models.ProjectMemberRoleOwner {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to remove members. Must be OWNER."})
			return
		}

		if targetMember.Role == models.ProjectMemberRoleOwner {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot remove an OWNER. They must leave or transfer ownership."})
			return
		}
	} else {
		// Leaving project
		if targetMember.Role == models.ProjectMemberRoleOwner {
			var ownerCount int64
			db.DB.Model(&models.ProjectMember{}).Where("project_id = ? AND role = ? AND status = ?", projectID, models.ProjectMemberRoleOwner, models.ProjectMemberStatusAccepted).Count(&ownerCount)
			if ownerCount <= 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "You are the only owner. You cannot leave without assigning another owner or deleting the project."})
				return
			}
		}
	}

	if err := db.DB.Delete(&targetMember).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove member"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member removed successfully"})
}

type UpdateMemberRoleRequest struct {
	Role models.ProjectMemberRole `json:"role" binding:"required"`
}

func UpdateMemberRole(c *gin.Context) {
	projectID := c.Param("projectId")
	targetUserID := c.Param("userId")

	var req UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	if req.Role != models.ProjectMemberRoleOwner && req.Role != models.ProjectMemberRoleEditor && req.Role != models.ProjectMemberRoleViewer {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role specified"})
		return
	}

	var targetMember models.ProjectMember
	if err := db.DB.Where("project_id = ? AND user_id = ?", projectID, targetUserID).First(&targetMember).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Member not found"})
		return
	}

	// Permission logic: only OWNER can change roles, which is handled by AuthorizeProjectMemberAccess middleware
	// But we need to ensure they don't remove the last owner
	if targetMember.Role == models.ProjectMemberRoleOwner && req.Role != models.ProjectMemberRoleOwner {
		var ownerCount int64
		db.DB.Model(&models.ProjectMember{}).Where("project_id = ? AND role = ? AND status = ?", projectID, models.ProjectMemberRoleOwner, models.ProjectMemberStatusAccepted).Count(&ownerCount)
		if ownerCount <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot change the role of the only owner."})
			return
		}
	}

	targetMember.Role = req.Role
	if err := db.DB.Save(&targetMember).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role updated successfully", "member": targetMember})
}
