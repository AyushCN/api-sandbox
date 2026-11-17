package api

import (
	"net/http"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
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
		IsDefault:       false,
	}

	if err := db.DB.Create(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project"})
		return
	}

	now := time.Now()

	// Add creator as OWNER in ProjectMember
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
	userIDVal, _ := c.Get("userId")
	userID := userIDVal.(string)

	var members []models.ProjectMember
	db.DB.Preload("Project").
		Preload("Project.CreatedByUser").
		Preload("Project.Members").
		Preload("Project.Members.User").
		Where("user_id = ? AND status = ?", userID, models.ProjectMemberStatusAccepted).
		Find(&members)

	var results []gin.H
	for _, m := range members {
		if m.Project != nil {
			p := m.Project
			isOwner := p.CreatedByUserID == userID || m.Role == models.ProjectMemberRoleOwner
			results = append(results, gin.H{
				"id":            p.ID,
				"name":          p.Name,
				"description":   p.Description,
				"createdBy":     p.CreatedByUserID,
				"isDefault":     p.IsDefault,
				"isPublic":      p.IsPublic,
				"role":          m.Role,
				"userRole":      m.Role,
				"isOwner":       isOwner,
				"members":       p.Members,
				"collaborators": p.Members,
				"createdAt":     p.CreatedAt,
				"updatedAt":     p.UpdatedAt,
			})
		}
	}

	c.JSON(http.StatusOK, results)
}

func GetProject(c *gin.Context) {
	projectID := c.Param("projectId")
	if projectID == "" {
		projectID = c.Param("id")
	}

	var project models.Project
	if err := db.DB.Preload("Members").Preload("Members.User").First(&project, "id = ?", projectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":            project.ID,
		"name":          project.Name,
		"description":   project.Description,
		"createdBy":     project.CreatedByUserID,
		"isDefault":     project.IsDefault,
		"members":       project.Members,
		"collaborators": project.Members,
		"repositories":  project.Repositories,
		"workspaces":    project.Workspaces,
		"environments":  project.Environments,
		"isPublic":      project.IsPublic,
		"createdAt":     project.CreatedAt,
		"updatedAt":     project.UpdatedAt,
	})
}

type InviteToProjectRequest struct {
	Identifier string                   `json:"identifier" binding:"required"` // Email or Username
	Role       models.ProjectMemberRole `json:"role" binding:"required"`
}

func InviteToProject(c *gin.Context) {
	projectID := c.Param("projectId")
	if projectID == "" {
		projectID = c.Param("id")
	}
	userIDVal, _ := c.Get("userId")
	inviterID := userIDVal.(string)

	var req InviteToProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	// Map frontend aliases for role
	if req.Role == "COLLABORATOR" || req.Role == "ADMIN" {
		req.Role = models.ProjectMemberRoleEditor
	}

	// Make sure role is valid (collaborators must be EDITOR or VIEWER, not OWNER)
	if req.Role != models.ProjectMemberRoleEditor && req.Role != models.ProjectMemberRoleViewer {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role specified. Role must be EDITOR or VIEWER."})
		return
	}

	var project models.Project
	if err := db.DB.First(&project, "id = ?", projectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	// CRITICAL REQUIREMENT: Default Project (Personal Space) must NEVER be shared
	if project.IsDefault || project.ID == "default-project" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Default Project (Personal Space) cannot be shared or receive invitations"})
		return
	}

	// Find the user to invite
	var userToInvite models.User
	if err := db.DB.Where("email = ? OR username = ?", req.Identifier, req.Identifier).First(&userToInvite).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found with the provided email or username"})
		return
	}

	// Edge case 1: User inviting themselves
	if userToInvite.ID == inviterID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot invite yourself to your own project"})
		return
	}

	// Edge case 2: Check if user is already an active member
	var existingMember models.ProjectMember
	if err := db.DB.Where("project_id = ? AND user_id = ? AND status = ?", projectID, userToInvite.ID, models.ProjectMemberStatusAccepted).First(&existingMember).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User is already a member of this project"})
		return
	}

	// Edge case 3: Check for duplicate active pending invitation
	now := time.Now()
	var existingInvite models.CollaborationInvitation
	if err := db.DB.Where("project_id = ? AND invitee_id = ? AND status = ? AND expires_at > ?", projectID, userToInvite.ID, models.InvitePending, now).First(&existingInvite).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "An invitation is already pending for this user"})
		return
	}

	// Create CollaborationInvitation with 7 days expiration
	expiresAt := now.Add(7 * 24 * time.Hour)
	invitation := models.CollaborationInvitation{
		ProjectID: projectID,
		InviterID: inviterID,
		InviteeID: userToInvite.ID,
		Role:      req.Role,
		Status:    models.InvitePending,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}

	if err := db.DB.Create(&invitation).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create invitation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Collaboration invitation sent successfully",
		"invitation": invitation,
	})
}

// GetUserInvitations returns all pending received invitations for the authenticated user.
func GetUserInvitations(c *gin.Context) {
	userIDVal, _ := c.Get("userId")
	userID := userIDVal.(string)

	now := time.Now()
	// Lazily mark expired invitations
	db.DB.Model(&models.CollaborationInvitation{}).
		Where("invitee_id = ? AND status = ? AND expires_at <= ?", userID, models.InvitePending, now).
		Update("status", models.InviteExpired)

	var invitations []models.CollaborationInvitation
	if err := db.DB.Preload("Project").Preload("Inviter").
		Where("invitee_id = ? AND status = ? AND expires_at > ?", userID, models.InvitePending, now).
		Order("created_at desc").
		Find(&invitations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch invitations"})
		return
	}

	c.JSON(http.StatusOK, invitations)
}

// AcceptInvitation handles invitation acceptance transactionally.
func AcceptInvitation(c *gin.Context) {
	invitationID := c.Param("id")
	userIDVal, _ := c.Get("userId")
	userID := userIDVal.(string)

	tx := db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var invitation models.CollaborationInvitation
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&invitation, "id = ?", invitationID).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found"})
		return
	}

	// Recipient authorization check
	if invitation.InviteeID != userID {
		tx.Rollback()
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not authorized to accept this invitation"})
		return
	}

	// Status checks
	if invitation.Status == models.InviteRevoked {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invitation was revoked by the project owner"})
		return
	}

	if invitation.Status == models.InviteRejected {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invitation was previously rejected"})
		return
	}

	if invitation.Status == models.InviteExpired || time.Now().After(invitation.ExpiresAt) {
		invitation.Status = models.InviteExpired
		tx.Save(&invitation)
		tx.Commit()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invitation has expired"})
		return
	}

	if invitation.Status == models.InviteAccepted {
		tx.Rollback()
		c.JSON(http.StatusOK, gin.H{"message": "Invitation already accepted", "projectId": invitation.ProjectID})
		return
	}

	// Check that target project is not default
	var project models.Project
	if err := tx.First(&project, "id = ?", invitation.ProjectID).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.IsDefault || project.ID == "default-project" {
		tx.Rollback()
		c.JSON(http.StatusForbidden, gin.H{"error": "Default Project (Personal Space) cannot be shared"})
		return
	}

	now := time.Now()
	invitation.Status = models.InviteAccepted
	invitation.AcceptedAt = &now
	if err := tx.Save(&invitation).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update invitation status"})
		return
	}

	// Create or update ProjectMember
	var member models.ProjectMember
	if err := tx.Where("project_id = ? AND user_id = ?", invitation.ProjectID, userID).First(&member).Error; err != nil {
		member = models.ProjectMember{
			ProjectID:       invitation.ProjectID,
			UserID:          userID,
			Role:            invitation.Role,
			Status:          models.ProjectMemberStatusAccepted,
			InvitedByUserID: &invitation.InviterID,
			InvitedAt:       invitation.CreatedAt,
			AcceptedAt:      &now,
		}
		if err := tx.Create(&member).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project membership"})
			return
		}
	} else {
		member.Role = invitation.Role
		member.Status = models.ProjectMemberStatusAccepted
		member.AcceptedAt = &now
		if err := tx.Save(&member).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update project membership"})
			return
		}
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"message":   "Invitation accepted successfully",
		"projectId": invitation.ProjectID,
		"role":      invitation.Role,
	})
}

// RejectInvitation marks an invitation as REJECTED without granting membership.
func RejectInvitation(c *gin.Context) {
	invitationID := c.Param("id")
	userIDVal, _ := c.Get("userId")
	userID := userIDVal.(string)

	var invitation models.CollaborationInvitation
	if err := db.DB.First(&invitation, "id = ?", invitationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found"})
		return
	}

	if invitation.InviteeID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not authorized to respond to this invitation"})
		return
	}

	invitation.Status = models.InviteRejected
	if err := db.DB.Save(&invitation).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject invitation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Invitation rejected successfully"})
}

// RevokeInvitation allows the project owner to cancel a pending invitation.
func RevokeInvitation(c *gin.Context) {
	projectID := c.Param("projectId")
	invitationID := c.Param("id")

	var invitation models.CollaborationInvitation
	if err := db.DB.Where("id = ? AND project_id = ?", invitationID, projectID).First(&invitation).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invitation not found"})
		return
	}

	if invitation.Status == models.InviteAccepted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot revoke an already accepted invitation. Use remove member instead."})
		return
	}

	invitation.Status = models.InviteRevoked
	if err := db.DB.Save(&invitation).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke invitation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Invitation revoked successfully"})
}

// Backwards compatibility handlers for /api/projects/:projectId/invites/accept and /decline
func AcceptProjectInvite(c *gin.Context) {
	projectID := c.Param("projectId")
	userIDVal, _ := c.Get("userId")
	userID := userIDVal.(string)

	var invitation models.CollaborationInvitation
	if err := db.DB.Where("project_id = ? AND invitee_id = ? AND status = ?", projectID, userID, models.InvitePending).
		Order("created_at desc").First(&invitation).Error; err == nil {
		c.Params = append(c.Params, gin.Param{Key: "id", Value: invitation.ID})
		AcceptInvitation(c)
		return
	}

	// Fallback to legacy ProjectMember pending record if exists
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
	userIDVal, _ := c.Get("userId")
	userID := userIDVal.(string)

	var invitation models.CollaborationInvitation
	if err := db.DB.Where("project_id = ? AND invitee_id = ? AND status = ?", projectID, userID, models.InvitePending).
		Order("created_at desc").First(&invitation).Error; err == nil {
		c.Params = append(c.Params, gin.Param{Key: "id", Value: invitation.ID})
		RejectInvitation(c)
		return
	}

	if err := db.DB.Where("project_id = ? AND user_id = ? AND status = ?", projectID, userID, models.ProjectMemberStatusPending).Delete(&models.ProjectMember{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decline invite"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Invite declined successfully"})
}

// GetProjectMembers returns all members of a project.
func GetProjectMembers(c *gin.Context) {
	projectID := c.Param("projectId")
	if projectID == "" {
		projectID = c.Param("id")
	}

	var members []models.ProjectMember
	if err := db.DB.Preload("User").Where("project_id = ?", projectID).Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch members"})
		return
	}

	c.JSON(http.StatusOK, members)
}

func GetUserInvites(c *gin.Context) {
	GetUserInvitations(c)
}

func RemoveCollaborator(c *gin.Context) {
	projectID := c.Param("projectId")
	if projectID == "" {
		projectID = c.Param("id")
	}
	targetUserID := c.Param("userId")

	currentUserIDVal, _ := c.Get("userId")
	currentUserID := currentUserIDVal.(string)

	currentRoleVal, _ := c.Get("projectMemberRole")
	currentRole := currentRoleVal.(models.ProjectMemberRole)

	var project models.Project
	if err := db.DB.Where("id = ?", projectID).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	// Default project cannot be shared or modified
	if project.IsDefault || project.ID == "default-project" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Default Project (Personal Space) cannot have members removed"})
		return
	}

	var targetMember models.ProjectMember
	if err := db.DB.Where("project_id = ? AND user_id = ?", projectID, targetUserID).First(&targetMember).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Member not found"})
		return
	}

	// Edge case 7: Owner attempting to remove themselves
	if targetUserID == project.CreatedByUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project owner cannot be removed"})
		return
	}

	// Permission logic
	if currentUserID != targetUserID {
		// Removing someone else requires OWNER role
		if currentRole != models.ProjectMemberRoleOwner {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions to remove members. Must be OWNER."})
			return
		}

		if targetMember.Role == models.ProjectMemberRoleOwner {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot remove an OWNER. They must transfer ownership."})
			return
		}
	} else {
		// Leaving project voluntarily
		if targetMember.Role == models.ProjectMemberRoleOwner {
			c.JSON(http.StatusBadRequest, gin.H{"error": "You are the project owner. You cannot leave without transferring ownership."})
			return
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
	if projectID == "" {
		projectID = c.Param("id")
	}
	targetUserID := c.Param("userId")

	var req UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	if req.Role != models.ProjectMemberRoleEditor && req.Role != models.ProjectMemberRoleViewer {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role specified. Role must be EDITOR or VIEWER."})
		return
	}

	var project models.Project
	if err := db.DB.Where("id = ?", projectID).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.IsDefault || project.ID == "default-project" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Default Project (Personal Space) permissions cannot be modified"})
		return
	}

	if targetUserID == project.CreatedByUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot change the role of the project owner"})
		return
	}

	var targetMember models.ProjectMember
	if err := db.DB.Where("project_id = ? AND user_id = ?", projectID, targetUserID).First(&targetMember).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Member not found"})
		return
	}

	targetMember.Role = req.Role
	if err := db.DB.Save(&targetMember).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role updated successfully", "member": targetMember})
}

func DeleteProject(c *gin.Context) {
	projectID := c.Param("projectId")
	if projectID == "" {
		projectID = c.Param("id")
	}

	var project models.Project
	if err := db.DB.Where("id = ?", projectID).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.IsDefault || project.ID == "default-project" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Default Project (Personal Space) cannot be deleted"})
		return
	}

	if err := db.DB.Delete(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete project"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Project deleted successfully"})
}

type TransferOwnershipRequest struct {
	NewOwnerID string `json:"newOwnerId" binding:"required"`
}

func TransferProjectOwnership(c *gin.Context) {
	projectID := c.Param("projectId")
	if projectID == "" {
		projectID = c.Param("id")
	}
	currentOwnerIDVal, _ := c.Get("userId")
	currentOwnerID := currentOwnerIDVal.(string)

	var req TransferOwnershipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	if currentOwnerID == req.NewOwnerID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "You are already the owner"})
		return
	}

	var project models.Project
	if err := db.DB.Where("id = ?", projectID).First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if project.IsDefault || project.ID == "default-project" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Default Project (Personal Space) ownership cannot be transferred"})
		return
	}

	tx := db.DB.Begin()

	// Demote current owner to editor
	var currentOwner models.ProjectMember
	if err := tx.Where("project_id = ? AND user_id = ?", projectID, currentOwnerID).First(&currentOwner).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Current owner member record not found"})
		return
	}
	currentOwner.Role = models.ProjectMemberRoleEditor
	if err := tx.Save(&currentOwner).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to demote current owner"})
		return
	}

	// Promote new owner
	var newOwner models.ProjectMember
	if err := tx.Where("project_id = ? AND user_id = ?", projectID, req.NewOwnerID).First(&newOwner).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Target user is not a member of this project"})
		return
	}

	if newOwner.Status != models.ProjectMemberStatusAccepted {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Target user has not accepted their invite yet"})
		return
	}

	newOwner.Role = models.ProjectMemberRoleOwner
	if err := tx.Save(&newOwner).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to promote new owner"})
		return
	}

	// Update project createdBy to new owner
	project.CreatedByUserID = req.NewOwnerID
	if err := tx.Save(&project).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update project owner"})
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"message": "Ownership transferred successfully"})
}

