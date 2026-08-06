package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

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

	log.Printf("Processing build job for environment %s", envID)

	var env models.Environment
	if err := db.DB.First(&env, "id = ?", envID).Error; err != nil {
		return fmt.Errorf("environment not found: %w", err)
	}

	// Idempotency: cleanup existing container if retrying
	if env.ContainerID != nil && *env.ContainerID != "" {
		log.Printf("Cleaning up existing container %s for retrying...", *env.ContainerID)
		_ = CleanupContainer(ctx, *env.ContainerID)
	}

	// 1. Clone & Build
	imageTag, err := CloneAndBuildImage(ctx, env.ID, env.GitURL, env.GithubBranch)
	if err != nil {
		log.Printf("Build failed: %v", err)
		db.DB.Model(&env).Update("status", models.StatusFailed)
		db.DB.Create(&models.Log{
			EnvironmentID: env.ID,
			Message:       fmt.Sprintf("Build failed: %v", err),
			Level:         models.LogLevelError,
		})
		return err
	}

	// 2. Start Container
	containerID, port, err := StartContainer(ctx, env.ID, imageTag)
	if err != nil {
		log.Printf("Start failed: %v", err)
		db.DB.Model(&env).Update("status", models.StatusFailed)
		db.DB.Create(&models.Log{
			EnvironmentID: env.ID,
			Message:       fmt.Sprintf("Container start failed: %v", err),
			Level:         models.LogLevelError,
		})
		return err
	}

	// 3. Update DB to RUNNING
	publicURL := fmt.Sprintf("http://%s.localhost", env.ID)
	db.DB.Model(&env).Updates(map[string]interface{}{
		"status":       models.StatusRunning,
		"container_id": containerID,
		"port":         port,
		"public_url":   publicURL,
	})

	log.Printf("Environment %s is now RUNNING on port %d", env.ID, port)
	return nil
}
