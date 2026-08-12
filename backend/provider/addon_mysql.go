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

type MySQLProvider struct {
	client *docker.Client
}

func NewMySQLProvider() *MySQLProvider {
	client, _ := docker.NewVersionedClientFromEnv("1.41")
	return &MySQLProvider{client: client}
}

func (p *MySQLProvider) Provision(ctx context.Context, addon *models.Addon, orgID string) (string, error) {
	slog.Info("Provisioning MySQL addon", "deploymentID", addon.DeploymentID)
	containerName := fmt.Sprintf("mysql-%s", addon.DeploymentID)

	networkName, _, err := EnsureOrgNetwork(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to ensure network: %v", err)
	}

	containerInfo, err := p.client.InspectContainer(containerName)
	if err == nil {
		// Already exists. Ensure it's running.
		if !containerInfo.State.Running {
			if err := p.client.StartContainer(containerName, nil); err != nil {
				return "", fmt.Errorf("failed to start existing mysql container: %v", err)
			}
		}

		// Extract password from existing environment
		existingPassword, found := extractEnvValue(containerInfo.Config.Env, "MYSQL_PASSWORD")
		if !found {
			return "", fmt.Errorf("failed to extract MYSQL_PASSWORD from existing container")
		}

		return fmt.Sprintf("mysql://appuser:%s@%s:3306/myapp", existingPassword, containerName), nil
	}

	err = p.client.PullImage(docker.PullImageOptions{Repository: "mysql", Tag: "8.0", Context: ctx}, docker.AuthConfiguration{})
	if err != nil {
		return "", fmt.Errorf("failed to pull mysql image: %v", err)
	}

	// Generate random password for new container
	passwordBytes := make([]byte, 8)
	rand.Read(passwordBytes)
	password := hex.EncodeToString(passwordBytes)

	opts := docker.CreateContainerOptions{
		Name: containerName,
		Config: &docker.Config{
			Image: "mysql:8.0",
			Env: []string{
				"MYSQL_ROOT_PASSWORD=" + password,
				"MYSQL_DATABASE=myapp",
				"MYSQL_USER=appuser",
				fmt.Sprintf("MYSQL_PASSWORD=%s", password),
			},
			Labels: map[string]string{
				"deploymentID": addon.DeploymentID,
				"addonType":    "mysql",
			},
		},
		HostConfig: &docker.HostConfig{
			NetworkMode: networkName,
		},
	}

	container, err := p.client.CreateContainer(opts)
	if err != nil {
		return "", fmt.Errorf("failed to create mysql container: %v", err)
	}

	if err := p.client.StartContainer(container.ID, nil); err != nil {
		return "", fmt.Errorf("failed to start mysql container: %v", err)
	}

	return fmt.Sprintf("mysql://appuser:%s@%s:3306/myapp", password, containerName), nil
}

func (p *MySQLProvider) Deprovision(ctx context.Context, addon *models.Addon, orgID string) error {
	containerName := fmt.Sprintf("mysql-%s", addon.DeploymentID)
	_ = p.client.StopContainer(containerName, 10)
	return p.client.RemoveContainer(docker.RemoveContainerOptions{
		ID:    containerName,
		Force: true,
	})
}
