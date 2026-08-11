package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/api-sandbox/backend/provider"
	"github.com/gin-gonic/gin"
)

type FileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"isDir"`
	Children []*FileNode `json:"children,omitempty"`
}

// Helper to build file tree recursively
func buildFileTree(rootDir, currentDir string) ([]*FileNode, error) {
	fullPath := filepath.Join(rootDir, currentDir)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var nodes []*FileNode
	for _, entry := range entries {
		name := entry.Name()
		// Skip VCS, dependencies and large/binary directories
		if name == ".git" || name == "node_modules" || name == ".next" || name == ".nixpacks" || name == "dist" || name == ".cache" {
			continue
		}

		relPath := filepath.Join(currentDir, name)
		node := &FileNode{
			Name:  name,
			Path:  relPath,
			IsDir: entry.IsDir(),
		}

		if entry.IsDir() {
			children, err := buildFileTree(rootDir, relPath)
			if err == nil {
				node.Children = children
			}
		}

		nodes = append(nodes, node)
	}

	// Sort: Directories first, then files alphabetically
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].IsDir && !nodes[j].IsDir {
			return true
		}
		if !nodes[i].IsDir && nodes[j].IsDir {
			return false
		}
		return nodes[i].Name < nodes[j].Name
	})

	return nodes, nil
}

func checkWorkspaceAccess(c *gin.Context, envID string) (*models.Environment, error) {
	userID, _ := c.Get("userId")
	projectIDs := getUserProjectIDs(userID)

	var env models.Environment
	query := db.DB.Preload("User")
	if len(projectIDs) > 0 {
		query = query.Where("project_id IN ? OR user_id = ?", projectIDs, userID)
	} else {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.First(&env, "id = ?", envID).Error; err != nil {
		return nil, fmt.Errorf("environment not found or access denied")
	}

	return &env, nil
}

func GetWorkspaceFiles(c *gin.Context) {
	id := c.Param("id")
	_, err := checkWorkspaceAccess(c, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get working directory"})
		return
	}
	workspaceDir := filepath.Join(wd, "workspaces", id)

	// Ensure the workspace directory exists (if it was somehow removed or not cloned yet)
	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	tree, err := buildFileTree(workspaceDir, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read workspace files: %v", err)})
		return
	}

	c.JSON(http.StatusOK, tree)
}

func GetWorkspaceFileContent(c *gin.Context) {
	id := c.Param("id")
	filePath := c.Query("path")

	_, err := checkWorkspaceAccess(c, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path parameter is required"})
		return
	}

	// Prevent path traversal
	if err := ValidateWorkspacePath(filePath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cleanPath := filepath.Clean(filePath)

	wd, err := os.Getwd()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get working directory"})
		return
	}
	fullPath := filepath.Join(wd, "workspaces", id, cleanPath)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"path":    cleanPath,
		"content": string(content),
	})
}

type UpdateFileRequest struct {
	Path    string `json:"path" binding:"required"`
	Content string `json:"content"`
}

func UpdateWorkspaceFileContent(c *gin.Context) {
	id := c.Param("id")
	var req UpdateFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	env, err := checkWorkspaceAccess(c, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Prevent path traversal
	if err := ValidateWorkspacePath(req.Path); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cleanPath := filepath.Clean(req.Path)

	wd, err := os.Getwd()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get working directory"})
		return
	}
	fullPath := filepath.Join(wd, "workspaces", id, cleanPath)

	// Save code to host workspace
	err = os.WriteFile(fullPath, []byte(req.Content), 0644)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to write file: %v", err)})
		return
	}

	workspaceDir := filepath.Join(wd, "workspaces", id)

	// Get git diff
	cmdDiff := exec.Command("git", "diff", "HEAD", req.Path)
	cmdDiff.Dir = workspaceDir
	diffOut, _ := cmdDiff.Output()

	// Stage file in git
	cmdAdd := exec.Command("git", "add", req.Path)
	cmdAdd.Dir = workspaceDir
	cmdAdd.Run()

	userID, _ := c.Get("userId")
	userIDStr := userID.(string)

	now := time.Now()

	// Update environment uncommitted changes
	db.DB.Model(&env).Updates(map[string]interface{}{
		"has_uncommitted_changes": true,
		"last_modified_at":        now,
		"modified_by_user_id":     userIDStr,
	})

	// Store environment change record
	db.DB.Create(&models.EnvironmentChange{
		EnvironmentID: &env.ID,
		FilePath:      cleanPath,
		ChangeType:    "modified",
		UserID:        userIDStr,
		Diff:          string(diffOut),
	})

	data := map[string]interface{}{
		"type":      "file_changed",
		"file_path": cleanPath,
		"user_id":   userIDStr,
		"user_name": GetCurrentUserName(userIDStr),
		"action":    "save",
		"timestamp": now,
		"diff":      string(diffOut),
	}
	dataBytes, _ := json.Marshal(data)

	db.DB.Create(&models.Activity{
		EnvironmentID: &env.ID,
		Type:          "file_edit",
		Data:          string(dataBytes),
		UserID:        &userIDStr,
	})

	// Broadcast to team via WebSocket
	BroadcastToProjectMembers(env.ID, data)

	// Restart container gracefully to reload server process with updated code
	if env.ContainerID != nil && *env.ContainerID != "" {
		db.DB.Create(&models.Log{
			EnvironmentID: &env.ID,
			Message:       fmt.Sprintf("File %s modified. Restarting container to apply updates...", cleanPath),
			Level:         models.LogLevelInfo,
		})
		err = provider.RestartContainer(c.Request.Context(), *env.ContainerID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to reload sandbox container: %v", err)})
			return
		}

		// Wait a small delay and query new port mapping from container, saving to DB
		time.Sleep(500 * time.Millisecond)
		newPort, err := provider.GetContainerPort(*env.ContainerID)
		if err == nil && newPort > 0 {
			env.Port = &newPort
			db.DB.Save(env)
			db.DB.Create(&models.Log{
				EnvironmentID: &env.ID,
				Message:       fmt.Sprintf("Container restarted and bound to new port %d.", newPort),
				Level:         models.LogLevelInfo,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File updated and environment reloaded successfully",
		"diff":    string(diffOut),
	})
}

type CreateFileOrFolderRequest struct {
	Path  string `json:"path" binding:"required"`
	IsDir bool   `json:"isDir"`
}

func CreateWorkspaceFileOrFolder(c *gin.Context) {
	id := c.Param("id")
	var req CreateFileOrFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	env, err := checkWorkspaceAccess(c, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Prevent path traversal
	if err := ValidateWorkspacePath(req.Path); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cleanPath := filepath.Clean(req.Path)

	wd, err := os.Getwd()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get working directory"})
		return
	}
	fullPath := filepath.Join(wd, "workspaces", id, cleanPath)

	if req.IsDir {
		err = os.MkdirAll(fullPath, 0755)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create folder: %v", err)})
			return
		}
	} else {
		// Ensure parent directory exists
		parentDir := filepath.Dir(fullPath)
		err = os.MkdirAll(parentDir, 0755)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create parent directory: %v", err)})
			return
		}
		// Create empty file if not exists
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			err = os.WriteFile(fullPath, []byte(""), 0644)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create file: %v", err)})
				return
			}
		}
	}

	db.DB.Create(&models.Log{
		EnvironmentID: &env.ID,
		Message: fmt.Sprintf("Created %s: %s", func() string {
			if req.IsDir {
				return "folder"
			}
			return "file"
		}(), cleanPath),
		Level: models.LogLevelInfo,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Created successfully"})
}

type DeleteFileOrFolderRequest struct {
	Path string `json:"path" binding:"required"`
}

// ValidateWorkspacePath ensures the path does not traverse out of the workspace root
func ValidateWorkspacePath(path string) error {
	cleanPath := filepath.Clean(path)
	if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) || cleanPath == "." || cleanPath == "/" || cleanPath == "" {
		return fmt.Errorf("Invalid path: cannot traverse or delete workspace root")
	}
	return nil
}

func DeleteWorkspaceFileOrFolder(c *gin.Context) {
	id := c.Param("id")
	var req DeleteFileOrFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	env, err := checkWorkspaceAccess(c, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Prevent path traversal
	if err := ValidateWorkspacePath(req.Path); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get working directory"})
		return
	}
	cleanPath := filepath.Clean(req.Path)
	fullPath := filepath.Join(wd, "workspaces", id, cleanPath)

	err = os.RemoveAll(fullPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete file or folder: %v", err)})
		return
	}

	db.DB.Create(&models.Log{
		EnvironmentID: &env.ID,
		Message:       fmt.Sprintf("Deleted: %s. Restarting container to apply updates...", cleanPath),
		Level:         models.LogLevelInfo,
	})

	// Restart container gracefully to reload server process with updated files
	if env.ContainerID != nil && *env.ContainerID != "" {
		err = provider.RestartContainer(c.Request.Context(), *env.ContainerID)
		if err == nil {
			time.Sleep(500 * time.Millisecond)
			newPort, err := provider.GetContainerPort(*env.ContainerID)
			if err == nil && newPort > 0 {
				env.Port = &newPort
				db.DB.Save(env)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deleted successfully and environment reloaded"})
}
