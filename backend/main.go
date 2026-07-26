package main

import (
	"log"
	"os"

	"github.com/api-sandbox/backend/api"
	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/queue"
	"github.com/api-sandbox/backend/worker"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if exists
	_ = godotenv.Load()

	// Initialize singletons
	db.InitDB()
	queue.InitQueue()
	worker.InitDocker()

	mode := os.Getenv("MODE")
	if mode == "worker" {
		runWorker()
	} else if mode == "api" {
		runAPI()
	} else {
		// Run both in dev by default using goroutine for worker
		go runScheduler()
		go runWorker()
		runAPI()
	}
}

func runScheduler() {
	redisUrl := os.Getenv("REDIS_URL")
	if redisUrl == "" {
		redisUrl = "redis://localhost:6379"
	}

	opt, _ := asynq.ParseRedisURI(redisUrl)
	scheduler := asynq.NewScheduler(opt, &asynq.SchedulerOpts{})

	// Register cron jobs
	// Every 30 seconds for metrics
	if _, err := scheduler.Register("@every 30s", asynq.NewTask(queue.TaskCollectMetrics, nil)); err != nil {
		log.Fatalf("Failed to register metrics cron: %v", err)
	}

	// Every 5 minutes for cleanup
	if _, err := scheduler.Register("@every 5m", asynq.NewTask(queue.TaskCleanupContainers, nil)); err != nil {
		log.Fatalf("Failed to register cleanup cron: %v", err)
	}

	log.Println("Starting Asynq scheduler...")
	if err := scheduler.Run(); err != nil {
		log.Fatalf("Could not run Asynq scheduler: %v", err)
	}
}

func runAPI() {
	r := gin.Default()
	api.SetupRoutes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting API server on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to run API server: %v", err)
	}
}

func runWorker() {
	redisUrl := os.Getenv("REDIS_URL")
	if redisUrl == "" {
		redisUrl = "redis://localhost:6379"
	}

	opt, _ := asynq.ParseRedisURI(redisUrl)
	srv := asynq.NewServer(
		opt,
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"default": 1,
			},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TaskBuildEnvironment, worker.HandleBuildEnvironmentTask)
	mux.HandleFunc(queue.TaskCollectMetrics, worker.HandleCollectMetricsTask)
	mux.HandleFunc(queue.TaskCleanupContainers, worker.HandleCleanupContainersTask)

	log.Println("Starting Asynq worker...")
	if err := srv.Run(mux); err != nil {
		log.Fatalf("Could not run Asynq server: %v", err)
	}
}
