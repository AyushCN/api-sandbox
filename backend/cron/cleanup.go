package cron

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/api-sandbox/backend/models"
	"github.com/api-sandbox/backend/provider"
	"gorm.io/gorm"
)

// StartIdleCleanup runs a background job that periodically sweeps the database
// for RUNNING environments that have been idle for too long and stops them.
func StartIdleCleanup(db *gorm.DB) {
	// Configurable timeout, defaulting to 6 hours for research prototype
	timeoutStr := os.Getenv("IDLE_TIMEOUT_HOURS")
	timeoutHours, err := strconv.Atoi(timeoutStr)
	if err != nil || timeoutHours <= 0 {
		timeoutHours = 6
	}

	idleDuration := time.Duration(timeoutHours) * time.Hour
	slog.Info("Starting Idle Cleanup Cron", "interval_hours", 1, "idle_timeout_hours", timeoutHours)

	ticker := time.NewTicker(1 * time.Hour)

	go func() {
		for range ticker.C {
			cleanupIdleEnvironments(db, idleDuration)
		}
	}()
}

func cleanupIdleEnvironments(db *gorm.DB, idleDuration time.Duration) {
	cutoffTime := time.Now().Add(-idleDuration)
	var idleEnvs []models.Environment

	// Find environments that are RUNNING and haven't had activity since cutoffTime
	if err := db.Where("status = ? AND last_activity_at < ?", models.StatusRunning, cutoffTime).Find(&idleEnvs).Error; err != nil {
		slog.Error("Failed to query idle environments", "error", err)
		return
	}

	if len(idleEnvs) == 0 {
		return
	}

	slog.Info("Found idle environments to clean up", "count", len(idleEnvs), "cutoff_time", cutoffTime)

	ctx := context.Background()
	for _, env := range idleEnvs {
		slog.Info("IDLE_STOP: Stopping idle environment", "env_id", env.ID, "last_activity", env.LastActivityAt)

		// 1. Cleanup App Container
		if env.ContainerID != nil && *env.ContainerID != "" {
			_ = provider.CleanupContainer(ctx, *env.ContainerID)
		} else {
			_ = provider.CleanupContainer(ctx, fmt.Sprintf("api-sandbox-env-%s", env.ID))
		}

		// 2. Cleanup Database Sidecar Container
		_ = provider.CleanupContainer(ctx, fmt.Sprintf("api-sandbox-db-%s", env.ID))

		// 3. Mark as STOPPED
		if err := db.Model(&env).Update("status", models.StatusStopped).Error; err != nil {
			slog.Error("Failed to update status to STOPPED for idle environment", "env_id", env.ID, "error", err)
		}
	}
}
