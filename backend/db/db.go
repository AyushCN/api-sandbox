package db

import (
	"log/slog"
	"os"
	"time"

	"github.com/api-sandbox/backend/models"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	DB          *gorm.DB
	RedisClient *redis.Client
)

func InitDB() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgresql://postgres:postgres@localhost:5432/api_sandbox?sslmode=disable"
	}

	var err error
	for i := 1; i <= 10; i++ {
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err == nil {
			break
		}
		slog.Warn("Database not ready yet", "attempt", i, "total_attempts", 10, "error", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		slog.Error("Failed to connect to database after 10 attempts", "error", err)
		os.Exit(1)
	}

	// Set up connection pool
	sqlDB, err := DB.DB()
	if err == nil {
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetMaxIdleConns(25)
		sqlDB.SetConnMaxLifetime(time.Minute * 5)
	}

	// Auto Migrate the schemas
	err = DB.AutoMigrate(&models.User{}, &models.Organization{}, &models.OrganizationMember{}, &models.Environment{}, &models.Log{}, &models.Metric{}, &models.AuditLog{})
	if err != nil {
		slog.Error("Failed to auto migrate database schemas", "error", err)
		os.Exit(1)
	}

	slog.Info("Database connection established and schemas migrated.")

	redisUrl := os.Getenv("REDIS_URL")
	if redisUrl == "" {
		redisUrl = "redis://localhost:6379"
	}
	
	opt, err := redis.ParseURL(redisUrl)
	if err != nil {
		slog.Error("Failed to parse Redis URI", "redis_url", redisUrl, "error", err)
		os.Exit(1)
	}

	RedisClient = redis.NewClient(opt)
}
