package queue

import (
	"log"
	"os"

	"github.com/hibiken/asynq"
)

var Client *asynq.Client

const (
	TaskBuildEnvironment  = "environment:build"
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
		log.Fatalf("Failed to parse Redis URI: %v", err)
	}

	Client = asynq.NewClient(opt)
	log.Println("Asynq client initialized.")
}
