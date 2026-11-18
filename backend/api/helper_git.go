package api

import (
	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/api-sandbox/backend/provider"
)

func getEnvironmentPrimaryRepo(envID string) string {
	var workspaceRepos []models.WorkspaceRepository
	db.DB.Preload("ProjectRepository").Where("workspace_id = ?", envID).Find(&workspaceRepos)

	var repoDirs []string
	for _, wRepo := range workspaceRepos {
		d := provider.GetWorkspaceRepositoryPath(envID, len(workspaceRepos), wRepo.WorkingDirectory, wRepo.ProjectRepository.Name, wRepo.ID)
		repoDirs = append(repoDirs, d)
	}

	return provider.GetPrimaryRepositoryPath(envID, repoDirs)
}
