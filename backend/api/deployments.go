package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/api-sandbox/backend/queue"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

func GetProviders(c *gin.Context) {
	// Return list of available providers
	providers := []map[string]interface{}{
		{
			"type":        "docker",
			"name":        "Local Docker",
			"description": "Deploy on the internal Docker infrastructure",
			"requires":    []string{},
			"default":     true,
		},
		{
			"type":        "heroku",
			"name":        "Heroku",
			"description": "Deploy to Heroku (Delegated)",
			"requires":    []string{"heroku_api_key"},
		},
		{
			"type":        "railway",
			"name":        "Railway",
			"description": "Deploy to Railway (Delegated)",
			"requires":    []string{"railway_api_key"},
		},
	}
	c.JSON(http.StatusOK, providers)
}

func CreateDeployment(c *gin.Context) {
	var req struct {
		ProjectID    string `json:"projectId" binding:"required"`
		GitURL       string `json:"gitUrl" binding:"required"`
		GitBranch    string `json:"gitBranch"`
		ProviderType string `json:"providerType" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.GitBranch == "" {
		req.GitBranch = "main"
	}

	deployment := models.Deployment{
		ProjectID:    req.ProjectID,
		GitURL:       req.GitURL,
		GitBranch:    req.GitBranch,
		ProviderType: req.ProviderType,
		Status:       "QUEUED",
		Replicas:     1,
	}

	if err := db.DB.Create(&deployment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create deployment record"})
		return
	}

	// Queue background job (worker.QueueDeployJob) to use the appropriate Provider
	taskPayload, _ := json.Marshal(map[string]string{
		"deploymentId": deployment.ID,
	})
	task := asynq.NewTask(queue.TaskDeploy, taskPayload)
	_, err := queue.Client.Enqueue(task)
	if err != nil {
		slog.Error("Failed to enqueue deploy task", "error", err)
	}

	c.JSON(http.StatusCreated, deployment)
}

func GetDeployment(c *gin.Context) {
	id := c.Param("id")
	var deployment models.Deployment

	if err := db.DB.First(&deployment, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	c.JSON(http.StatusOK, deployment)
}

func CreateDeploymentAddon(c *gin.Context) {
	id := c.Param("id")
	
	var req struct {
		Type string `json:"type" binding:"required"`
		Plan string `json:"plan"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var deployment models.Deployment
	if err := db.DB.First(&deployment, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	if req.Plan == "" {
		req.Plan = "free"
	}

	addon := models.Addon{
		DeploymentID: deployment.ID,
		Type:         req.Type,
		Plan:         req.Plan,
	}

	if err := db.DB.Create(&addon).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create addon record"})
		return
	}

	// For Phase 3: Immediate provisioning or defer to deploy phase
	// In a real system, you might queue this or handle it synchronously if it's fast
	// Here, we just store it. DockerProvider.Deploy will provision it and inject the URI.

	c.JSON(http.StatusCreated, addon)
}
