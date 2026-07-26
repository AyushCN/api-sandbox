package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/api-sandbox/backend/queue"
	"github.com/api-sandbox/backend/worker"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
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
		api.POST("/auth/login", Login)
		api.GET("/auth/verify", VerifyEmail)
		api.POST("/auth/forgot-password", RateLimitRegister(), ForgotPassword)
		api.POST("/auth/reset-password", ResetPassword)

		protected := api.Group("/environments")
		protected.Use(AuthMiddleware())
		{
			protected.GET("", GetEnvironments)
			protected.POST("", CreateEnvironment)
			protected.GET("/:id", GetEnvironment)
			protected.DELETE("/:id", DeleteEnvironment)
			protected.GET("/:id/logs/stream", StreamLogs)
		}
	}
}

func GetEnvironments(c *gin.Context) {
	userID, _ := c.Get("userId")
	var environments []models.Environment
	if err := db.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&environments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch environments"})
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

	env := models.Environment{
		UserID:       uid,
		Name:         req.Name,
		GitURL:       req.GitURL,
		GithubBranch: req.GithubBranch,
		Status:       models.StatusBuilding,
	}

	if err := db.DB.Create(&env).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create environment"})
		return
	}

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
	if err := db.DB.Preload("Logs").Preload("Metrics").Where("user_id = ?", userID).First(&env, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}
	c.JSON(http.StatusOK, env)
}

func StreamLogs(c *gin.Context) {
	envID := c.Param("id")

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
	if err := db.DB.Where("user_id = ?", userID).First(&env, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	// Try to stop and remove docker container if it exists
	if env.ContainerID != nil && *env.ContainerID != "" {
		_ = worker.CleanupContainer(c.Request.Context(), *env.ContainerID)
	}

	// Delete from database
	if err := db.DB.Delete(&env).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete environment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Environment deleted successfully"})
}

