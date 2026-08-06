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
	"github.com/api-sandbox/backend/queue"
	"github.com/api-sandbox/backend/worker"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"
)

type CreateEnvironmentRequest struct {
	Name         string `json:"name" binding:"required"`
	GitURL       string `json:"gitUrl" binding:"required,url"`
	GithubBranch string `json:"githubBranch"`
}

func SetupRoutes(router *gin.Engine) {
	api := router.Group("/api")
	{
		api.POST("/auth/register", RateLimitRegister(), Register)
		api.POST("/auth/login", RateLimitLogin(), Login)
		api.GET("/auth/verify", RateLimitVerifyEmail(), VerifyEmail)
		api.POST("/auth/forgot-password", RateLimitPasswordReset(), ForgotPassword)
		api.POST("/auth/reset-password", RateLimitPasswordReset(), ResetPassword)

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

	// Get User's Organizations
	var orgMembers []models.OrganizationMember
	db.DB.Where("user_id = ?", userID).Find(&orgMembers)
	var orgIDs []string
	for _, m := range orgMembers {
		orgIDs = append(orgIDs, m.OrganizationID)
	}

	for attempt := 0; attempt < 3; attempt++ {
		query := db.DB.WithContext(context.Background())
		if len(orgIDs) > 0 {
			query = query.Where("organization_id IN ? OR user_id = ?", orgIDs, userID)
		} else {
			query = query.Where("user_id = ?", userID)
		}

		err = query.Order("created_at desc").
			Offset(offset).
			Limit(params.Limit).
			Find(&environments).Error
			
		if err == nil {
			break
		}
		if attempt < 2 {
			time.Sleep(100 * time.Millisecond * time.Duration(math.Pow(2, float64(attempt))))
		}
	}

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

	// Fetch first organization of user to assign environment
	var orgMember models.OrganizationMember
	var orgID string
	if err := db.DB.Where("user_id = ?", uid).First(&orgMember).Error; err == nil {
		orgID = orgMember.OrganizationID
	}

	env := models.Environment{
		UserID:         uid,
		OrganizationID: orgID,
		Name:           req.Name,
		GitURL:         req.GitURL,
		GithubBranch:   req.GithubBranch,
		Status:         models.StatusBuilding,
	}

	if err := db.DB.Create(&env).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create environment"})
		return
	}

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
	
	// Get User's Organizations
	var orgMembers []models.OrganizationMember
	db.DB.Where("user_id = ?", userID).Find(&orgMembers)
	var orgIDs []string
	for _, m := range orgMembers {
		orgIDs = append(orgIDs, m.OrganizationID)
	}

	var err error
	for attempt := 0; attempt < 3; attempt++ {
		query := db.DB.WithContext(context.Background()).
			Preload("Logs", func(db *gorm.DB) *gorm.DB {
				return db.Order("timestamp desc").Limit(100)
			}).
			Preload("Metrics", func(db *gorm.DB) *gorm.DB {
				return db.Order("timestamp desc").Limit(100)
			})

		if len(orgIDs) > 0 {
			query = query.Where("organization_id IN ? OR user_id = ?", orgIDs, userID)
		} else {
			query = query.Where("user_id = ?", userID)
		}

		err = query.First(&env, "id = ?", id).Error
		
		if err == nil {
			break
		}
		if attempt < 2 {
			time.Sleep(100 * time.Millisecond * time.Duration(math.Pow(2, float64(attempt))))
		}
	}

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

func StreamLogs(c *gin.Context) {
	envID := c.Param("id")
	userID, _ := c.Get("userId")
	orgIDs := getUserOrgIDs(userID)

	// Verify access
	var envCount int64
	query := db.DB.Model(&models.Environment{}).Where("id = ?", envID)
	if len(orgIDs) > 0 {
		query = query.Where("organization_id IN ? OR user_id = ?", orgIDs, userID)
	} else {
		query = query.Where("user_id = ?", userID)
	}
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
	orgIDs := getUserOrgIDs(userID)

	var env models.Environment
	query := db.DB
	if len(orgIDs) > 0 {
		query = query.Where("organization_id IN ? OR user_id = ?", orgIDs, userID)
	} else {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.First(&env, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	// Try to stop and remove docker container if it exists
	if env.ContainerID != nil && *env.ContainerID != "" {
		_ = worker.CleanupContainer(c.Request.Context(), *env.ContainerID)
	}
	// Also attempt to cleanup by predictable name, in case it was created but ContainerID wasn't saved
	_ = worker.CleanupContainer(c.Request.Context(), fmt.Sprintf("api-sandbox-env-%s", env.ID))

	// Cleanup workspace folder on host
	_ = worker.CleanupWorkspace(env.ID)

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
	orgIDs := getUserOrgIDs(userID)
	
	var env models.Environment
	query := db.DB
	if len(orgIDs) > 0 {
		query = query.Where("organization_id IN ? OR user_id = ?", orgIDs, userID)
	} else {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.First(&env, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	// Try to stop and remove old docker container if it exists
	if env.ContainerID != nil && *env.ContainerID != "" {
		_ = worker.CleanupContainer(c.Request.Context(), *env.ContainerID)
	}
	// Also attempt to cleanup by predictable name, in case it was created but ContainerID wasn't saved
	_ = worker.CleanupContainer(c.Request.Context(), fmt.Sprintf("api-sandbox-env-%s", env.ID))

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


func getUserOrgIDs(userID interface{}) []string {
	var orgMembers []models.OrganizationMember
	db.DB.Where("user_id = ?", userID).Find(&orgMembers)
	var orgIDs []string
	for _, m := range orgMembers {
		orgIDs = append(orgIDs, m.OrganizationID)
	}
	return orgIDs
}

func GetDockerLogs(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("userId")
	orgIDs := getUserOrgIDs(userID)

	var env models.Environment
	query := db.DB
	if len(orgIDs) > 0 {
		query = query.Where("organization_id IN ? OR user_id = ?", orgIDs, userID)
	} else {
		query = query.Where("user_id = ?", userID)
	}

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
