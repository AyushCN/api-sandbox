package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
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
	devConfig, err := provider.ResolveRuntime(&env, workspaceDir, "")
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

	db.DB.Create(&models.Log{
		EnvironmentID: &env.ID,
		Message:       fmt.Sprintf("Detected %s environment. Generating sandbox-start.sh...", devConfig.BaseImage),
		Level:         models.LogLevelInfo,
	})

	scriptContent := provider.GenerateSandboxStartScript(devConfig)
	scriptPath := filepath.Join(workspaceDir, "sandbox-start.sh")
	err = os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		slog.Error("Failed to write sandbox-start.sh", "env_id", envID, "error", err)
		return err
	}

	// 4. Start Dev Sandbox
	netID := env.OrganizationID
	if netID == "" {
		netID = env.UserID
	}

	containerID, port, err := provider.ProvisionDevSandbox(ctx, env.ID, devConfig, netID, dbURL)
	if err != nil {
		slog.Error("Dev Sandbox start failed", "env_id", envID, "error", err)
		db.DB.Model(&env).Update("status", models.StatusFailed)
		db.DB.Create(&models.Log{
			EnvironmentID: &env.ID,
			Message:       fmt.Sprintf("Sandbox start failed: %v", err),
			Level:         models.LogLevelError,
		})
		return err
	}

	// 5. Container Health Check on Boot
	// Poll until the container is confirmed running OR exits.
	// pip install / npm install / go mod download can take 60-120s on a cold image.
	db.DB.Create(&models.Log{
		EnvironmentID: &env.ID,
		Message:       "Waiting for sandbox to initialize (dependency install + app boot)...",
		Level:         models.LogLevelInfo,
	})

	bootTimeout := 120 * time.Second
	pollInterval := 5 * time.Second
	deadline := time.Now().Add(bootTimeout)
	isRunning := false
	var crashLogs string

	healthType := "tcp"
	if env.HealthCheckType != nil {
		healthType = *env.HealthCheckType
	}

	// StartCommand port=0 heuristic
	if env.StartCommand != nil && *env.StartCommand != "" && (env.Port == nil || *env.Port == 0) {
		healthType = "none"
	}

	domain := os.Getenv("DOMAIN")
	if domain == "" {
		domain = "localhost"
	}

	for time.Now().Before(deadline) {
		var checkErr error
		isRunning, crashLogs, checkErr = ProviderCheckContainerHealth(containerID)
		if checkErr != nil {
			slog.Warn("Health check error during boot polling", "env_id", envID, "error", checkErr)
			break
		}

		if !isRunning {
			// Container has already exited
			break
		}

		if healthType == "none" {
			// Skip HTTP/TCP port check
			break
		}

		// Traefik HTTP Check (Ensures port is bound and accepting traffic)
		req, _ := http.NewRequest("GET", "http://api-sandbox-traefik", nil)
		req.Host = fmt.Sprintf("%s.%s", envID, domain)

		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Do(req)

		if err == nil {
			// 502 Bad Gateway means Traefik can't reach the container port yet
			if resp.StatusCode != http.StatusBadGateway {
				resp.Body.Close()
				break // Application is responding!
			}
			resp.Body.Close()
		}

		time.Sleep(pollInterval)
	}

	// If still not running after timeout, poll one last time
	if !isRunning && time.Now().After(deadline) {
		isRunning, crashLogs, _ = ProviderCheckContainerHealth(containerID)
	}

	if !isRunning {
		slog.Error("Dev Sandbox failed to start", "env_id", envID)
		db.DB.Model(&env).Update("status", models.StatusFailed)
		msg := "Sandbox failed to start"
		if crashLogs != "" {
			msg = fmt.Sprintf("Sandbox crashed on boot:\n%s", crashLogs)
		} else if healthType == "tcp" {
			msg = "Sandbox failed to start: No process listening on configured port before timeout. Check your port settings or disable the health check."
		}
		db.DB.Create(&models.Log{
			EnvironmentID: &env.ID,
			Message:       msg,
			Level:         models.LogLevelError,
		})
		return fmt.Errorf("container crashed or failed healthcheck on boot")
	}

	_ = pollInterval // used in future polling refinement

	// 6. Update DB to RUNNING
	protocol := "https"
	if domain == "localhost" {
		protocol = "http"
	}

	publicURL := fmt.Sprintf("%s://%s.%s", protocol, env.ID, domain)
	db.DB.Model(&env).Updates(map[string]interface{}{
		"status":       models.StatusRunning,
		"container_id": containerID,
		"port":         port,
		"public_url":   publicURL,
	})

	slog.Info("Environment is now RUNNING", "env_id", env.ID, "port", port)
	return nil
}
