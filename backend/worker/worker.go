package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/api-sandbox/backend/api"
	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/api-sandbox/backend/provider"
	"github.com/hibiken/asynq"
)

var (
	ProviderCleanupContainer           = provider.CleanupContainer
	ProviderCloneOrFetch               = provider.CloneOrFetch
	ProviderDetectDatabaseRequirements = provider.DetectDatabaseRequirements
	ProviderStartSidecarDatabase       = provider.StartSidecarDatabase
	ProviderCheckContainerHealth       = provider.CheckContainerHealth
)

func HandleBuildEnvironmentTask(ctx context.Context, t *asynq.Task) error {
	var payload map[string]string
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	envID, ok := payload["environmentId"]
	if !ok {
		return fmt.Errorf("missing environmentId: %w", asynq.SkipRetry)
	}

	slog.Info("Processing build job", "environment_id", envID)

	retryCount, _ := asynq.GetRetryCount(ctx)
	maxRetry, _ := asynq.GetMaxRetry(ctx)
	if retryCount > 0 {
		db.DB.Create(&models.Log{
			EnvironmentID: &envID,
			Message:       fmt.Sprintf("⚠️ Attempt %d of %d: Retrying failed build job...", retryCount+1, maxRetry+1),
			Level:         models.LogLevelInfo,
		})
	}

	var env models.Environment
	if err := db.DB.First(&env, "id = ?", envID).Error; err != nil {
		return fmt.Errorf("environment not found: %w", err)
	}

	var user models.User
	if err := db.DB.First(&user, "id = ?", env.UserID).Error; err != nil {
		slog.Warn("User not found for environment", "user_id", env.UserID)
	}

	// Try to get token, decrypt it
	githubToken := ""
	if user.GithubToken != "" {
		decrypted, err := api.Decrypt(user.GithubToken)
		if err == nil {
			githubToken = decrypted
		} else {
			slog.Error("Failed to decrypt github token", "error", err)
		}
	}

	// Idempotency: cleanup existing container if retrying
	if env.ContainerID != nil && *env.ContainerID != "" {
		slog.Info("Cleaning up existing container", "container_id", *env.ContainerID)
		_ = provider.CleanupContainer(ctx, *env.ContainerID)
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %v", err)
	}
	workspaceDir := filepath.Join(wd, "workspaces", env.ID)

	// 1. Clone Repo directly
	db.DB.Create(&models.Log{
		EnvironmentID: &env.ID,
		Message:       fmt.Sprintf("Synchronizing repository %s (branch: %s)...", env.GitURL, env.GithubBranch),
		Level:         models.LogLevelInfo,
	})

	err = ProviderCloneOrFetch(ctx, workspaceDir, env.GitURL, env.GithubBranch, githubToken)
	if err != nil {
		slog.Error("Clone failed", "env_id", envID, "error", err)
		db.DB.Model(&env).Update("status", models.StatusFailed)
		db.DB.Create(&models.Log{
			EnvironmentID: &env.ID,
			Message:       fmt.Sprintf("Git clone failed: %v", err),
			Level:         models.LogLevelError,
		})
		return err
	}

	// Update WorkspaceRepository with BaseCommit and CurrentCommit
	var workspace models.Workspace
	if err := db.DB.Where("environment_id = ?", env.ID).First(&workspace).Error; err == nil {
		// Get commit hash
		cmdHash := exec.Command("git", "rev-parse", "HEAD")
		cmdHash.Dir = workspaceDir
		if hashOut, err := cmdHash.Output(); err == nil {
			hashStr := strings.TrimSpace(string(hashOut))
			db.DB.Model(&models.WorkspaceRepository{}).
				Where("workspace_id = ?", workspace.ID).
				Updates(map[string]interface{}{
					"base_commit":    hashStr,
					"current_commit": hashStr,
				})
			db.DB.Model(&env).Update("commit_hash", hashStr)
		}
	}

	// 2. Database Provisioning
	var dbURL string
	if env.UserProvidedDBURL != nil && *env.UserProvidedDBURL != "" {
		dbURL = *env.UserProvidedDBURL
		db.DB.Create(&models.Log{
			EnvironmentID: &env.ID,
			Message:       "Using user-provided DATABASE_URL",
			Level:         models.LogLevelInfo,
		})
	} else {
		dbType, _ := provider.DetectDatabaseRequirements(workspaceDir)
		if dbType != provider.DBTypeNone {
			db.DB.Create(&models.Log{
				EnvironmentID: &env.ID,
				Message:       fmt.Sprintf("Auto-detected database requirement: %s", string(dbType)),
				Level:         models.LogLevelInfo,
			})

			netID := env.OrganizationID
			if netID == "" {
				netID = env.UserID
			}

			url, err := provider.StartSidecarDatabase(ctx, env.ID, netID, dbType)
			if err != nil {
				slog.Error("Failed to start sidecar db", "env_id", envID, "error", err)
				db.DB.Create(&models.Log{
					EnvironmentID: &env.ID,
					Message:       fmt.Sprintf("Failed to provision database: %v", err),
					Level:         models.LogLevelError,
				})
				db.DB.Model(&env).Update("status", models.StatusFailed)
				return err
			} else {
				dbURL = url
			}
		}
	}

	// 3. Detect Runtime or Use Overrides
	configs, err := provider.ResolveRuntimes(&env, workspaceDir, "")
	if err != nil {
		slog.Error("Failed to detect Dev Runtime", "env_id", envID, "error", err)
		db.DB.Model(&env).Update("status", models.StatusFailed)
		db.DB.Create(&models.Log{
			EnvironmentID: &env.ID,
			Message:       fmt.Sprintf("Runtime detection failed: %v", err),
			Level:         models.LogLevelError,
		})
		return err
	}

	netID := env.OrganizationID
	if netID == "" {
		netID = env.UserID
	}

	domain := os.Getenv("DOMAIN")
	if domain == "" {
		domain = "localhost"
	}

	var finalContainerID string
	var finalPort int
	var finalCrashLogs string
	var isRunning bool
	var checkErr error

	maxAttempts := 3
	if len(configs) < maxAttempts {
		maxAttempts = len(configs)
	}

	var devConfig provider.DevRuntimeConfig

	for i := 0; i < maxAttempts; i++ {
		devConfig = configs[i]

		db.DB.Create(&models.Log{
			EnvironmentID: &env.ID,
			Message:       fmt.Sprintf("Attempting to boot using %s runtime (Attempt %d/%d)...", devConfig.BaseImage, i+1, maxAttempts),
			Level:         models.LogLevelInfo,
		})

		scriptContent := provider.GenerateSandboxStartScript(devConfig)
		scriptPath := filepath.Join(workspaceDir, "sandbox-start.sh")
		_ = os.WriteFile(scriptPath, []byte(scriptContent), 0755)

		containerID, port, provErr := provider.ProvisionDevSandbox(ctx, env.ID, devConfig, netID, dbURL)
		if provErr != nil {
			slog.Error("Dev Sandbox start failed", "env_id", envID, "error", provErr)
			continue
		}

		bootTimeout := 120 * time.Second
		pollInterval := 5 * time.Second
		deadline := time.Now().Add(bootTimeout)
		isRunning = false
		finalCrashLogs = ""

		healthType := "tcp"
		if env.HealthCheckType != nil {
			healthType = *env.HealthCheckType
		}

		if env.StartCommand != nil && *env.StartCommand != "" && (env.Port == nil || *env.Port == 0) {
			healthType = "none"
		}

		for time.Now().Before(deadline) {
			isRunning, finalCrashLogs, checkErr = ProviderCheckContainerHealth(containerID)
			if checkErr != nil || !isRunning {
				break
			}

			if healthType == "none" {
				break
			}

			req, _ := http.NewRequest("GET", "http://api-sandbox-traefik", nil)
			req.Host = fmt.Sprintf("%s.%s", envID, domain)
			client := &http.Client{Timeout: 2 * time.Second}
			resp, httpErr := client.Do(req)

			if httpErr == nil {
				if resp.StatusCode != http.StatusBadGateway {
					resp.Body.Close()
					break // success!
				}
				resp.Body.Close()
			}
			time.Sleep(pollInterval)
		}

		if !isRunning && time.Now().After(deadline) {
			isRunning, finalCrashLogs, _ = ProviderCheckContainerHealth(containerID)
		}

		if isRunning {
			finalContainerID = containerID
			finalPort = port
			break
		}

		// Clean up before fallback
		_ = ProviderCleanupContainer(ctx, containerID)
	}

	if !isRunning {
		slog.Error("Dev Sandbox failed to start after fallbacks", "env_id", envID)
		db.DB.Model(&env).Update("status", models.StatusFailed)

		msg := "Couldn't auto-detect how to run this repo. Please manually configure the runtime and entry command."
		if finalCrashLogs != "" {
			msg = fmt.Sprintf("Last attempt crashed:\n%s\nCouldn't auto-detect how to run this repo.", finalCrashLogs)
		}

		db.DB.Create(&models.Log{
			EnvironmentID: &env.ID,
			Message:       msg,
			Level:         models.LogLevelError,
		})
		return fmt.Errorf("container crashed or failed healthcheck on boot")
	}

	// 6. Update DB to RUNNING
	protocol := "https"
	if domain == "localhost" {
		protocol = "http"
	}

	publicURL := fmt.Sprintf("%s://%s.%s", protocol, env.ID, domain)
	db.DB.Model(&env).Updates(map[string]interface{}{
		"status":       models.StatusRunning,
		"container_id": finalContainerID,
		"port":         finalPort,
		"public_url":   publicURL,
	})

	slog.Info("Environment is now RUNNING", "env_id", env.ID, "port", finalPort)
	return nil
}
