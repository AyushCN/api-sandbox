package provider

import (
	"context"
	"github.com/api-sandbox/backend/models"
)

// DeploymentProvider is the universal interface for all deployment targets
type DeploymentProvider interface {
	// Build prepares the application (e.g., building a Docker image)
	Build(ctx context.Context, deployment *models.Deployment) error

	// Deploy launches the application on the provider's infrastructure
	Deploy(ctx context.Context, deployment *models.Deployment) error

	// Stop halts a running deployment
	Stop(ctx context.Context, deployment *models.Deployment) error

	// GetLogs streams or retrieves logs for a deployment
	GetLogs(ctx context.Context, deployment *models.Deployment) (string, error)

	// Scale adjusts the number of replicas for a deployment
	Scale(ctx context.Context, deployment *models.Deployment, replicas int) error
}

// AddonProvider manages external databases and services
type AddonProvider interface {
	// Provision creates the requested add-on (e.g., PostgreSQL database)
	Provision(ctx context.Context, addon *models.Addon, orgID string) (string, error)

	// Deprovision removes the add-on
	Deprovision(ctx context.Context, addon *models.Addon, orgID string) error
}

// Buildpack handles language-specific build logic
type Buildpack interface {
	// Detect returns true if this buildpack can build the code at repoPath
	Detect(repoPath string) bool

	// Build generates the container image or deployment artifact
	Build(ctx context.Context, repoPath string) (string, error)

	// GetPort returns the default port the application listens on
	GetPort(repoPath string) int
}
