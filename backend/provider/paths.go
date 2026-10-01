package provider

import (
	"os"
	"path/filepath"
)

// GetWorkspacesRootDir returns the base directory where workspaces are stored.
// Priority: WORKSPACES_DIR env var > HOST_WORKSPACES_DIR env var > CWD/workspaces.
func GetWorkspacesRootDir() string {
	if dir := os.Getenv("WORKSPACES_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("HOST_WORKSPACES_DIR"); dir != "" {
		return dir
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, "workspaces")
}

// GetWorkspacePath returns the absolute path for a workspace's environment.
func GetWorkspacePath(environmentID string) string {
	return filepath.Join(GetWorkspacesRootDir(), environmentID)
}

// GetWorkspaceRepositoryPath returns the path to a specific repository in a multi-repo workspace.
// For single-repo workspaces (numRepos <= 1) the workspace root itself is the repo directory.
// For multi-repo workspaces the resolution order is: explicit WorkingDirectory > project repo name > ID prefix.
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
	if len(workspaceRepoID) >= 8 {
		return filepath.Join(basePath, "repo_"+workspaceRepoID[:8])
	}
	return filepath.Join(basePath, "repo_"+workspaceRepoID)
}

// GetPrimaryRepositoryPath returns the path that runtime detection and container
// start scripts should use.  When there is exactly one repository the workspace
// root IS the repo directory.  When there are multiple repositories the first
// (canonical/primary) entry is used, identified by the lowest sort key.
// Callers that need a specific repository should use GetWorkspaceRepositoryPath.
func GetPrimaryRepositoryPath(environmentID string, repoDirs []string) string {
	if len(repoDirs) == 0 {
		return GetWorkspacePath(environmentID)
	}
	return repoDirs[0]
}
