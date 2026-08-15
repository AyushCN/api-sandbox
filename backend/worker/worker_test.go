package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/api-sandbox/backend/provider"
	"github.com/hibiken/asynq"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) {
	var err error
	db.DB, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	err = db.DB.AutoMigrate(
		&models.Environment{},
		&models.Log{},
	)
	if err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
}

func TestHandleBuildEnvironmentTask_CloneError(t *testing.T) {
	setupTestDB(t)

	env := models.Environment{
		Name:   "Clone Error Env",
		Status: models.StatusBuilding,
	}
	db.DB.Create(&env)

	// Mock ProviderCloneOrFetch to fail
	ProviderCloneOrFetch = func(ctx context.Context, targetDir, repoURL, branch, token string) error {
		return fmt.Errorf("mock clone failure")
	}

	payload, _ := json.Marshal(map[string]string{"environmentId": env.ID})
	task := asynq.NewTask("build:environment", payload)

	err := HandleBuildEnvironmentTask(context.Background(), task)
	if err == nil {
		t.Errorf("Expected error from HandleBuildEnvironmentTask, got nil")
	}

	// Verify DB status is FAILED
	var updatedEnv models.Environment
	db.DB.First(&updatedEnv, "id = ?", env.ID)
	if updatedEnv.Status != models.StatusFailed {
		t.Errorf("Expected status %s, got %s", models.StatusFailed, updatedEnv.Status)
	}

	// Verify error log was written
	var logEntry models.Log
	if err := db.DB.Where("environment_id = ? AND level = ?", env.ID, models.LogLevelError).First(&logEntry).Error; err != nil {
		t.Errorf("Expected error log to be written, but not found")
	}
}

func TestHandleBuildEnvironmentTask_SidecarError(t *testing.T) {
	setupTestDB(t)

	env := models.Environment{
		Name:   "Sidecar Error Env",
		Status: models.StatusBuilding,
	}
	db.DB.Create(&env)

	// Mock providers
	ProviderCloneOrFetch = func(ctx context.Context, targetDir, repoURL, branch, token string) error {
		return nil
	}
	ProviderDetectDatabaseRequirements = func(workspaceDir string) (provider.DBType, error) {
		return provider.DBTypePostgres, nil
	}
	ProviderStartSidecarDatabase = func(ctx context.Context, envID, netID string, dbType provider.DBType) (string, error) {
		return "", fmt.Errorf("mock sidecar provision failure")
	}

	payload, _ := json.Marshal(map[string]string{"environmentId": env.ID})
	task := asynq.NewTask("build:environment", payload)

	err := HandleBuildEnvironmentTask(context.Background(), task)
	if err == nil {
		t.Errorf("Expected error from HandleBuildEnvironmentTask, got nil")
	}

	// Verify DB status is FAILED
	var updatedEnv models.Environment
	db.DB.First(&updatedEnv, "id = ?", env.ID)
	if updatedEnv.Status != models.StatusFailed {
		t.Errorf("Expected status %s, got %s", models.StatusFailed, updatedEnv.Status)
	}

	// Verify error log was written
	var logEntry models.Log
	if err := db.DB.Where("environment_id = ? AND level = ?", env.ID, models.LogLevelError).First(&logEntry).Error; err != nil {
		t.Errorf("Expected error log to be written, but not found")
	}
}
