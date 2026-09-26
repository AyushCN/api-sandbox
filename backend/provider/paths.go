package provider

import (
	"os"
	"path/filepath"
)

// GetWorkspacePath returns the absolute path for a workspace's environment.
func GetWorkspacePath(environmentID string) string {
	wd, _ := os.Getwd()
	// When running inside docker, the workdir is /app and workspaces are in /app/workspaces.
	// When running locally from backend/, it resolves to backend/workspaces.
	return filepath.Join(wd, "workspaces", environmentID)
}

// GetWorkspaceRepositoryPath returns the path to a specific repository in a multi-repo workspace.
func GetWorkspaceRepositoryPath(environmentID string, numRepos int, dirName string, projectRepoName string, workspaceRepoID string) string {
	basePath := GetWorkspacePath(environmentID)
	if numRepos <= 1 {
		return basePath
	}
	
	if dirName != "" {
		return filepath.Join(basePath, dirName)
	}
	if projectRepoName != "" {
		return filepath.Join(basePath, projectRepoName)
	}
	return filepath.Join(basePath, "repo_" + workspaceRepoID[:8])
}
