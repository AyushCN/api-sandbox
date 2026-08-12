package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/api-sandbox/backend/provider"
	"github.com/api-sandbox/backend/queue"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type CreateEnvironmentRequest struct {
	Name         string `json:"name" binding:"required"`
	GitURL       string `json:"gitUrl" binding:"required,url"`
	GithubBranch string `json:"githubBranch"`
	DatabaseUrl  string `json:"databaseUrl"`
	ProjectID    string `json:"projectId"` // Now uses projectId
}

func SetupRoutes(router *gin.Engine) {
	api := router.Group("/api")
	{
		api.POST("/auth/register", RateLimitRegister(), Register)
		api.POST("/auth/login", RateLimitLogin(), Login)
		api.POST("/auth/logout", Logout)
		api.GET("/auth/verify", RateLimitVerifyEmail(), VerifyEmail)
		api.POST("/auth/forgot-password", RateLimitPasswordReset(), ForgotPassword)
		api.POST("/auth/reset-password", RateLimitPasswordReset(), ResetPassword)

		projects := api.Group("/projects")
		projects.Use(AuthMiddleware(), RateLimitAPI())
		{
			projects.GET("", GetUserProjects)
			projects.POST("", CreateProject)

			projectDetail := projects.Group("/:projectId")

			// Non-authorized project endpoints (requires token only)
			projectDetail.POST("/invites/accept", AcceptProjectInvite)
			projectDetail.POST("/invites/decline", DeclineProjectInvite)

			// Authorized project endpoints
			projectDetail.Use(AuthorizeProjectAccess(models.ProjectRoleViewer))
			{
				projectDetail.GET("", GetProject)
				projectDetail.GET("/activity", GetProjectActivity)
				projectDetail.GET("/team-status", GetProjectTeamStatus)
				projectDetail.POST("/invite", AuthorizeProjectAccess(models.ProjectRoleAdmin), InviteToProject)
				projectDetail.DELETE("/collaborators/:userId", RemoveCollaborator)
			}
		}
		providers := api.Group("/providers")
		providers.Use(AuthMiddleware(), RateLimitAPI())
		{
			providers.GET("", GetProviders)
		}

		deployments := api.Group("/deployments")
		deployments.Use(AuthMiddleware(), RateLimitAPI())
		{
			deployments.GET("", GetDeployments)
			deployments.POST("", CreateDeployment)
			deployments.GET("/:id", GetDeployment)
			deployments.POST("/:id/addons", CreateDeploymentAddon)
			deployments.POST("/:id/restart", RestartDeployment)
			deployments.DELETE("/:id", DeleteDeployment)
		}

		protected := api.Group("/environments")
		protected.Use(AuthMiddleware(), RateLimitAPI())
		{
			protected.GET("", GetEnvironments)
			protected.POST("", CreateEnvironment)
			protected.GET("/:id", GetEnvironment)
			protected.POST("/:id/restart", RestartEnvironment)
			protected.DELETE("/:id", DeleteEnvironment)
			protected.GET("/:id/logs/stream", StreamLogs)
			protected.GET("/:id/files", GetWorkspaceFiles)
			protected.GET("/:id/files/content", GetWorkspaceFileContent)
			protected.POST("/:id/files/content", UpdateWorkspaceFileContent)
			protected.POST("/:id/files/create", CreateWorkspaceFileOrFolder)
			protected.POST("/:id/files/delete", DeleteWorkspaceFileOrFolder)
			protected.GET("/:id/docker-logs", GetDockerLogs)
			protected.GET("/:id/git-tree", GetGitTree)
			protected.POST("/:id/commit", CommitChanges)
			protected.POST("/:id/sync", SyncEnvironmentWithGitHub)
		}

		wsGroup := api.Group("/ws/environments")
		wsGroup.Use(AuthMiddleware())
		{
			wsGroup.GET("/:id", ServeWS)
		}

		userGroup := api.Group("/user")
		userGroup.Use(AuthMiddleware(), RateLimitAPI())
		{
			userGroup.GET("/me", GetMe)
			userGroup.PUT("/me", UpdateMe)
			userGroup.GET("/activity", GetUserActivity)
			userGroup.PUT("/me/password", ChangePassword)
			userGroup.GET("/invites", GetUserInvites)
		}
	}

	router.GET("/metrics", PrometheusMetrics)
}

func PrometheusMetrics(c *gin.Context) {
	c.Header("Content-Type", "text/plain; version=0.0.4")

	var responseBuilder strings.Builder

	// Database Metrics
	sqlDB, err := db.DB.DB()
	if err == nil {
		stats := sqlDB.Stats()
		responseBuilder.WriteString(fmt.Sprintf("db_connections_open %d\n", stats.OpenConnections))
		responseBuilder.WriteString(fmt.Sprintf("db_connections_in_use %d\n", stats.InUse))
		responseBuilder.WriteString(fmt.Sprintf("db_connections_idle %d\n", stats.Idle))
	}

	// Application Metrics
	var totalEnvs int64
	db.DB.Model(&models.Environment{}).Count(&totalEnvs)
	responseBuilder.WriteString(fmt.Sprintf("total_environments %d\n", totalEnvs))

	var activeEnvs int64
	db.DB.Model(&models.Environment{}).Where("status = ?", models.StatusRunning).Count(&activeEnvs)
	responseBuilder.WriteString(fmt.Sprintf("active_environments %d\n", activeEnvs))

	c.String(http.StatusOK, responseBuilder.String())
}

type PaginationParams struct {
	Page  int `form:"page"`
	Limit int `form:"limit"`
}

func applyEnvironmentScope(query *gorm.DB, userID interface{}) *gorm.DB {
	projectIDs := getUserProjectIDs(userID)
	if len(projectIDs) > 0 {
		return query.Where("project_id IN ? OR user_id = ?", projectIDs, userID)
	}
	return query.Where("user_id = ?", userID)
}

func executeWithRetry(maxAttempts int, initialBackoff time.Duration, fn func() error) error {
	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}
		if attempt < maxAttempts-1 {
			time.Sleep(initialBackoff * time.Duration(math.Pow(2, float64(attempt))))
		}
	}
	return err
}

func GetEnvironments(c *gin.Context) {
	userID, _ := c.Get("userId")

	var params PaginationParams
	_ = c.ShouldBindQuery(&params)
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 || params.Limit > 100 {
		params.Limit = 20
	}
	offset := (params.Page - 1) * params.Limit

	var environments []models.Environment
	var err error

	err = executeWithRetry(3, 100*time.Millisecond, func() error {
		query := applyEnvironmentScope(db.DB.WithContext(context.Background()), userID)
		return query.Order("created_at desc").
			Offset(offset).
			Limit(params.Limit).
			Find(&environments).Error
	})

	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database temporarily unavailable"})
		return
	}

	c.JSON(http.StatusOK, environments)
}

func CreateEnvironment(c *gin.Context) {
	var req CreateEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload", "details": err.Error()})
		return
	}

	if !strings.HasPrefix(req.GitURL, "https://github.com/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only https://github.com/ URLs are supported"})
		return
	}

	if req.GithubBranch == "" {
		req.GithubBranch = "main"
	}

	userID, _ := c.Get("userId")
	uid := userID.(string)

	// Fetch user to get quota limits
	var user models.User
	if err := db.DB.First(&user, "id = ?", uid).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user limits"})
		return
	}

	// 1. Check concurrent running/building environments
	var currentActive int64
	db.DB.Model(&models.Environment{}).
		Where("user_id = ? AND status IN ?", uid, []models.EnvironmentStatus{models.StatusBuilding, models.StatusRunning}).
		Count(&currentActive)

	if currentActive >= int64(user.MaxEnvironments) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": fmt.Sprintf("You have reached the limit of %d concurrent environments. Stop or delete an existing environment first.", user.MaxEnvironments),
		})
		return
	}

	// 2. Check builds per hour
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	var buildsLastHour int64
	db.DB.Model(&models.Environment{}).
		Where("user_id = ? AND created_at > ?", uid, oneHourAgo).
		Count(&buildsLastHour)

	if buildsLastHour >= int64(user.MaxBuildsPerHour) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": fmt.Sprintf("You can only create %d environments per hour. Please try again later.", user.MaxBuildsPerHour),
		})
		return
	}

	// Find project to assign to
	var projectID string
	var orgID string
	if req.ProjectID != "" {
		// Verify access
		var collab models.ProjectCollaborator
		if err := db.DB.Preload("Project").Where("project_id = ? AND user_id = ?", req.ProjectID, uid).First(&collab).Error; err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have access to this project"})
			return
		}
		if collab.Role == models.ProjectRoleViewer {
			c.JSON(http.StatusForbidden, gin.H{"error": "Viewers cannot create environments"})
			return
		}
		projectID = req.ProjectID
		orgID = collab.Project.OwnerOrganizationID
	} else {
		// Fallback for legacy frontend: use first available project, or create one
		var collab models.ProjectCollaborator
		if err := db.DB.Preload("Project").Where("user_id = ?", uid).First(&collab).Error; err == nil {
			projectID = collab.ProjectID
			orgID = collab.Project.OwnerOrganizationID
		} else {
			// No projects exist, create a default one (legacy behavior fallback)
			var orgMember models.OrganizationMember
			if err := db.DB.Where("user_id = ?", uid).First(&orgMember).Error; err == nil {
				defaultProject := models.Project{
					Name:                "Default Workspace",
					OwnerOrganizationID: orgMember.OrganizationID,
					CreatedByUserID:     uid,
				}
				db.DB.Create(&defaultProject)
				db.DB.Create(&models.ProjectCollaborator{
					ProjectID: defaultProject.ID,
					UserID:    uid,
					Role:      models.ProjectRoleOwner,
				})
				projectID = defaultProject.ID
				orgID = orgMember.OrganizationID
			}
		}
	}

	var dbUrlPtr *string
	if req.DatabaseUrl != "" {
		dbUrlPtr = &req.DatabaseUrl
	}

	env := models.Environment{
		UserID:            uid,
		ProjectID:         projectID,
		OrganizationID:    orgID,
		Name:              req.Name,
		GitURL:            req.GitURL,
		GithubBranch:      req.GithubBranch,
		Status:            models.StatusBuilding,
		UserProvidedDBURL: dbUrlPtr,
	}

	if err := db.DB.Create(&env).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create environment"})
		return
	}

	// Add creator as ADMIN
	db.DB.Create(&models.EnvironmentMember{
		EnvironmentID: env.ID,
		UserID:        uid,
		Role:          models.EnvRoleAdmin,
	})

	db.DB.Create(&models.AuditLog{
		UserID:    uid,
		Action:    "CREATE_ENVIRONMENT",
		Resource:  env.ID,
		IPAddress: c.ClientIP(),
	})

	// Enqueue the build task
	payload, err := json.Marshal(map[string]string{"environmentId": env.ID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize task payload"})
		return
	}

	task := asynq.NewTask(queue.TaskBuildEnvironment, payload)
	_, err = queue.Client.Enqueue(task, asynq.MaxRetry(3))
	if err != nil {
		// Log error, but don't fail the response since DB record is created
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue build task"})
		return
	}

	c.JSON(http.StatusCreated, env)
}

func GetEnvironment(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("userId")
	var env models.Environment

	err := executeWithRetry(3, 100*time.Millisecond, func() error {
		query := applyEnvironmentScope(db.DB.WithContext(context.Background()), userID).
			Preload("Logs", func(db *gorm.DB) *gorm.DB {
				return db.Order("timestamp desc").Limit(100)
			}).
			Preload("Metrics", func(db *gorm.DB) *gorm.DB {
				return db.Order("timestamp desc").Limit(100)
			})

		return query.First(&env, "id = ?", id).Error
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		} else {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database temporarily unavailable"})
		}
		return
	}

	c.JSON(http.StatusOK, env)
}

func getUserProjectIDs(userID interface{}) []string {
	var collabs []models.ProjectCollaborator
	db.DB.Where("user_id = ?", userID).Find(&collabs)
	var projectIDs []string
	for _, c := range collabs {
		projectIDs = append(projectIDs, c.ProjectID)
	}
	return projectIDs
}

func StreamLogs(c *gin.Context) {
	envID := c.Param("id")
	userID, _ := c.Get("userId")
	// Verify access
	var envCount int64
	query := applyEnvironmentScope(db.DB.Model(&models.Environment{}), userID).Where("id = ?", envID)
	query.Count(&envCount)
	if envCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found or access denied"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	var lastTimestamp time.Time

	c.Stream(func(w io.Writer) bool {
		var logs []models.Log
		// Query logs after the last seen timestamp
		query := db.DB.Where("environment_id = ?", envID).Order("timestamp asc")
		if !lastTimestamp.IsZero() {
			query = query.Where("timestamp > ?", lastTimestamp)
		}

		if err := query.Find(&logs).Error; err == nil && len(logs) > 0 {
			for _, log := range logs {
				// Format SSE message
				c.SSEvent("message", log.Message)
				lastTimestamp = log.Timestamp
			}
		}

		// Check if environment is still building, if not, we can close the stream eventually
		// For now, keep it open and polling
		time.Sleep(1 * time.Second)
		return true
	})
}

func DeleteEnvironment(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("userId")
	var env models.Environment
	query := applyEnvironmentScope(db.DB, userID)

	if err := query.First(&env, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	// Try to stop and remove docker container if it exists
	if env.ContainerID != nil && *env.ContainerID != "" {
		_ = provider.CleanupContainer(c.Request.Context(), *env.ContainerID, "environment")
	}
	// Also attempt to cleanup by predictable name, in case it was created but ContainerID wasn't saved
	_ = provider.CleanupContainer(c.Request.Context(), fmt.Sprintf("api-sandbox-env-%s", env.ID), "environment")

	// Cleanup workspace folder on host
	_ = provider.CleanupWorkspace(env.ID)

	// Delete associated data first to satisfy foreign key constraints
	db.DB.Where("environment_id = ?", env.ID).Delete(&models.Log{})
	db.DB.Where("environment_id = ?", env.ID).Delete(&models.Metric{})
	db.DB.Where("environment_id = ?", env.ID).Delete(&models.EnvironmentMember{})

	// Delete from database
	if err := db.DB.Delete(&env).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete environment"})
		return
	}

	db.DB.Create(&models.AuditLog{
		UserID:    fmt.Sprintf("%v", userID),
		Action:    "DELETE_ENVIRONMENT",
		Resource:  env.ID,
		IPAddress: c.ClientIP(),
	})

	c.JSON(http.StatusOK, gin.H{"message": "Environment deleted successfully"})
}

func RestartEnvironment(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("userId")
	projectIDs := getUserProjectIDs(userID)

	var env models.Environment
	query := db.DB
	if len(projectIDs) > 0 {
		query = query.Where("project_id IN ? OR user_id = ?", projectIDs, userID)
	} else {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.First(&env, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	// Try to stop and remove old docker container if it exists
	if env.ContainerID != nil && *env.ContainerID != "" {
		_ = provider.CleanupContainer(c.Request.Context(), *env.ContainerID, "environment")
	}
	// Also attempt to cleanup by predictable name, in case it was created but ContainerID wasn't saved
	_ = provider.CleanupContainer(c.Request.Context(), fmt.Sprintf("api-sandbox-env-%s", env.ID), "environment")

	// Delete old logs
	db.DB.Where("environment_id = ?", env.ID).Delete(&models.Log{})

	// Update status back to building
	env.Status = models.StatusBuilding
	env.ContainerID = nil
	if err := db.DB.Save(&env).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update environment status"})
		return
	}

	db.DB.Create(&models.AuditLog{
		UserID:    fmt.Sprintf("%v", userID),
		Action:    "RESTART_ENVIRONMENT",
		Resource:  env.ID,
		IPAddress: c.ClientIP(),
	})

	// Enqueue the build task again
	payload, err := json.Marshal(map[string]string{"environmentId": env.ID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize task payload"})
		return
	}

	task := asynq.NewTask(queue.TaskBuildEnvironment, payload)
	_, err = queue.Client.Enqueue(task, asynq.MaxRetry(3))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue restart task"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Environment restart initiated"})
}

func GetDockerLogs(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("userId")
	var env models.Environment
	query := applyEnvironmentScope(db.DB, userID)

	if err := query.First(&env, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	containerName := fmt.Sprintf("api-sandbox-env-%s", env.ID)

	// Fetch last 500 lines of logs from Docker
	cmd := exec.CommandContext(c.Request.Context(), "docker", "logs", "--tail", "500", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Log the error but return whatever output we got (or a friendly message if empty)
		if len(output) == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch container logs or container is not running."})
			return
		}
	}

	c.String(http.StatusOK, string(output))
}

type MeResponse struct {
	ID               string `json:"id"`
	Email            string `json:"email"`
	IsEmailVerified  bool   `json:"isEmailVerified"`
	MaxEnvironments  int    `json:"maxEnvironments"`
	MaxBuildsPerHour int    `json:"maxBuildsPerHour"`
	Bio              string `json:"bio"`
	Pronouns         string `json:"pronouns"`
	Location         string `json:"location"`
	Website          string `json:"website"`
	Twitter          string `json:"twitter"`
	Github           string `json:"github"`
	CreatedAt        string `json:"createdAt"`
	EnvCount         int64  `json:"envCount"`
	OrgName          string `json:"orgName"`
	OrgRole          string `json:"orgRole"`
}

func GetMe(c *gin.Context) {
	userID, _ := c.Get("userId")

	var user models.User
	if err := db.DB.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get env count
	var envCount int64
	db.DB.Model(&models.Environment{}).Where("user_id = ?", userID).Count(&envCount)

	// Get org membership
	var orgMember models.OrganizationMember
	orgName := ""
	orgRole := ""
	if err := db.DB.Preload("Organization").Where("user_id = ?", userID).First(&orgMember).Error; err == nil {
		orgName = orgMember.Organization.Name
		orgRole = string(orgMember.Role)
	}

	c.JSON(http.StatusOK, MeResponse{
		ID:               user.ID,
		Email:            user.Email,
		IsEmailVerified:  user.IsEmailVerified,
		MaxEnvironments:  user.MaxEnvironments,
		MaxBuildsPerHour: user.MaxBuildsPerHour,
		Bio:              user.Bio,
		Pronouns:         user.Pronouns,
		Location:         user.Location,
		Website:          user.Website,
		Twitter:          user.Twitter,
		Github:           user.Github,
		CreatedAt:        user.CreatedAt.Format(time.RFC3339),
		EnvCount:         envCount,
		OrgName:          orgName,
		OrgRole:          orgRole,
	})
}

type UpdateMeRequest struct {
	Bio      string `json:"bio"`
	Pronouns string `json:"pronouns"`
	Location string `json:"location"`
	Website  string `json:"website"`
	Twitter  string `json:"twitter"`
	Github   string `json:"github"`
}

func UpdateMe(c *gin.Context) {
	userID, _ := c.Get("userId")

	var req UpdateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	var user models.User
	if err := db.DB.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.Bio = req.Bio
	user.Pronouns = req.Pronouns
	user.Location = req.Location
	user.Website = req.Website
	user.Twitter = req.Twitter
	user.Github = req.Github

	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	// Create an audit log for profile update
	db.DB.Create(&models.AuditLog{
		UserID:    user.ID,
		Action:    "UPDATE_PROFILE",
		IPAddress: c.ClientIP(),
	})

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=12"`
}

func ChangePassword(c *gin.Context) {
	userID, _ := c.Get("userId")

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := db.DB.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
		return
	}

	// Validate new password strength
	if err := ValidatePassword(req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Hash and save
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	if err := db.DB.Model(&user).Update("password", string(hashed)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

func GetUserActivity(c *gin.Context) {
	userID, _ := c.Get("userId")

	var activities []models.AuditLog

	// Get last 365 days of activity
	oneYearAgo := time.Now().AddDate(-1, 0, 0)

	if err := db.DB.Where("user_id = ? AND timestamp >= ?", userID, oneYearAgo).
		Order("timestamp desc").
		Find(&activities).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user activity"})
		return
	}

	c.JSON(http.StatusOK, activities)
}
