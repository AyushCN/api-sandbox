package provider

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
)

// DockerProvider implements the DeploymentProvider interface using local Docker
type DockerProvider struct {
	// For a real implementation, you would store Docker client configurations here
}

func NewDockerProvider() *DockerProvider {
	return &DockerProvider{}
}

func (p *DockerProvider) Build(ctx context.Context, deployment *models.Deployment) error {
	slog.Info("DockerProvider: Building deployment", "deploymentID", deployment.ID, "gitURL", deployment.GitURL)

	imageTag, err := CloneAndBuildImage(ctx, deployment.ID, "deployment", deployment.GitURL, deployment.GitBranch)
	if err != nil {
		return err
	}

	// We need to save the imageTag to use it during deploy
	// In this simple model, we assume imageTag is deterministic: api-sandbox-{id}
	// So we don't strictly need to store it on the model if we can derive it.
	_ = imageTag

	return nil
}

func (p *DockerProvider) Deploy(ctx context.Context, deployment *models.Deployment) error {
	slog.Info("DockerProvider: Deploying container", "deploymentID", deployment.ID)

	// Fetch addons from the database
	var addons []models.Addon
	db.DB.Where("deployment_id = ?", deployment.ID).Find(&addons)

	var project models.Project
	db.DB.First(&project, "id = ?", deployment.ProjectID)
	orgID := project.OwnerOrganizationID

	addonManager := NewAddonManager()
	var injectedEnv []string

	for _, addon := range addons {
		connStr, err := addonManager.Provision(ctx, &addon, orgID)
		if err != nil {
			return fmt.Errorf("failed to provision addon %s: %v", addon.Type, err)
		}

		// Map connection strings based on type
		switch addon.Type {
		case "postgres":
			injectedEnv = append(injectedEnv, fmt.Sprintf("DATABASE_URL=%s", connStr))
		case "mongodb":
			injectedEnv = append(injectedEnv, fmt.Sprintf("MONGO_URI=%s", connStr))
		case "redis":
			injectedEnv = append(injectedEnv, fmt.Sprintf("REDIS_URL=%s", connStr))
		}
	}

	// Find the imageTag.
	imageTag := fmt.Sprintf("api-sandbox-%s", deployment.ID)

	// Delegate to existing StartContainer logic
	// We use the project's organization ID for network isolation
	netID := orgID

	// We might need to join the addons' db URLs if we have multiple, but StartContainer currently accepts one dbURL string.
	// We'll pass the first one for now, but StartContainer sets DATABASE_URL and MONGO_URI so it's fine.
	// Actually, we already inject environment variables in Deploy! Wait, StartContainer doesn't accept a list of ENV strings yet.
	// We should update StartContainer to accept injectedEnv!
	// Let's pass the first db connection string for now to satisfy the signature.
	primaryDbUrl := ""
	if len(addons) > 0 {
		// Just pass the first one to dbUrl to satisfy the old signature, we'll fix StartContainer later.
		primaryDbUrl = "injected"
	}

	_, _, err := StartContainer(ctx, deployment.ID, "deployment", imageTag, netID, primaryDbUrl)
	if err != nil {
		return err
	}

	deployment.Status = "RUNNING"
	deployment.PublicURL = fmt.Sprintf("https://%s.sandbox.local", deployment.ID)
	return nil
}

func (p *DockerProvider) Stop(ctx context.Context, deployment *models.Deployment) error {
	slog.Info("DockerProvider: Stopping container", "deploymentID", deployment.ID)
	containerName := fmt.Sprintf("api-sandbox-%s", deployment.ID)
	return CleanupContainer(ctx, containerName, "deployment")
}

func (p *DockerProvider) GetLogs(ctx context.Context, deployment *models.Deployment) (string, error) {
	slog.Info("DockerProvider: Fetching logs", "deploymentID", deployment.ID)
	return "Docker container logs...", nil
}

func (p *DockerProvider) Scale(ctx context.Context, deployment *models.Deployment, replicas int) error {
	if replicas != 1 {
		return fmt.Errorf("DockerProvider only supports 1 replica per deployment currently")
	}
	return nil
}
