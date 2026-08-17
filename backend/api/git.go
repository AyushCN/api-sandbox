package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
)

type GitNode struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "githubCommit", "localCommit", "localEdits"
	Data     gin.H  `json:"data"`
	Position gin.H  `json:"position"`
}

type GitEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
}

func GetGitTree(c *gin.Context) {
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

	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		c.JSON(http.StatusOK, gin.H{"nodes": []GitNode{}, "edges": []GitEdge{}})
		return
	}

	// 1. Get remote commits
	cmdRemotes := exec.Command("git", "log", "--remotes", "--format=%H")
	cmdRemotes.Dir = workspaceDir
	remotesOut, _ := cmdRemotes.Output()

	githubCommits := make(map[string]bool)
	for _, hash := range strings.Split(string(remotesOut), "\n") {
		hash = strings.TrimSpace(hash)
		if hash != "" {
			githubCommits[hash] = true
		}
	}

	// 2. Get all commits
	cmdAll := exec.Command("git", "log", "--all", "--format=%H|%P|%s|%an|%D")
	cmdAll.Dir = workspaceDir
	allOut, _ := cmdAll.Output()

	var nodes []GitNode
	var edges []GitEdge

	lines := strings.Split(string(allOut), "\n")

	// We want to track if we need an uncommitted edits node.
	// We will attach it to the currently checked out commit (HEAD).
	cmdHead := exec.Command("git", "rev-parse", "HEAD")
	cmdHead.Dir = workspaceDir
	headOut, _ := cmdHead.Output()
	headHash := strings.TrimSpace(string(headOut))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) != 5 {
			continue
		}

		hash := parts[0]
		parentsStr := parts[1]
		msg := parts[2]
		author := parts[3]
		refs := parts[4]

		isGithub := githubCommits[hash]
		nodeType := "localCommit"
		if isGithub {
			nodeType = "githubCommit"
		}

		nodes = append(nodes, GitNode{
			ID:   hash,
			Type: nodeType,
			Data: gin.H{
				"label":  msg,
				"author": author,
				"hash":   hash[:7],
				"refs":   refs,
			},
			Position: gin.H{"x": 0, "y": 0},
		})

		parents := strings.Split(parentsStr, " ")
		for _, parent := range parents {
			if parent != "" {
				edges = append(edges, GitEdge{
					ID:     fmt.Sprintf("%s-%s", hash, parent),
					Source: parent, // Flow goes from parent -> child
					Target: hash,
				})
			}
		}
	}

	// 3. Check for uncommitted edits
	cmdStatus := exec.Command("git", "status", "--porcelain")
	cmdStatus.Dir = workspaceDir
	statusOut, _ := cmdStatus.Output()
	statusLines := strings.Split(strings.TrimSpace(string(statusOut)), "\n")

	if len(statusLines) > 0 && statusLines[0] != "" && headHash != "" {
		var modifiedFiles []string
		for _, line := range statusLines {
			line = strings.TrimSpace(line)
			if len(line) > 2 {
				// Get filename, which starts after the 2-char status code and space
				modifiedFiles = append(modifiedFiles, strings.TrimSpace(line[2:]))
			}
		}

		filesStr := strings.Join(modifiedFiles, ", ")
		if len(filesStr) > 40 {
			filesStr = filesStr[:37] + "..."
		}

		editsID := "uncommitted-edits"
		nodes = append(nodes, GitNode{
			ID:   editsID,
			Type: "localEdits",
			Data: gin.H{
				"label":  "Uncommitted Edits",
				"author": "You",
				"hash":   "",
				"refs":   filesStr,
			},
			Position: gin.H{"x": 0, "y": 0},
		})

		edges = append(edges, GitEdge{
			ID:     fmt.Sprintf("%s-%s", headHash, editsID),
			Source: headHash,
			Target: editsID,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes": nodes,
		"edges": edges,
	})
}

// GitStatus returns the rich status object requested
func GitStatus(c *gin.Context) {
	id := c.Param("id")
	_, err := checkWorkspaceAccess(c, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	wd, _ := os.Getwd()
	workspaceDir := filepath.Join(wd, "workspaces", id)

	// Get dirty state
	cmdStatus := exec.Command("git", "status", "--porcelain")
	cmdStatus.Dir = workspaceDir
	statusOut, _ := cmdStatus.Output()
	statusStr := strings.TrimSpace(string(statusOut))
	dirty := len(statusStr) > 0

	// Get branch and detached state
	cmdBranch := exec.Command("git", "branch", "--show-current")
	cmdBranch.Dir = workspaceDir
	branchOut, _ := cmdBranch.Output()
	branch := strings.TrimSpace(string(branchOut))
	detached := branch == ""

	// Get upstream
	cmdUpstream := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	cmdUpstream.Dir = workspaceDir
	upstreamOut, _ := cmdUpstream.Output()
	upstream := strings.TrimSpace(string(upstreamOut))

	// Get ahead/behind
	ahead := 0
	behind := 0
	if upstream != "" && !strings.Contains(upstream, "fatal:") {
		cmdRevList := exec.Command("git", "rev-list", "--left-right", "--count", fmt.Sprintf("HEAD...%s", upstream))
		cmdRevList.Dir = workspaceDir
		revOut, _ := cmdRevList.Output()
		revParts := strings.Fields(string(revOut))
		if len(revParts) == 2 {
			ahead, _ = strconv.Atoi(revParts[0])
			behind, _ = strconv.Atoi(revParts[1])
		}
	}

	if detached {
		cmdHash := exec.Command("git", "rev-parse", "HEAD")
		cmdHash.Dir = workspaceDir
		hashOut, _ := cmdHash.Output()
		branch = strings.TrimSpace(string(hashOut))
	}

	c.JSON(http.StatusOK, gin.H{
		"branch":   branch,
		"detached": detached,
		"dirty":    dirty,
		"ahead":    ahead,
		"behind":   behind,
		"upstream": upstream,
	})
}

// GitListBranches returns all local and remote branches
func GitListBranches(c *gin.Context) {
	id := c.Param("id")
	_, err := checkWorkspaceAccess(c, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	wd, _ := os.Getwd()
	workspaceDir := filepath.Join(wd, "workspaces", id)

	cmd := exec.Command("git", "branch", "-a", "--format=%(refname:short)")
	cmd.Dir = workspaceDir
	out, err := cmd.Output()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list branches"})
		return
	}

	var branches []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && line != "HEAD" {
			// Deduplicate origin/ branches if they have a local counterpart?
			// For simplicity, we just return all distinct names.
			branches = append(branches, line)
		}
	}

	c.JSON(http.StatusOK, gin.H{"branches": branches})
}

type GitBranchRequest struct {
	Branch string `json:"branch" binding:"required"`
}

func GitBranch(c *gin.Context) {
	id := c.Param("id")
	env, err := checkWorkspaceWriteAccess(c, id)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var req GitBranchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wd, _ := os.Getwd()
	workspaceDir := filepath.Join(wd, "workspaces", id)

	cmd := exec.Command("git", "checkout", "-b", req.Branch)
	cmd.Dir = workspaceDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": string(out)})
		return
	}

	// Update DB
	env.GithubBranch = req.Branch
	db.DB.Save(env)

	TouchEnvironmentActivity(id)

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Created and checked out branch %s", req.Branch)})
}

type GitCheckoutRequest struct {
	Ref   string `json:"ref" binding:"required"`
	Force bool   `json:"force"`
}

func GitCheckout(c *gin.Context) {
	id := c.Param("id")
	env, err := checkWorkspaceWriteAccess(c, id)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	var req GitCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wd, _ := os.Getwd()
	workspaceDir := filepath.Join(wd, "workspaces", id)

	// Dirty working tree protection
	if !req.Force {
		cmdStatus := exec.Command("git", "status", "--porcelain")
		cmdStatus.Dir = workspaceDir
		statusOut, _ := cmdStatus.Output()
		if len(strings.TrimSpace(string(statusOut))) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Working tree is dirty. Commit, stash, or force checkout."})
			return
		}
	}

	args := []string{"checkout"}
	if req.Force {
		args = append(args, "-f")
	}
	args = append(args, req.Ref)

	cmd := exec.Command("git", args...)
	cmd.Dir = workspaceDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": string(out)})
		return
	}

	// Verify the actual checked out branch
	cmdVerify := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmdVerify.Dir = workspaceDir
	verifyOut, _ := cmdVerify.Output()
	actualBranch := strings.TrimSpace(string(verifyOut))

	// Update DB
	env.GithubBranch = actualBranch
	db.DB.Save(env)

	TouchEnvironmentActivity(id)

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Checked out %s", req.Ref), "branch": actualBranch})
}

func GitPull(c *gin.Context) {
	id := c.Param("id")
	_, err := checkWorkspaceWriteAccess(c, id)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	wd, _ := os.Getwd()
	workspaceDir := filepath.Join(wd, "workspaces", id)

	cmd := exec.Command("git", "pull")
	cmd.Dir = workspaceDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": string(out)})
		return
	}

	TouchEnvironmentActivity(id)

	c.JSON(http.StatusOK, gin.H{"message": "Successfully pulled", "output": string(out)})
}

// GitLog returns the last 20 commits for an environment.
func GitLog(c *gin.Context) {
	id := c.Param("id")
	_, err := checkWorkspaceAccess(c, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	wd, _ := os.Getwd()
	workspaceDir := filepath.Join(wd, "workspaces", id)

	if _, err := os.Stat(workspaceDir); os.IsNotExist(err) {
		c.JSON(http.StatusOK, gin.H{"commits": []interface{}{}})
		return
	}

	cmd := exec.Command("git", "log", "-n", "20", "--format=%H|%h|%s|%an|%aI")
	cmd.Dir = workspaceDir
	out, err := cmd.Output()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"commits": []interface{}{}})
		return
	}

	type Commit struct {
		Hash      string `json:"hash"`
		ShortHash string `json:"shortHash"`
		Message   string `json:"message"`
		Author    string `json:"author"`
		Date      string `json:"date"`
	}

	var commits []Commit
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}
		commits = append(commits, Commit{
			Hash:      parts[0],
			ShortHash: parts[1],
			Message:   parts[2],
			Author:    parts[3],
			Date:      parts[4],
		})
	}

	c.JSON(http.StatusOK, gin.H{"commits": commits})
}


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

	envIDs := []string{}
	for _, env := range envs {
		envIDs = append(envIDs, env.ID)
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

	// Fetch from Activity table
	if len(envIDs) > 0 {
		var dbActivities []models.Activity
		db.DB.Preload("User").Where("environment_id IN ?", envIDs).Order("created_at DESC").Limit(100).Find(&dbActivities)

		for _, dbAct := range dbActivities {
			var data map[string]interface{}
			json.Unmarshal([]byte(dbAct.Data), &data)

			actorName := "System"
			if dbAct.UserID != nil && dbAct.User.Username != "" {
				actorName = dbAct.User.Username
			} else if name, ok := data["user_name"].(string); ok && name != "" {
				actorName = name
			}

			action := dbAct.Type
			if act, ok := data["action"].(string); ok && act != "" {
				action = act
			}

			message := dbAct.Type
			if filePath, ok := data["file_path"].(string); ok {
				message = action + " " + filePath
			}

			hash := dbAct.ID
			if len(hash) > 8 {
				hash = hash[:8]
			}

			// Skip noisy file watcher events from being shown as Team Activity
			if actorName == "System/External" {
				continue
			}

			activityMap[dbAct.ID] = GitActivity{
				Timestamp:     dbAct.CreatedAt,
				TimestampStr:  dbAct.CreatedAt.Format(time.RFC3339),
				ActorName:     actorName,
				ActorEmail:    dbAct.User.Email,
				Action:        action,
				Message:       message,
				Hash:          hash,
				EnvironmentID: getEnvId(dbAct.EnvironmentID),
				Branch:        "live",
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

		// Check for actual merge conflicts using git diff
		cmdConflict := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
		cmdConflict.Dir = workspaceDir
		conflictOut, _ := cmdConflict.Output()
		conflictFiles := strings.Split(strings.TrimSpace(string(conflictOut)), "\n")
		hasConflict := len(conflictFiles) > 0 && conflictFiles[0] != ""

		status := "ready_to_merge"
		if hasConflict {
			status = "conflict"
		} else if hasUncommitted {
			status = "in_progress"
		} else if env.Status == models.StatusBuilding {
			status = "building"
		}

		branchInfo := map[string]interface{}{
			"environment_id":   env.ID,
			"environment_name": env.Name,
			"name":             env.GithubBranch,
			"status":           status,
			"latest_commit":    latestCommit,
			"author": map[string]string{
				"id":    env.User.ID,
				"name":  env.User.Email, // Using email as name for mock
				"email": env.User.Email,
			},
			"has_uncommitted": hasUncommitted,
		}
		branches = append(branches, branchInfo)

		// Create blocker alerts
		if hasConflict {
			blockers = append(blockers, map[string]interface{}{
				"type":        "conflict",
				"environment": env.Name,
				"branch":      env.GithubBranch,
				"files":       conflictFiles,
				"resolution":  "Needs manual merge resolution",
			})
		}
		if hasUncommitted && !hasConflict {
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

	env, err := checkWorkspaceWriteAccess(c, envID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
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

	// Add all changes
	cmdAdd := exec.Command("git", "add", "-A")
	cmdAdd.Dir = workspaceDir
	cmdAdd.Run()

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

	TouchEnvironmentActivity(envID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"commit":  hashStr,
		"message": req.Message,
	})
}

func getEnvId(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func PushChanges(c *gin.Context) {
	id := c.Param("id")
	env, err := checkWorkspaceWriteAccess(c, id)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("userId")
	userIDStr := userID.(string)
	var user models.User
	if err := db.DB.First(&user, "id = ?", userIDStr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
		return
	}

	githubToken := ""
	if user.GithubToken != "" {
		decrypted, err := Decrypt(user.GithubToken)
		if err == nil {
			githubToken = decrypted
		}
	}

	if githubToken == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "GitHub token not found. Please re-authenticate with GitHub."})
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get working directory"})
		return
	}
	workspaceDir := filepath.Join(wd, "workspaces", env.ID)

	// Ensure token is configured locally in git
	cmdConfig := exec.Command("git", "config", "--local", "url.https://x-access-token:"+githubToken+"@github.com/.insteadOf", "https://github.com/")
	cmdConfig.Dir = workspaceDir
	cmdConfig.Run()

	// Push the current branch (HEAD) and set upstream
	cmdPush := exec.Command("git", "push", "-u", "origin", "HEAD")
	cmdPush.Dir = workspaceDir
	pushOut, err := cmdPush.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Push failed. You may need to sync (pull) latest changes first.",
			"details": string(pushOut),
		})
		return
	}

	TouchEnvironmentActivity(env.ID)

	c.JSON(http.StatusOK, gin.H{"message": "Successfully pushed to GitHub"})
}
