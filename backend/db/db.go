package db

import (
	"log"
	"os"
	"time"

	"github.com/api-sandbox/backend/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"github.com/redis/go-redis/v9"
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
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Set up connection pool
	sqlDB, err := DB.DB()
	if err == nil {
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetMaxIdleConns(25)
		sqlDB.SetConnMaxLifetime(time.Minute * 5)
	}

	// Auto Migrate the schemas
	err = DB.AutoMigrate(&models.User{}, &models.Environment{}, &models.Log{}, &models.Metric{})
	if err != nil {
		log.Fatalf("Failed to auto migrate database schemas: %v", err)
	}

	log.Println("Database connection established and schemas migrated.")

	redisUrl := os.Getenv("REDIS_URL")
	if redisUrl == "" {
		redisUrl = "redis://localhost:6379"
	}
	
	opt, err := redis.ParseURL(redisUrl)
	if err != nil {
		log.Fatalf("Failed to parse Redis URI: %v", err)
	}

	RedisClient = redis.NewClient(opt)
}
