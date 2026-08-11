package queue

import (
	"log/slog"
	"os"

	"github.com/hibiken/asynq"
)

var Client *asynq.Client

const (
	TaskBuildEnvironment  = "environment:build"
	TaskDeploy            = "deployment:deploy"
	TaskCollectMetrics    = "system:metrics"
	TaskCleanupContainers = "system:cleanup"
)

func InitQueue() {
	redisUrl := os.Getenv("REDIS_URL")
	if redisUrl == "" {
		redisUrl = "redis://localhost:6379"
	}

	opt, err := asynq.ParseRedisURI(redisUrl)
	if err != nil {
		slog.Error("Failed to parse Redis URI in InitQueue", "redis_url", redisUrl, "error", err)
		os.Exit(1)
	}

	Client = asynq.NewClient(opt)
	slog.Info("Asynq client initialized.")
}
