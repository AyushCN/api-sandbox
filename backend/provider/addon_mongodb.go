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

type MongoProvider struct {
	client *docker.Client
}

func NewMongoProvider() *MongoProvider {
	client, _ := docker.NewVersionedClientFromEnv("1.41")
	return &MongoProvider{client: client}
}

func (p *MongoProvider) Provision(ctx context.Context, addon *models.Addon, orgID string) (string, error) {
	slog.Info("Provisioning MongoDB addon", "deploymentID", addon.DeploymentID)
	containerName := fmt.Sprintf("mongo-%s", addon.DeploymentID)

	networkName, _, err := EnsureOrgNetwork(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to ensure network: %v", err)
	}



	containerInfo, err := p.client.InspectContainer(containerName)
	if err == nil {
		// Already exists. Ensure it's running.
		if !containerInfo.State.Running {
			if err := p.client.StartContainer(containerName, nil); err != nil {
				return "", fmt.Errorf("failed to start existing mongo container: %v", err)
			}
		}

		// Extract password from existing environment
		existingPassword, found := extractEnvValue(containerInfo.Config.Env, "MONGO_INITDB_ROOT_PASSWORD")
		if !found {
			return "", fmt.Errorf("failed to extract MONGO_INITDB_ROOT_PASSWORD from existing container")
		}

		return fmt.Sprintf("mongodb://admin:%s@%s:27017/myapp?authSource=admin", existingPassword, containerName), nil
	}

	err = p.client.PullImage(docker.PullImageOptions{Repository: "mongo", Tag: "6.0", Context: ctx}, docker.AuthConfiguration{})
	if err != nil {
		return "", fmt.Errorf("failed to pull mongo image: %v", err)
	}

	// Generate random password for new container
	passwordBytes := make([]byte, 8)
	rand.Read(passwordBytes)
	password := hex.EncodeToString(passwordBytes)

	opts := docker.CreateContainerOptions{
		Name: containerName,
		Config: &docker.Config{
			Image: "mongo:6.0",
			Env: []string{
				"MONGO_INITDB_DATABASE=myapp",
				"MONGO_INITDB_ROOT_USERNAME=admin",
				fmt.Sprintf("MONGO_INITDB_ROOT_PASSWORD=%s", password),
			},
			Labels: map[string]string{
				"deploymentID": addon.DeploymentID,
				"addonType":    "mongo",
			},
		},
		HostConfig: &docker.HostConfig{
			NetworkMode: networkName,
		},
	}

	container, err := p.client.CreateContainer(opts)
	if err != nil {
		return "", fmt.Errorf("failed to create mongo container: %v", err)
	}

	if err := p.client.StartContainer(container.ID, nil); err != nil {
		return "", fmt.Errorf("failed to start mongo container: %v", err)
	}

	return fmt.Sprintf("mongodb://admin:%s@%s:27017/myapp?authSource=admin", password, containerName), nil
}

func (p *MongoProvider) Deprovision(ctx context.Context, addon *models.Addon, orgID string) error {
	containerName := fmt.Sprintf("mongo-%s", addon.DeploymentID)
	_ = p.client.StopContainer(containerName, 10)
	return p.client.RemoveContainer(docker.RemoveContainerOptions{
		ID:    containerName,
		Force: true,
	})
}
