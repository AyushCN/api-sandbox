package api

import (
	"net/http"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
)

type AddRepositoryRequest struct {
	Name          string `json:"name" binding:"required"`
	GitURL        string `json:"gitUrl" binding:"required"`
	DefaultBranch string `json:"defaultBranch"`
}

func AddProjectRepository(c *gin.Context) {
	projectID := c.Param("projectId")

	var req AddRepositoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	branch := req.DefaultBranch
	if branch == "" {
		branch = "main"
	}

	repo := models.ProjectRepository{
		ProjectID:     projectID,
		Name:          req.Name,
		GitURL:        req.GitURL,
		DefaultBranch: branch,
	}

	if err := db.DB.Create(&repo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add repository"})
		return
	}

	c.JSON(http.StatusCreated, repo)
}

func GetProjectRepositories(c *gin.Context) {
	projectID := c.Param("projectId")

	var repos []models.ProjectRepository
	if err := db.DB.Where("project_id = ?", projectID).Find(&repos).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch repositories"})
		return
	}

	c.JSON(http.StatusOK, repos)
}

func RemoveProjectRepository(c *gin.Context) {
	projectID := c.Param("projectId")
	repoID := c.Param("repositoryId")

	if err := db.DB.Where("id = ? AND project_id = ?", repoID, projectID).Delete(&models.ProjectRepository{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove repository"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Repository removed successfully"})
}
