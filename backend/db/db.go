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
	err = DB.AutoMigrate(
		&models.User{},
		&models.Organization{},
		&models.OrganizationMember{},
		&models.Project{},
		&models.ProjectCollaborator{},
		&models.Environment{},
		&models.EnvironmentMember{},
		&models.Log{},
		&models.Metric{},
		&models.Deployment{},
		&models.ProcessType{},
		&models.Addon{},
		&models.ProviderConfig{},
		&models.AuditLog{},
		&models.Activity{},
		&models.BenchmarkRun{},
	)
	if err != nil {
		slog.Error("Failed to auto migrate database schemas", "error", err)
		os.Exit(1)
	}

	// Migration: Create default projects for environments that only have OrganizationID
	var envsWithoutProject []models.Environment
	DB.Where("project_id = '' OR project_id IS NULL").Find(&envsWithoutProject)

	if len(envsWithoutProject) > 0 {
		slog.Info("Migrating existing environments to Project model...", "count", len(envsWithoutProject))
		for _, env := range envsWithoutProject {
			if env.OrganizationID == "" {
				continue // Can't migrate without org
			}

			// Find or create a default project for this org
			var project models.Project
			err := DB.Where("owner_organization_id = ? AND name = ?", env.OrganizationID, "Default Workspace").First(&project).Error

			if err != nil {
				// Create the project
				project = models.Project{
					Name:                "Default Workspace",
					Description:         "Auto-migrated default project",
					OwnerOrganizationID: env.OrganizationID,
					CreatedByUserID:     env.UserID,
				}
				DB.Create(&project)

				// Add the creator as OWNER
				DB.Create(&models.ProjectCollaborator{
					ProjectID:       project.ID,
					UserID:          env.UserID,
					Role:            models.ProjectRoleOwner,
					InvitedByUserID: env.UserID,
				})
			}

			// Assign environment to project
			env.ProjectID = project.ID
			DB.Save(&env)
		}
		slog.Info("Migration complete.")
	}

	// Migration: Auto-accept all existing collaborators so users don't lose access
	err = DB.Exec("UPDATE project_collaborators SET accepted_at = CURRENT_TIMESTAMP WHERE accepted_at IS NULL").Error
	if err != nil {
		slog.Error("Failed to auto-accept existing collaborators", "error", err)
	} else {
		slog.Info("Migrated existing collaborators to accepted status.")
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
