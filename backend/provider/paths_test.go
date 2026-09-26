package provider

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGetWorkspacePath(t *testing.T) {
	envID := "test-env-123"
	path := GetWorkspacePath(envID)

	if !strings.HasSuffix(path, filepath.Join("workspaces", envID)) {
		t.Errorf("GetWorkspacePath: expected suffix 'workspaces/%s', got '%s'", envID, path)
	}
}

func TestGetWorkspaceRepositoryPath_SingleRepo(t *testing.T) {
	envID := "env-abc"
	path := GetWorkspaceRepositoryPath(envID, 1, "", "myrepo", "wrid-123")

	// Single repo → same as workspace path (no subdirectory)
	expected := GetWorkspacePath(envID)
	if path != expected {
		t.Errorf("Single-repo: expected %s, got %s", expected, path)
	}
}

func TestGetWorkspaceRepositoryPath_MultiRepoWithDirName(t *testing.T) {
	envID := "env-multi"
	path := GetWorkspaceRepositoryPath(envID, 2, "frontend", "", "wr-001")

	if !strings.HasSuffix(path, filepath.Join("workspaces", envID, "frontend")) {
		t.Errorf("MultiRepo with dirName: expected suffix 'workspaces/%s/frontend', got '%s'", envID, path)
	}
}

func TestGetWorkspaceRepositoryPath_MultiRepoWithProjectName(t *testing.T) {
	envID := "env-multi2"
	path := GetWorkspaceRepositoryPath(envID, 2, "", "api-service", "wr-002")

	if !strings.HasSuffix(path, filepath.Join("workspaces", envID, "api-service")) {
		t.Errorf("MultiRepo with projectName: expected suffix 'workspaces/%s/api-service', got '%s'", envID, path)
	}
}

func TestGetWorkspaceRepositoryPath_MultiRepoFallback(t *testing.T) {
	envID := "env-fallback"
	wrid := "abcdefgh-0000-0000-0000-000000000000"
	path := GetWorkspaceRepositoryPath(envID, 2, "", "", wrid)

	if !strings.HasSuffix(path, filepath.Join("workspaces", envID, "repo_"+wrid[:8])) {
		t.Errorf("MultiRepo fallback: expected suffix with repo_%.8s, got '%s'", wrid, path)
	}
}

func TestWorkspacePathsDeterministic(t *testing.T) {
	envID := "det-env"
	p1 := GetWorkspacePath(envID)
	p2 := GetWorkspacePath(envID)
	if p1 != p2 {
		t.Errorf("GetWorkspacePath is not deterministic: %s != %s", p1, p2)
	}

	rp1 := GetWorkspaceRepositoryPath(envID, 2, "src", "", "")
	rp2 := GetWorkspaceRepositoryPath(envID, 2, "src", "", "")
	if rp1 != rp2 {
		t.Errorf("GetWorkspaceRepositoryPath is not deterministic: %s != %s", rp1, rp2)
	}
}

func TestWorkspacePathsIsolation(t *testing.T) {
	// Two editors must have separate paths
	path1 := GetWorkspacePath("editor-env-1")
	path2 := GetWorkspacePath("editor-env-2")
	if path1 == path2 {
		t.Errorf("Two different environments share the same path: %s", path1)
	}

	// Their repo paths must also differ
	rp1 := GetWorkspaceRepositoryPath("editor-env-1", 2, "repo-a", "repo-a", "wr-1")
	rp2 := GetWorkspaceRepositoryPath("editor-env-2", 2, "repo-a", "repo-a", "wr-2")
	if rp1 == rp2 {
		t.Errorf("Two different workspaces share the same repo path: %s", rp1)
	}
}
