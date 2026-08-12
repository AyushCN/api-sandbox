package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/api-sandbox/backend/models"
	docker "github.com/fsouza/go-dockerclient"
	"log/slog"
)

type PostgresProvider struct {
	client *docker.Client
}

func NewPostgresProvider() *PostgresProvider {
	client, _ := docker.NewVersionedClientFromEnv("1.41")
	return &PostgresProvider{client: client}
}

func (p *PostgresProvider) Provision(ctx context.Context, addon *models.Addon, orgID string) (string, error) {
	slog.Info("Provisioning Postgres addon", "deploymentID", addon.DeploymentID)
	containerName := fmt.Sprintf("postgres-%s", addon.DeploymentID)

	networkName, _, err := EnsureOrgNetwork(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to ensure network: %v", err)
	}



	containerInfo, err := p.client.InspectContainer(containerName)
	if err == nil {
		// Already exists. Ensure it's running.
		if !containerInfo.State.Running {
			if err := p.client.StartContainer(containerName, nil); err != nil {
				return "", fmt.Errorf("failed to start existing postgres container: %v", err)
			}
		}

		// Extract password from existing environment
		existingPassword, found := extractEnvValue(containerInfo.Config.Env, "POSTGRES_PASSWORD")
		if !found {
			return "", fmt.Errorf("failed to extract POSTGRES_PASSWORD from existing container")
		}

		return fmt.Sprintf("postgresql://appuser:%s@%s:5432/myapp", existingPassword, containerName), nil
	}

	err = p.client.PullImage(docker.PullImageOptions{Repository: "postgres", Tag: "15", Context: ctx}, docker.AuthConfiguration{})
	if err != nil {
		return "", fmt.Errorf("failed to pull postgres image: %v", err)
	}

	// Generate random password for new container
	passwordBytes := make([]byte, 8)
	rand.Read(passwordBytes)
	password := hex.EncodeToString(passwordBytes)

	opts := docker.CreateContainerOptions{
		Name: containerName,
		Config: &docker.Config{
			Image: "postgres:15",
			Env: []string{
				"POSTGRES_DB=myapp",
				"POSTGRES_USER=appuser",
				fmt.Sprintf("POSTGRES_PASSWORD=%s", password),
			},
			Labels: map[string]string{
				"deploymentID": addon.DeploymentID,
				"addonType":    "postgres",
			},
		},
		HostConfig: &docker.HostConfig{
			NetworkMode: networkName,
		},
	}

	container, err := p.client.CreateContainer(opts)
	if err != nil {
		return "", fmt.Errorf("failed to create postgres container: %v", err)
	}

	if err := p.client.StartContainer(container.ID, nil); err != nil {
		return "", fmt.Errorf("failed to start postgres container: %v", err)
	}

	return fmt.Sprintf("postgresql://appuser:%s@%s:5432/myapp", password, containerName), nil
}

func (p *PostgresProvider) Deprovision(ctx context.Context, addon *models.Addon, orgID string) error {
	containerName := fmt.Sprintf("postgres-%s", addon.DeploymentID)
	_ = p.client.StopContainer(containerName, 10)
	return p.client.RemoveContainer(docker.RemoveContainerOptions{
		ID:    containerName,
		Force: true,
	})
}
