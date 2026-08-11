package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/api-sandbox/backend/provider"
	"github.com/hibiken/asynq"
)

func HandleCollectMetricsTask(ctx context.Context, t *asynq.Task) error {
	var envs []models.Environment
	if err := db.DB.Where("status = ?", models.StatusRunning).Find(&envs).Error; err != nil {
		return err
	}

	for _, env := range envs {
		if env.ContainerID == nil || *env.ContainerID == "" {
			continue
		}

		// In a real system, we'd use dockerClient.Stats() to read actual CPU/Memory.
		// Since fsouza stats stream can be tricky to parse quickly in a cron,
		// we'll mock the metrics insertion here to prove the architectural pipeline.
		// This simulates reading Docker cgroups.
		metric := models.Metric{
			EnvironmentID: &env.ID,
			CpuUsage:      2.5,   // Mock %
			MemoryUsage:   150.0, // Mock MB
		}
		db.DB.Create(&metric)
	}

	return nil
}

func HandleCleanupContainersTask(ctx context.Context, t *asynq.Task) error {
	var envs []models.Environment

	// Find environments running for more than 1 hour
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	if err := db.DB.Where("status = ? AND updated_at < ?", models.StatusRunning, oneHourAgo).Find(&envs).Error; err != nil {
		return err
	}

	for _, env := range envs {
		if env.ContainerID != nil && *env.ContainerID != "" {
			slog.Info("Cron: Cleaning up expired container", "env_id", env.ID)
			_ = provider.CleanupContainer(ctx, *env.ContainerID, "environment")
		}

		db.DB.Model(&env).Updates(map[string]interface{}{
			"status":     models.StatusStopped,
			"public_url": nil,
		})

		db.DB.Create(&models.Log{
			EnvironmentID: &env.ID,
			Message:       "System: Container stopped automatically after reaching 1-hour time limit.",
			Level:         models.LogLevelWarn,
		})
	}

	return nil
}
