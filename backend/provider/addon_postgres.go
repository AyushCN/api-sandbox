package provider

import (
	"context"
	"fmt"
	"log/slog"
	"github.com/api-sandbox/backend/models"
	docker "github.com/fsouza/go-dockerclient"
)

type PostgresProvider struct {
	client *docker.Client
}

func NewPostgresProvider() *PostgresProvider {
	client, _ := docker.NewVersionedClientFromEnv("1.41")
	return &PostgresProvider{client: client}
}

func (p *PostgresProvider) Provision(ctx context.Context, addon *models.Addon) (string, error) {
	slog.Info("Provisioning Postgres addon", "deploymentID", addon.DeploymentID)
	containerName := fmt.Sprintf("postgres-%s", addon.DeploymentID)

	_, err := p.client.InspectContainer(containerName)
	if err == nil {
		// Already exists
		return fmt.Sprintf("postgresql://appuser:apppassword@%s:5432/myapp", containerName), nil
	}

	// Pull image if not exists
	err = p.client.PullImage(docker.PullImageOptions{Repository: "postgres", Tag: "15", Context: ctx}, docker.AuthConfiguration{})
	if err != nil {
		return "", fmt.Errorf("failed to pull postgres image: %v", err)
	}

	opts := docker.CreateContainerOptions{
		Name: containerName,
		Config: &docker.Config{
			Image: "postgres:15",
			Env: []string{
				"POSTGRES_DB=myapp",
				"POSTGRES_USER=appuser",
				"POSTGRES_PASSWORD=apppassword",
			},
			Labels: map[string]string{
				"deploymentID": addon.DeploymentID,
				"addonType":    "postgres",
			},
		},
		HostConfig: &docker.HostConfig{
			NetworkMode: "api-sandbox-network",
		},
	}

	container, err := p.client.CreateContainer(opts)
	if err != nil {
		return "", fmt.Errorf("failed to create postgres container: %v", err)
	}

	if err := p.client.StartContainer(container.ID, nil); err != nil {
		return "", fmt.Errorf("failed to start postgres container: %v", err)
	}

	return fmt.Sprintf("postgresql://appuser:apppassword@%s:5432/myapp", containerName), nil
}

func (p *PostgresProvider) Deprovision(ctx context.Context, addon *models.Addon) error {
	containerName := fmt.Sprintf("postgres-%s", addon.DeploymentID)
	_ = p.client.StopContainer(containerName, 10)
	return p.client.RemoveContainer(docker.RemoveContainerOptions{
		ID:    containerName,
		Force: true,
	})
}
