package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/api-sandbox/backend/api"
	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/api-sandbox/backend/provider"
	"github.com/api-sandbox/backend/queue"
	"github.com/api-sandbox/backend/worker"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
)

func main() {
	// Initialize structured JSON logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Load .env file if exists
	_ = godotenv.Load()

	// Initialize singletons
	db.InitDB()

	// Recover environments stuck in BUILDING from previous runs
	var stuckEnvs []models.Environment
	if err := db.DB.Where("status = ?", "BUILDING").Find(&stuckEnvs).Error; err == nil && len(stuckEnvs) > 0 {
		for _, env := range stuckEnvs {
			db.DB.Create(&models.Log{
				EnvironmentID: &env.ID,
				Message:       "Build failed: build process was interrupted due to server shutdown/restart.",
				Level:         models.LogLevelError,
			})
			db.DB.Model(&env).Update("status", "FAILED")
			slog.Info("Recovered stuck environment status to FAILED", "env_id", env.ID)
		}
	}

	queue.InitQueue()
	provider.InitDocker()

	// Start WebSocket Hub and File Watcher
	go api.WSHub.Run()
	go api.WatchAllEnvironments()

	mode := os.Getenv("MODE")

	var httpServer *http.Server
	var asynqServer *asynq.Server
	var asynqScheduler *asynq.Scheduler

	if mode == "" || mode == "worker" {
		asynqServer = startWorker()
		if mode == "" {
			asynqScheduler = startScheduler()
		}
	}

	if mode == "" || mode == "api" {
		httpServer = startAPI()
	}

	// Wait for interrupt signal to gracefully shut down the servers
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down gracefully...")

	// The context is used to inform the server it has 30 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if httpServer != nil {
		if err := httpServer.Shutdown(ctx); err != nil {
			slog.Error("API Server forced to shutdown", "error", err)
			os.Exit(1)
		}
		slog.Info("API Server exiting")
	}

	if asynqScheduler != nil {
		asynqScheduler.Shutdown()
		slog.Info("Asynq Scheduler exiting")
	}

	if asynqServer != nil {
		asynqServer.Shutdown()
		slog.Info("Asynq Worker exiting")
	}

	slog.Info("Graceful shutdown complete.")
}

func startScheduler() *asynq.Scheduler {
	redisUrl := os.Getenv("REDIS_URL")
	if redisUrl == "" {
		redisUrl = "redis://localhost:6379"
	}

	opt, _ := asynq.ParseRedisURI(redisUrl)
	scheduler := asynq.NewScheduler(opt, &asynq.SchedulerOpts{})

	// Register cron jobs
	if _, err := scheduler.Register("@every 30s", asynq.NewTask(queue.TaskCollectMetrics, nil)); err != nil {
		slog.Error("Failed to register metrics cron", "error", err)
		os.Exit(1)
	}

	if _, err := scheduler.Register("@every 5m", asynq.NewTask(queue.TaskCleanupContainers, nil)); err != nil {
		slog.Error("Failed to register cleanup cron", "error", err)
		os.Exit(1)
	}

	slog.Info("Starting Asynq scheduler...")
	go func() {
		if err := scheduler.Run(); err != nil {
			slog.Error("Asynq scheduler error", "error", err)
		}
	}()
	return scheduler
}

func startAPI() *http.Server {
	r := gin.Default()
	api.SetupRoutes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	slog.Info("Starting API server", "port", port)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Failed to run API server", "error", err)
			os.Exit(1)
		}
	}()
	return srv
}

func startWorker() *asynq.Server {
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
	mux.HandleFunc(queue.TaskDeploy, worker.HandleDeployTask)
	mux.HandleFunc(queue.TaskCollectMetrics, worker.HandleCollectMetricsTask)
	mux.HandleFunc(queue.TaskCleanupContainers, worker.HandleCleanupContainersTask)

	slog.Info("Starting Asynq worker...")
	go func() {
		if err := srv.Start(mux); err != nil {
			slog.Error("Could not run Asynq server", "error", err)
			os.Exit(1)
		}
	}()
	return srv
}
