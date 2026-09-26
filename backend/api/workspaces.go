package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/api-sandbox/backend/queue"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

func GetProjectWorkspaces(c *gin.Context) {
	projectID := c.Param("projectId")

	var workspaces []models.Workspace
	if err := db.DB.Where("project_id = ?", projectID).Preload("OwnerUser").Find(&workspaces).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch workspaces"})
		return
	}

	c.JSON(http.StatusOK, workspaces)
}

func GetWorkspace(c *gin.Context) {
	projectID := c.Param("projectId")
	workspaceID := c.Param("workspaceId")

	var workspace models.Workspace
	if err := db.DB.Where("id = ? AND project_id = ?", workspaceID, projectID).Preload("OwnerUser").Preload("Repositories").First(&workspace).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found"})
		return
	}

	c.JSON(http.StatusOK, workspace)
}

func EditProject(c *gin.Context) {
	projectID := c.Param("projectId")
	userID, _ := c.Get("userId")
	uid := userID.(string)

	var member models.ProjectMember
	if err := db.DB.Preload("Project").Where("project_id = ? AND user_id = ? AND status = ?", projectID, uid, models.ProjectMemberStatusAccepted).First(&member).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found or access denied"})
		return
	}

	if member.Role == models.ProjectMemberRoleViewer {
		c.JSON(http.StatusForbidden, gin.H{"error": "Viewers cannot edit this project"})
		return
	}

	if member.Role == models.ProjectMemberRoleOwner {
		var canonicalWorkspace models.Workspace
		if err := db.DB.Where("project_id = ? AND type = ?", projectID, models.WorkspaceTypeCanonical).Preload("Environment").First(&canonicalWorkspace).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Canonical workspace not found"})
			return
		}
		c.JSON(http.StatusOK, canonicalWorkspace)
		return
	}

	// EDITOR logic
	var editorWorkspace models.Workspace
	if err := db.DB.Where("project_id = ? AND owner_user_id = ? AND type = ?", projectID, uid, models.WorkspaceTypeFork).Preload("Environment").First(&editorWorkspace).Error; err == nil {
		c.JSON(http.StatusOK, editorWorkspace)
		return
	}

	// Create a new fork workspace for the editor
	editorWorkspace = models.Workspace{
		ProjectID:   projectID,
		OwnerUserID: uid,
		Type:        models.WorkspaceTypeFork,
		Status:      models.WorkspaceStatusActive,
	}

	// We should create a new Environment for this workspace too
	// Fetch project repos to determine GitURL
	var repos []models.ProjectRepository
	db.DB.Where("project_id = ?", projectID).Find(&repos)

	var gitURL string
	var branch string = "main"
	if len(repos) > 0 {
		gitURL = repos[0].GitURL
		if repos[0].DefaultBranch != "" {
			branch = repos[0].DefaultBranch
		}
	}

	// Create environment
	env := models.Environment{
		UserID:         uid,
		ProjectID:      projectID,
		Name:           fmt.Sprintf("Editor Workspace for %s", uid),
		GitURL:         gitURL,
		GithubBranch:   branch,
		Status:         models.StatusBuilding,
	}

	if err := db.DB.Create(&env).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create environment for workspace"})
		return
	}

	// Add creator as ADMIN to env
	db.DB.Create(&models.EnvironmentMember{
		EnvironmentID: env.ID,
		UserID:        uid,
		Role:          models.EnvRoleAdmin,
	})

	editorWorkspace.EnvironmentID = &env.ID
	if err := db.DB.Create(&editorWorkspace).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create workspace"})
		return
	}

	for _, repo := range repos {
		db.DB.Create(&models.WorkspaceRepository{
			WorkspaceID:         editorWorkspace.ID,
			ProjectRepositoryID: repo.ID,
			Branch:              fmt.Sprintf("editors/%s", uid), // In a real system we would create this branch on the remote or locally
			Status:              "SYNCED",
		})
	}
	
	// Preload the environment so it can be returned
	editorWorkspace.Environment = &env

	// Enqueue build task
	payload, _ := json.Marshal(map[string]string{"environmentId": env.ID})
	task := asynq.NewTask(queue.TaskBuildEnvironment, payload)
	queue.Client.Enqueue(task, asynq.MaxRetry(3))

	c.JSON(http.StatusOK, editorWorkspace)
}

func StartWorkspace(c *gin.Context) {
	projectID := c.Param("projectId")
	workspaceID := c.Param("workspaceId")
	userID, _ := c.Get("userId")
	uid := userID.(string)

	var workspace models.Workspace
	if err := db.DB.Where("id = ? AND project_id = ?", workspaceID, projectID).Preload("Environment").First(&workspace).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found"})
		return
	}

	// Verify ownership or project membership (OWNERs can start any workspace, EDITORs can start their own)
	var member models.ProjectMember
	if err := db.DB.Preload("Project").Where("project_id = ? AND user_id = ? AND status = ?", projectID, uid, models.ProjectMemberStatusAccepted).First(&member).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if member.Role != models.ProjectMemberRoleOwner && workspace.OwnerUserID != uid {
		c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to start this workspace"})
		return
	}

	if workspace.EnvironmentID != nil && workspace.Environment != nil && (workspace.Environment.Status == models.StatusRunning || workspace.Environment.Status == models.StatusBuilding) {
		c.JSON(http.StatusOK, workspace)
		return
	}

	// Fetch project repos to determine GitURL
	var repos []models.ProjectRepository
	db.DB.Where("project_id = ?", projectID).Find(&repos)

	var gitURL string
	var branch string = "main"
	if len(repos) > 0 {
		gitURL = repos[0].GitURL
		if repos[0].DefaultBranch != "" {
			branch = repos[0].DefaultBranch
		}
	}

	env := models.Environment{
		UserID:         workspace.OwnerUserID,
		ProjectID:      projectID,
		Name:           fmt.Sprintf("Workspace Environment %s", workspace.ID),
		GitURL:         gitURL,
		GithubBranch:   branch,
		Status:         models.StatusBuilding,
	}

	if err := db.DB.Create(&env).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create environment"})
		return
	}

	db.DB.Create(&models.EnvironmentMember{
		EnvironmentID: env.ID,
		UserID:        workspace.OwnerUserID,
		Role:          models.EnvRoleAdmin,
	})

	workspace.EnvironmentID = &env.ID
	workspace.Environment = &env
	workspace.Status = models.WorkspaceStatusActive
	db.DB.Save(&workspace)

	payload, _ := json.Marshal(map[string]string{"environmentId": env.ID})
	task := asynq.NewTask(queue.TaskBuildEnvironment, payload)
	queue.Client.Enqueue(task, asynq.MaxRetry(3))

	c.JSON(http.StatusOK, workspace)
}
