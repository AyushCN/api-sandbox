package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		slog.Warn("No .env file found")
	}

	db.InitDB()

	client, err := docker.NewVersionedClientFromEnv("1.41")
	if err != nil {
		slog.Error("Failed to initialize docker client", "error", err)
		os.Exit(1)
	}

	// 1. Delete all Addon records and Log an activity
	var addons []models.Addon
	db.DB.Find(&addons)

	if len(addons) > 0 {
		slog.Info(fmt.Sprintf("Found %d Addon records. Deleting them...", len(addons)))
		for _, addon := range addons {
			// Create Activity
			activity := models.Activity{
				DeploymentID: &addon.DeploymentID,
				Type:         "addon_removed",
				Data:         fmt.Sprintf(`{"message": "Addon %s removed due to security migration. Please redeploy."}`, addon.Type),
			}
			db.DB.Create(&activity)

			// Delete Addon
			db.DB.Delete(&addon)
		}
	} else {
		slog.Info("No Addon records found in database.")
	}

	// 2. Kill containers on api-sandbox-network
	networkName := "api-sandbox-network"

	networks, err := client.ListNetworks()
	if err != nil {
		slog.Error("Failed to list networks", "error", err)
		os.Exit(1)
	}

	var networkID string
	for _, net := range networks {
		if net.Name == networkName {
			networkID = net.ID
			break
		}
	}

	if networkID == "" {
		slog.Info("Network 'api-sandbox-network' does not exist. Nothing to clean up.")
		return
	}

	net, err := client.NetworkInfo(networkID)
	if err != nil {
		slog.Error("Failed to inspect network", "error", err)
		os.Exit(1)
	}

	for _, containerInfo := range net.Containers {
		slog.Info(fmt.Sprintf("Force removing container %s (%s)...", containerInfo.Name, containerInfo.IPv4Address))
		err := client.RemoveContainer(docker.RemoveContainerOptions{
			ID:    containerInfo.Name,
			Force: true,
		})
		if err != nil {
			slog.Error("Failed to remove container", "error", err)
		}
	}

	// 3. Delete the network
	slog.Info("Removing network 'api-sandbox-network'...")
	err = client.RemoveNetwork(networkID)
	if err != nil {
		slog.Error("Failed to remove network", "error", err)
		os.Exit(1)
	}

	slog.Info("Teardown completed successfully!")
}
