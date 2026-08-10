package api

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
)

type GitActivity struct {
	Timestamp     time.Time `json:"timestamp"`
	TimestampStr  string    `json:"timestampStr"`
	ActorName     string    `json:"actorName"`
	ActorEmail    string    `json:"actorEmail"`
	Action        string    `json:"action"`
	Message       string    `json:"message"`
	Hash          string    `json:"hash"`
	EnvironmentID string    `json:"environmentId"`
	Branch        string    `json:"branch"`
}

func GetProjectActivity(c *gin.Context) {
	projectID := c.Param("projectId")

	var envs []models.Environment
	if err := db.DB.Where("project_id = ?", projectID).Find(&envs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch project environments"})
		return
	}

	activityMap := make(map[string]GitActivity) // Deduplicate by hash
	
	wd, err := os.Getwd()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get working directory"})
		return
	}

	for _, env := range envs {
		workspaceDir := filepath.Join(wd, "workspaces", env.ID)
		if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
			continue
		}

		cmd := exec.Command("git", "log", "--all", "--format=%H|%an|%ae|%s|%aI|%D", "-n", "100")
		cmd.Dir = workspaceDir
		output, _ := cmd.Output()

		for _, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			parts := strings.SplitN(line, "|", 6)
			if len(parts) < 5 {
				continue
			}

			hash := parts[0]
			name := parts[1]
			email := parts[2]
			msg := parts[3]
			timeStr := parts[4]
			refs := ""
			if len(parts) >= 6 {
				refs = parts[5]
			}

			t, err := time.Parse(time.RFC3339, timeStr)
			if err != nil {
				continue
			}

			// We keep the activity if it hasn't been seen, or if this one has branch refs
			existing, exists := activityMap[hash]
			if !exists || (existing.Branch == "" && refs != "") {
				branchStr := env.GithubBranch
				if refs != "" {
					branchStr = refs
				}

				activityMap[hash] = GitActivity{
					Timestamp:     t,
					TimestampStr:  timeStr,
					ActorName:     name,
					ActorEmail:    email,
					Action:        "pushed",
					Message:       msg,
					Hash:          hash[:8],
					EnvironmentID: env.ID,
					Branch:        branchStr,
				}
			}
		}
	}

	var activities []GitActivity
	for _, act := range activityMap {
		activities = append(activities, act)
	}

	// Sort descending by timestamp
	sort.Slice(activities, func(i, j int) bool {
		return activities[i].Timestamp.After(activities[j].Timestamp)
	})

	// Limit to top 50
	if len(activities) > 50 {
		activities = activities[:50]
	}

	c.JSON(http.StatusOK, gin.H{
		"activities": activities,
		"total":      len(activities),
	})
}

func GetProjectTeamStatus(c *gin.Context) {
	projectID := c.Param("projectId")

	var envs []models.Environment
	if err := db.DB.Preload("User").Where("project_id = ?", projectID).Find(&envs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch project environments"})
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get working directory"})
		return
	}

	var branches []map[string]interface{}
	var blockers []map[string]interface{}

	for _, env := range envs {
		workspaceDir := filepath.Join(wd, "workspaces", env.ID)
		if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
			continue
		}

		// Get uncommitted edits
		cmdStatus := exec.Command("git", "status", "--porcelain")
		cmdStatus.Dir = workspaceDir
		statusOut, _ := cmdStatus.Output()
		statusLines := strings.Split(strings.TrimSpace(string(statusOut)), "\n")
		hasUncommitted := len(statusLines) > 0 && statusLines[0] != ""

		// Get latest commit
		cmdLog := exec.Command("git", "log", "-1", "--format=%H|%s|%aI")
		cmdLog.Dir = workspaceDir
		logOut, _ := cmdLog.Output()
		logParts := strings.Split(strings.TrimSpace(string(logOut)), "|")

		latestCommit := map[string]string{}
		if len(logParts) >= 3 {
			latestCommit = map[string]string{
				"hash":    logParts[0][:8],
				"message": logParts[1],
				"time":    logParts[2],
			}
		}

		// Mock ahead/behind vs main
		// In a real scenario we'd do git rev-list count, but here we simulate or skip
		// Let's check if the current branch has conflicts by checking if there's a merge in progress
		mergeHeadPath := filepath.Join(workspaceDir, ".git", "MERGE_HEAD")
		hasConflict := false
		if _, err := os.Stat(mergeHeadPath); err == nil {
			hasConflict = true
		}

		status := "ready_to_merge"
		if hasConflict {
			status = "conflict"
		} else if hasUncommitted {
			status = "in_progress"
		} else if env.Status == models.StatusBuilding {
			status = "building"
		}

		branchInfo := map[string]interface{}{
			"environment_id": env.ID,
			"environment_name": env.Name,
			"name":           env.GithubBranch,
			"status":         status,
			"latest_commit":  latestCommit,
			"author": map[string]string{
				"id":    env.User.ID,
				"name":  env.User.Email, // Using email as name for mock
				"email": env.User.Email,
			},
			"tests_passing":   true, // Mocked as requested
			"reviews_pending": 0,    // Mocked
			"has_uncommitted": hasUncommitted,
		}
		branches = append(branches, branchInfo)

		// Create blocker alerts
		if hasConflict {
			blockers = append(blockers, map[string]interface{}{
				"type":        "conflict",
				"environment": env.Name,
				"branch":      env.GithubBranch,
				"resolution":  "Needs manual merge resolution",
			})
		}
		if hasUncommitted {
			blockers = append(blockers, map[string]interface{}{
				"type":        "uncommitted_changes",
				"environment": env.Name,
				"branch":      env.GithubBranch,
				"resolution":  "Commit or discard changes",
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"branches": branches,
		"blockers": blockers,
	})
}

type CommitRequest struct {
	Message string `json:"message" binding:"required"`
}

func CommitChanges(c *gin.Context) {
	envID := c.Param("id")
	userID, _ := c.Get("userId")
	userIDStr := userID.(string)

	var req CommitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var env models.Environment
	if err := db.DB.Preload("User").First(&env, "id = ?", envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	var user models.User
	if err := db.DB.First(&user, "id = ?", userIDStr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get working directory"})
		return
	}

	workspaceDir := filepath.Join(wd, "workspaces", envID)

	// Configure git user
	name := user.Username
	if name == "" {
		name = "Unknown"
	}
	email := user.Email

	cmdName := exec.Command("git", "config", "user.name", name)
	cmdName.Dir = workspaceDir
	cmdName.Run()

	cmdEmail := exec.Command("git", "config", "user.email", email)
	cmdEmail.Dir = workspaceDir
	cmdEmail.Run()

	// Commit
	cmdCommit := exec.Command("git", "commit", "-m", req.Message)
	cmdCommit.Dir = workspaceDir
	output, err := cmdCommit.CombinedOutput()

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": string(output)})
		return
	}

	// Get new commit hash
	cmdHash := exec.Command("git", "rev-parse", "HEAD")
	cmdHash.Dir = workspaceDir
	commitHash, _ := cmdHash.Output()
	hashStr := strings.TrimSpace(string(commitHash))

	// Update environment
	db.DB.Model(&env).Updates(map[string]interface{}{
		"has_uncommitted_changes": false,
		"commit_hash":             hashStr,
	})

	// Broadcast
	BroadcastToProjectMembers(envID, map[string]interface{}{
		"type":        "committed",
		"commit_hash": hashStr,
		"message":     req.Message,
		"user":        name,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"commit":  hashStr,
		"message": req.Message,
	})
}

