package api

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

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

type GitBranchRequest struct {
	Branch string `json:"branch" binding:"required"`
}

func GitBranch(c *gin.Context) {
	id := c.Param("id")
	_, err := checkWorkspaceAccess(c, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
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

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Created and checked out branch %s", req.Branch)})
}

type GitCheckoutRequest struct {
	Ref   string `json:"ref" binding:"required"`
	Force bool   `json:"force"`
}

func GitCheckout(c *gin.Context) {
	id := c.Param("id")
	_, err := checkWorkspaceAccess(c, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
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

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Checked out %s", req.Ref)})
}

func GitPull(c *gin.Context) {
	id := c.Param("id")
	_, err := checkWorkspaceAccess(c, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
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

	c.JSON(http.StatusOK, gin.H{"message": "Successfully pulled", "output": string(out)})
}
