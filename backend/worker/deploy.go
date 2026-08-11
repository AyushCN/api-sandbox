package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/api-sandbox/backend/provider"
	"github.com/hibiken/asynq"
)

func HandleDeployTask(ctx context.Context, t *asynq.Task) error {
	var payload map[string]string
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	deploymentID, ok := payload["deploymentId"]
	if !ok {
		return fmt.Errorf("missing deploymentId: %w", asynq.SkipRetry)
	}

	slog.Info("Processing deployment job", "deployment_id", deploymentID)

	var deployment models.Deployment
	if err := db.DB.First(&deployment, "id = ?", deploymentID).Error; err != nil {
		return fmt.Errorf("deployment not found: %w", err)
	}

	db.DB.Model(&deployment).Update("status", "BUILDING")

	// Instantiate the correct provider
	var p provider.DeploymentProvider
	switch deployment.ProviderType {
	case "docker":
		p = provider.NewDockerProvider()
	case "heroku":
		p = provider.NewHerokuProvider("") // Should retrieve from org settings
	case "railway":
		p = provider.NewRailwayProvider("") // Should retrieve from org settings
	default:
		// Fallback to Docker
		p = provider.NewDockerProvider()
	}

	if err := p.Build(ctx, &deployment); err != nil {
		slog.Error("Build phase failed", "deployment_id", deploymentID, "error", err)
		db.DB.Model(&deployment).Update("status", "FAILED")
		return err
	}

	if err := p.Deploy(ctx, &deployment); err != nil {
		slog.Error("Deploy phase failed", "deployment_id", deploymentID, "error", err)
		db.DB.Model(&deployment).Update("status", "FAILED")
		return err
	}

	// Persist the updated deployment (e.g. status, public URL)
	db.DB.Save(&deployment)

	slog.Info("Deployment job completed successfully", "deployment_id", deploymentID)
	return nil
}
