package api

import (
	"encoding/json"
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
		api.GET("/environments", GetEnvironments)
		api.POST("/environments", CreateEnvironment)
		api.GET("/environments/:id", GetEnvironment)
		api.DELETE("/environments/:id", DeleteEnvironment)
		api.GET("/environments/:id/logs/stream", StreamLogs)
	}
}

func GetEnvironments(c *gin.Context) {
	var environments []models.Environment
	if err := db.DB.Order("created_at desc").Find(&environments).Error; err != nil {
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

	env := models.Environment{
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
	var env models.Environment
	if err := db.DB.Preload("Logs").Preload("Metrics").First(&env, "id = ?", id).Error; err != nil {
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
	var env models.Environment
	if err := db.DB.First(&env, "id = ?", id).Error; err != nil {
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

