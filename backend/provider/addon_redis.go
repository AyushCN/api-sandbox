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

type RedisProvider struct {
	client *docker.Client
}

func NewRedisProvider() *RedisProvider {
	client, _ := docker.NewVersionedClientFromEnv("1.41")
	return &RedisProvider{client: client}
}

func (p *RedisProvider) Provision(ctx context.Context, addon *models.Addon, orgID string) (string, error) {
	slog.Info("Provisioning Redis addon", "deploymentID", addon.DeploymentID)
	containerName := fmt.Sprintf("redis-%s", addon.DeploymentID)

	networkName, _, err := EnsureOrgNetwork(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to ensure network: %v", err)
	}



	containerInfo, err := p.client.InspectContainer(containerName)
	if err == nil {
		// Already exists. Ensure it's running.
		if !containerInfo.State.Running {
			if err := p.client.StartContainer(containerName, nil); err != nil {
				return "", fmt.Errorf("failed to start existing redis container: %v", err)
			}
		}

		// Extract password from existing command
		existingPassword, found := extractFlagValue(containerInfo.Config.Cmd, "--requirepass")
		if !found {
			return "", fmt.Errorf("failed to extract --requirepass flag from existing container")
		}

		return fmt.Sprintf("redis://:%s@%s:6379", existingPassword, containerName), nil
	}

	err = p.client.PullImage(docker.PullImageOptions{Repository: "redis", Tag: "alpine", Context: ctx}, docker.AuthConfiguration{})
	if err != nil {
		return "", fmt.Errorf("failed to pull redis image: %v", err)
	}

	// Generate random password for new container
	passwordBytes := make([]byte, 8)
	rand.Read(passwordBytes)
	password := hex.EncodeToString(passwordBytes)

	opts := docker.CreateContainerOptions{
		Name: containerName,
		Config: &docker.Config{
			Image: "redis:alpine",
			Cmd:   []string{"redis-server", "--requirepass", password},
			Labels: map[string]string{
				"deploymentID": addon.DeploymentID,
				"addonType":    "redis",
			},
		},
		HostConfig: &docker.HostConfig{
			NetworkMode: networkName,
		},
	}

	container, err := p.client.CreateContainer(opts)
	if err != nil {
		return "", fmt.Errorf("failed to create redis container: %v", err)
	}

	if err := p.client.StartContainer(container.ID, nil); err != nil {
		return "", fmt.Errorf("failed to start redis container: %v", err)
	}

	return fmt.Sprintf("redis://:%s@%s:6379", password, containerName), nil
}

func (p *RedisProvider) Deprovision(ctx context.Context, addon *models.Addon, orgID string) error {
	containerName := fmt.Sprintf("redis-%s", addon.DeploymentID)
	_ = p.client.StopContainer(containerName, 10)
	return p.client.RemoveContainer(docker.RemoveContainerOptions{
		ID:    containerName,
		Force: true,
	})
}
