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

	var orgMembers []models.OrganizationMember
	db.DB.Where("user_id = ?", uid).Find(&orgMembers)

	if len(orgMembers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Must be part of an organization to create projects"})
		return
	}

	project := models.Project{
		Name:                req.Name,
		Description:         req.Description,
		CreatedByUserID:     uid,
		OwnerOrganizationID: orgMembers[0].OrganizationID,
	}

	if err := db.DB.Create(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project"})
		return
	}

	db.DB.Create(&models.ProjectCollaborator{
		ProjectID:       project.ID,
		UserID:          uid,
		Role:            models.ProjectRoleOwner,
		InvitedByUserID: uid,
	})

	c.JSON(http.StatusCreated, gin.H{"project": project})
}

func GetUserProjects(c *gin.Context) {
	userID, _ := c.Get("userId")

	var collabs []models.ProjectCollaborator
	db.DB.Preload("Project").Where("user_id = ?", userID).Find(&collabs)

	var projects []models.Project
	for _, c := range collabs {
		projects = append(projects, c.Project)
	}

	c.JSON(http.StatusOK, projects)
}

func GetProject(c *gin.Context) {
	projectID := c.Param("projectId")

	var project models.Project
	if err := db.DB.Preload("Collaborators").Preload("Collaborators.User").First(&project, "id = ?", projectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	c.JSON(http.StatusOK, project)
}

type InviteToProjectRequest struct {
	Identifier string             `json:"identifier" binding:"required"` // Email or Username
	Role       models.ProjectRole `json:"role" binding:"required"`
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
	if req.Role != models.ProjectRoleAdmin && req.Role != models.ProjectRoleCollaborator && req.Role != models.ProjectRoleViewer {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role specified"})
		return
	}

	// Find the user to invite
	var userToInvite models.User
	if err := db.DB.Where("email = ? OR username = ?", req.Identifier, req.Identifier).First(&userToInvite).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found with the provided email or username"})
		return
	}

	// Check if user is already a collaborator
	var existingCollab models.ProjectCollaborator
	if err := db.DB.Where("project_id = ? AND user_id = ?", projectID, userToInvite.ID).First(&existingCollab).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User is already a collaborator on this project"})
		return
	}

	// Add the collaborator
	newCollab := models.ProjectCollaborator{
		ProjectID:       projectID,
		UserID:          userToInvite.ID,
		Role:            req.Role,
		InvitedByUserID: inviterID,
	}

	if err := db.DB.Create(&newCollab).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add collaborator"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Collaborator added successfully", "collaborator": newCollab})
}

func GetUserInvites(c *gin.Context) {
	userID, _ := c.Get("userId")

	var invites []models.ProjectCollaborator
	if err := db.DB.Preload("Project").Where("user_id = ? AND accepted_at IS NULL", userID).Find(&invites).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch invites"})
		return
	}

	c.JSON(http.StatusOK, invites)
}

func AcceptProjectInvite(c *gin.Context) {
	projectID := c.Param("projectId")
	userID, _ := c.Get("userId")

	var collab models.ProjectCollaborator
	if err := db.DB.Where("project_id = ? AND user_id = ? AND accepted_at IS NULL", projectID, userID).First(&collab).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invite not found or already accepted"})
		return
	}

	now := time.Now()
	collab.AcceptedAt = &now
	if err := db.DB.Save(&collab).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to accept invite"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Invite accepted successfully"})
}

func DeclineProjectInvite(c *gin.Context) {
	projectID := c.Param("projectId")
	userID, _ := c.Get("userId")

	if err := db.DB.Where("project_id = ? AND user_id = ? AND accepted_at IS NULL", projectID, userID).Delete(&models.ProjectCollaborator{}).Error; err != nil {
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

	currentRoleVal, _ := c.Get("projectRole")
	currentRole := currentRoleVal.(models.ProjectRole)

	var targetCollab models.ProjectCollaborator
	if err := db.DB.Where("project_id = ? AND user_id = ?", projectID, targetUserID).First(&targetCollab).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Collaborator not found"})
		return
	}

	// Permission logic
	if currentUserID != targetUserID {
		// Removing someone else
		if currentRole != models.ProjectRoleAdmin && currentRole != models.ProjectRoleOwner {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to remove collaborators"})
			return
		}

		if targetCollab.Role == models.ProjectRoleOwner {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot remove an OWNER. They must leave or transfer ownership."})
			return
		}
	} else {
		// Leaving project
		if targetCollab.Role == models.ProjectRoleOwner {
			var ownerCount int64
			db.DB.Model(&models.ProjectCollaborator{}).Where("project_id = ? AND role = ?", projectID, models.ProjectRoleOwner).Count(&ownerCount)
			if ownerCount <= 1 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "You are the only owner. You cannot leave without assigning another owner or deleting the project."})
				return
			}
		}
	}

	if err := db.DB.Delete(&targetCollab).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove collaborator"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Collaborator removed successfully"})
}
