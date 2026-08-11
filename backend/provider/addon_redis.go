package provider

import (
	"context"
	"fmt"
	"log/slog"
	"github.com/api-sandbox/backend/models"
	docker "github.com/fsouza/go-dockerclient"
)

type RedisProvider struct {
	client *docker.Client
}

func NewRedisProvider() *RedisProvider {
	client, _ := docker.NewVersionedClientFromEnv("1.41")
	return &RedisProvider{client: client}
}

func (p *RedisProvider) Provision(ctx context.Context, addon *models.Addon) (string, error) {
	slog.Info("Provisioning Redis addon", "deploymentID", addon.DeploymentID)
	containerName := fmt.Sprintf("redis-%s", addon.DeploymentID)

	_, err := p.client.InspectContainer(containerName)
	if err == nil {
		return fmt.Sprintf("redis://%s:6379", containerName), nil
	}

	err = p.client.PullImage(docker.PullImageOptions{Repository: "redis", Tag: "alpine", Context: ctx}, docker.AuthConfiguration{})
	if err != nil {
		return "", fmt.Errorf("failed to pull redis image: %v", err)
	}

	opts := docker.CreateContainerOptions{
		Name: containerName,
		Config: &docker.Config{
			Image: "redis:alpine",
			Labels: map[string]string{
				"deploymentID": addon.DeploymentID,
				"addonType":    "redis",
			},
		},
		HostConfig: &docker.HostConfig{
			NetworkMode: "api-sandbox-network",
		},
	}

	container, err := p.client.CreateContainer(opts)
	if err != nil {
		return "", fmt.Errorf("failed to create redis container: %v", err)
	}

	if err := p.client.StartContainer(container.ID, nil); err != nil {
		return "", fmt.Errorf("failed to start redis container: %v", err)
	}

	return fmt.Sprintf("redis://%s:6379", containerName), nil
}

func (p *RedisProvider) Deprovision(ctx context.Context, addon *models.Addon) error {
	containerName := fmt.Sprintf("redis-%s", addon.DeploymentID)
	_ = p.client.StopContainer(containerName, 10)
	return p.client.RemoveContainer(docker.RemoveContainerOptions{
		ID:    containerName,
		Force: true,
	})
}
