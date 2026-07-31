package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/hibiken/asynq"
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

	var env models.Environment
	if err := db.DB.First(&env, "id = ?", envID).Error; err != nil {
		return fmt.Errorf("environment not found: %w", err)
	}

	// Idempotency: cleanup existing container if retrying
	if env.ContainerID != nil && *env.ContainerID != "" {
		slog.Info("Cleaning up existing container", "container_id", *env.ContainerID)
		_ = CleanupContainer(ctx, *env.ContainerID)
	}

	// 1. Clone & Build
	imageTag, err := CloneAndBuildImage(ctx, env.ID, env.GitURL, env.GithubBranch)
	if err != nil {
		slog.Error("Build failed", "env_id", envID, "error", err)
		db.DB.Model(&env).Update("status", models.StatusFailed)
		db.DB.Create(&models.Log{
			EnvironmentID: env.ID,
			Message:       fmt.Sprintf("Build failed: %v", err),
			Level:         models.LogLevelError,
		})
		return err
	}

	// 2. Start Container
	netID := env.OrganizationID
	if netID == "" {
		netID = env.UserID
	}
	containerID, port, err := StartContainer(ctx, env.ID, imageTag, netID)
	if err != nil {
		slog.Error("Start failed", "env_id", envID, "error", err)
		db.DB.Model(&env).Update("status", models.StatusFailed)
		db.DB.Create(&models.Log{
			EnvironmentID: env.ID,
			Message:       fmt.Sprintf("Container start failed: %v", err),
			Level:         models.LogLevelError,
		})
		return err
	}

	// 3. Update DB to RUNNING
	domain := os.Getenv("DOMAIN")
	if domain == "" {
		domain = "localhost"
	}
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
