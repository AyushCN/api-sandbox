package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/api-sandbox/backend/provider"
	"github.com/api-sandbox/backend/queue"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"
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
	}
	c.JSON(http.StatusOK, providers)
}

func GetDeployments(c *gin.Context) {
	userID, _ := c.Get("userId")

	var collabs []models.ProjectCollaborator
	db.DB.Where("user_id = ?", userID).Find(&collabs)

	var projectIDs []string
	for _, collab := range collabs {
		projectIDs = append(projectIDs, collab.ProjectID)
	}

	var deployments []models.Deployment
	if len(projectIDs) > 0 {
		db.DB.Where("project_id IN ?", projectIDs).Order("created_at desc").Find(&deployments)
	} else {
		deployments = []models.Deployment{}
	}

	c.JSON(http.StatusOK, deployments)
}

func CreateDeployment(c *gin.Context) {
	var req struct {
		Name         string `json:"name"`
		ProjectID    string `json:"projectId" binding:"required"`
		GitURL       string `json:"gitUrl" binding:"required"`
		GitBranch    string `json:"gitBranch"`
		ProviderType string `json:"providerType" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ProviderType != "docker" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only docker provider is currently implemented and allowed"})
		return
	}

	if req.GitBranch == "" {
		req.GitBranch = "main"
	}
	
	if req.Name == "" {
		req.Name = "Untitled Deployment"
	}

	deployment := models.Deployment{
		Name:         req.Name,
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

	if err := db.DB.Preload("AddOns").Preload("Logs", func(db *gorm.DB) *gorm.DB {
		return db.Order("timestamp desc").Limit(100)
	}).First(&deployment, "id = ?", id).Error; err != nil {
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

func DeleteDeployment(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("userId")
	
	var dep models.Deployment
	if err := db.DB.First(&dep, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	// Try to stop and remove docker container if it exists
	containerName := fmt.Sprintf("deploy-%s", dep.ID)
	_ = provider.CleanupContainer(c.Request.Context(), containerName, "deployment")

	// Delete associated addons and their containers
	var addons []models.Addon
	db.DB.Where("deployment_id = ?", dep.ID).Find(&addons)
	for _, a := range addons {
		addonContainerName := fmt.Sprintf("%s-%s", a.Type, dep.ID)
		_ = provider.CleanupContainer(c.Request.Context(), addonContainerName, "addon")
	}

	// Delete associated database records manually since GORM might not have configured ON DELETE CASCADE correctly
	db.DB.Where("deployment_id = ?", dep.ID).Delete(&models.Log{})
	db.DB.Where("deployment_id = ?", dep.ID).Delete(&models.Metric{})
	db.DB.Where("deployment_id = ?", dep.ID).Delete(&models.Activity{})
	db.DB.Where("deployment_id = ?", dep.ID).Delete(&models.ProcessType{})
	db.DB.Where("deployment_id = ?", dep.ID).Delete(&models.Addon{})
	db.DB.Where("deployment_id = ?", dep.ID).Delete(&models.EnvironmentChange{})
	db.DB.Where("deployment_id = ?", dep.ID).Delete(&models.BenchmarkRun{})

	// Delete from database
	if err := db.DB.Delete(&dep).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete deployment"})
		return
	}

	db.DB.Create(&models.AuditLog{
		UserID:    fmt.Sprintf("%v", userID),
		Action:    "DELETE_DEPLOYMENT",
		Resource:  dep.ID,
		IPAddress: c.ClientIP(),
	})

	c.JSON(http.StatusOK, gin.H{"message": "Deployment deleted successfully"})
}

func RestartDeployment(c *gin.Context) {
	id := c.Param("id")
	userID, _ := c.Get("userId")

	var dep models.Deployment
	if err := db.DB.First(&dep, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
		return
	}

	// Try to stop and remove old docker container if it exists
	containerName := fmt.Sprintf("deploy-%s", dep.ID)
	_ = provider.CleanupContainer(c.Request.Context(), containerName, "deployment")

	// Update status back to queued/building
	dep.Status = "QUEUED"
	if err := db.DB.Save(&dep).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update deployment status"})
		return
	}

	db.DB.Create(&models.AuditLog{
		UserID:    fmt.Sprintf("%v", userID),
		Action:    "RESTART_DEPLOYMENT",
		Resource:  dep.ID,
		IPAddress: c.ClientIP(),
	})

	// Enqueue the deploy task again
	payload, err := json.Marshal(map[string]string{"deploymentId": dep.ID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize task payload"})
		return
	}

	task := asynq.NewTask(queue.TaskDeploy, payload)
	_, err = queue.Client.Enqueue(task, asynq.MaxRetry(3))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enqueue restart task"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deployment restart initiated"})
}
