package db

import (
	"log"
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
		log.Printf("Attempt %d/10: Database not ready yet (%v). Retrying in 2 seconds...", i, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to connect to database after 10 attempts: %v", err)
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
