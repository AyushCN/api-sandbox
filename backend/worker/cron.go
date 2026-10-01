package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/api-sandbox/backend/provider"
	"github.com/hibiken/asynq"
)

func HandleCleanupContainersTask(ctx context.Context, t *asynq.Task) error {
	var envs []models.Environment

	idleHoursStr := os.Getenv("IDLE_TIMEOUT_HOURS")
	idleHours := 6
	if h, err := strconv.Atoi(idleHoursStr); err == nil && h > 0 {
		idleHours = h
	}

	idleThreshold := time.Now().Add(-1 * time.Duration(idleHours) * time.Hour)

	// Find environments that are RUNNING and idle past threshold (checking last_activity_at first)
	if err := db.DB.Where("status = ? AND COALESCE(last_activity_at, updated_at) < ?", models.StatusRunning, idleThreshold).Find(&envs).Error; err != nil {
		return err
	}

	for _, env := range envs {
		slog.Info("Cron: Cleaning up expired environment", "env_id", env.ID)

		// 1. Stop main container
		if env.ContainerID != nil && *env.ContainerID != "" {
			_ = provider.CleanupContainer(ctx, *env.ContainerID)
		} else {
			// Fallback cleanup by name
			_ = provider.CleanupContainer(ctx, fmt.Sprintf("api-sandbox-env-%s", env.ID))
		}

		// 2. Stop DB sidecar
		_ = provider.CleanupContainer(ctx, fmt.Sprintf("api-sandbox-db-%s", env.ID))

		// 3. Cleanup Workspace on disk
		_ = provider.CleanupWorkspace(env.ID)

		// 4. Update status
		db.DB.Model(&env).Updates(map[string]interface{}{
			"status":     models.StatusStopped,
			"public_url": nil,
		})

		db.DB.Create(&models.Log{
			EnvironmentID: &env.ID,
			Message:       fmt.Sprintf("System: Environment stopped automatically after %d hours of inactivity.", idleHours),
			Level:         models.LogLevelWarn,
		})
	}

	return nil
}

func HandleReapOrphansTask(ctx context.Context, t *asynq.Task) error {
	slog.Info("Cron: Running orphan reaper to clean up stale containers and workspaces on the host.")
	if err := provider.ReapOrphanContainers(ctx); err != nil {
		slog.Error("ReapOrphanContainers failed", "error", err)
		return err
	}
	return nil
}
