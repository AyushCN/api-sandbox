package api

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// HealthCheck verifies connectivity to required external services (DB, Redis)
func HealthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	status := gin.H{"status": "ok"}
	statusCode := http.StatusOK

	// Check DB
	sqlDB, err := db.DB.DB()
	if err != nil {
		status["db"] = "error"
		status["db_error"] = err.Error()
		statusCode = http.StatusServiceUnavailable
	} else if err := sqlDB.Ping(); err != nil {
		status["db"] = "error"
		status["db_error"] = err.Error()
		statusCode = http.StatusServiceUnavailable
	} else {
		status["db"] = "ok"
	}

	// Check Redis
	redisUrl := os.Getenv("REDIS_URL")
	if redisUrl == "" {
		redisUrl = "redis://localhost:6379"
	}
	opts, err := redis.ParseURL(redisUrl)
	if err != nil {
		status["redis"] = "error"
		status["redis_error"] = err.Error()
		statusCode = http.StatusServiceUnavailable
	} else {
		rdb := redis.NewClient(opts)
		defer rdb.Close()
		if err := rdb.Ping(ctx).Err(); err != nil {
			status["redis"] = "error"
			status["redis_error"] = err.Error()
			statusCode = http.StatusServiceUnavailable
		} else {
			status["redis"] = "ok"
		}
	}

	if statusCode != http.StatusOK {
		status["status"] = "error"
	}

	c.JSON(statusCode, status)
}
