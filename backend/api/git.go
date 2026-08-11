package api

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
