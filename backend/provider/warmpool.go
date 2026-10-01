package provider

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/google/uuid"
)

func CreateWarmContainer(ctx context.Context, runtimeType string) (string, error) {
	var image string
	switch runtimeType {
	case "node":
		image = "node:20-alpine"
	case "python":
		image = "python:3.11-slim"
	case "go":
		image = "golang:alpine"
	default:
		return "", fmt.Errorf("unsupported warm pool runtime: %s", runtimeType)
	}

	_ = dockerClient.PullImage(docker.PullImageOptions{
		Repository: image,
	}, docker.AuthConfiguration{})

	id := uuid.New().String()
	name := fmt.Sprintf("api-sandbox-warm-%s-%s", runtimeType, id)

	hostWorkspacesDir := os.Getenv("HOST_WORKSPACES_DIR")
	if hostWorkspacesDir == "" {
		wd, _ := os.Getwd()
		hostWorkspacesDir = filepath.Join(wd, "workspaces")
	}

	cacheDir := os.Getenv("HOST_CACHE_DIR")
	if cacheDir == "" {
		wd, _ := os.Getwd()
		cacheDir = filepath.Join(wd, "cache")
	}
	os.MkdirAll(cacheDir, 0755)

	pidsLimit := int64(256)
	opts := docker.CreateContainerOptions{
		Name: name,
		Config: &docker.Config{
			Image: image,
			Cmd:   []string{"sleep", "infinity"},
		},
		HostConfig: &docker.HostConfig{
			Memory:        512 * 1024 * 1024,
			MemorySwap:    512 * 1024 * 1024,
			CPUQuota:      100000,
			CPUPeriod:     100000,
			CPUShares:     1024,
			PidsLimit:     &pidsLimit,
			RestartPolicy: docker.RestartOnFailure(3),
			SecurityOpt:   []string{"no-new-privileges:true"},
			CapDrop:       []string{"ALL"},
			Binds: []string{
				fmt.Sprintf("%s:/workspaces", hostWorkspacesDir),
				fmt.Sprintf("%s/npm:/root/.npm", cacheDir),
				fmt.Sprintf("%s/pnpm:/root/.local/share/pnpm/store", cacheDir),
				fmt.Sprintf("%s/pip:/root/.cache/pip", cacheDir),
				fmt.Sprintf("%s/go:/go/pkg/mod", cacheDir),
			},
		},
	}

	container, err := dockerClient.CreateContainer(opts)
	if err != nil {
		return "", err
	}

	if err := dockerClient.StartContainer(container.ID, nil); err != nil {
		return "", err
	}

	return container.ID, nil
}

func ProvisionDevSandbox(ctx context.Context, envID string, config DevRuntimeConfig, orgID string, dbURL string) (string, int, error) {
	createLog(envID, fmt.Sprintf("Provisioning Dev Sandbox (Image: %s)...", config.BaseImage), models.LogLevelInfo)
	_ = CleanupContainer(ctx, fmt.Sprintf("api-sandbox-env-%s", envID))

	networkName, networkID, err := EnsureOrgNetwork(ctx, orgID)
	if err != nil {
		createLog(envID, err.Error(), models.LogLevelError)
		return "", 0, err
	}

	if networkID != "" {
		_ = dockerClient.ConnectNetwork(networkID, docker.NetworkConnectionOptions{
			Container: "api-sandbox-traefik",
		})
	}

	domain := os.Getenv("DOMAIN")
	if domain == "" {
		domain = "localhost"
	}

	exposedPort := config.ExposedPort
	if exposedPort == "" {
		exposedPort = "5000" // Fallback
	}

	assignedPort, _ := strconv.Atoi(exposedPort)
	if assignedPort == 0 {
		assignedPort = 8080 // fallback
	}

	// Try to pop warm container
	var containerID string
	if config.RuntimeType == "node" || config.RuntimeType == "python" || config.RuntimeType == "go" {
		warmID, err := db.RedisClient.LPop(ctx, "warm-pool:"+config.RuntimeType).Result()
		if err == nil && warmID != "" {
			// Rename container to be picked up by reaper properly
			_ = dockerClient.RenameContainer(docker.RenameContainerOptions{
				ID:   warmID,
				Name: fmt.Sprintf("api-sandbox-env-%s", envID),
			})
			containerID = warmID
			createLog(envID, "Reused pre-started warm container", models.LogLevelInfo)
		}
	}

	if containerID == "" {
		// Cold start
		_ = dockerClient.PullImage(docker.PullImageOptions{
			Repository: config.BaseImage,
		}, docker.AuthConfiguration{})

		wd, _ := os.Getwd()
		hostWorkspacesDir := os.Getenv("HOST_WORKSPACES_DIR")
		if hostWorkspacesDir == "" {
			hostWorkspacesDir = filepath.Join(wd, "workspaces")
		}

		cacheDir := os.Getenv("HOST_CACHE_DIR")
		if cacheDir == "" {
			cacheDir = filepath.Join(wd, "cache")
		}
		os.MkdirAll(cacheDir, 0755)

		hostWorkspaceDir := filepath.Join(hostWorkspacesDir, envID)

		pidsLimit := int64(256)
		opts := docker.CreateContainerOptions{
			Name: fmt.Sprintf("api-sandbox-env-%s", envID),
			Config: &docker.Config{
				Image: config.BaseImage,
				Cmd:   []string{"sleep", "infinity"},
			},
			HostConfig: &docker.HostConfig{
				Memory:        512 * 1024 * 1024,
				MemorySwap:    512 * 1024 * 1024,
				CPUQuota:      100000,
				CPUPeriod:     100000,
				CPUShares:     1024,
				PidsLimit:     &pidsLimit,
				RestartPolicy: docker.RestartOnFailure(3),
				SecurityOpt:   []string{"no-new-privileges:true"},
				CapDrop:       []string{"ALL"},
				Binds: []string{
					fmt.Sprintf("%s:/workspaces", hostWorkspacesDir),
					fmt.Sprintf("%s:/app", hostWorkspaceDir),
					fmt.Sprintf("%s/npm:/root/.npm", cacheDir),
					fmt.Sprintf("%s/pnpm:/root/.local/share/pnpm/store", cacheDir),
					fmt.Sprintf("%s/pip:/root/.cache/pip", cacheDir),
					fmt.Sprintf("%s/go:/go/pkg/mod", cacheDir),
				},
			},
		}

		container, err := dockerClient.CreateContainer(opts)
		if err != nil {
			return "", 0, fmt.Errorf("failed to create container: %v", err)
		}
		if err := dockerClient.StartContainer(container.ID, nil); err != nil {
			return "", 0, fmt.Errorf("failed to start container: %v", err)
		}
		containerID = container.ID
		createLog(envID, "Created cold start container", models.LogLevelInfo)
	}

	_ = dockerClient.ConnectNetwork(networkID, docker.NetworkConnectionOptions{
		Container: containerID,
	})

	containerInfo, err := dockerClient.InspectContainer(containerID)
	if err != nil {
		return "", 0, fmt.Errorf("failed to inspect container: %v", err)
	}

	ip := containerInfo.NetworkSettings.Networks[networkName].IPAddress

	// Write Traefik configuration to Redis
	rdb := db.RedisClient
	prefix := fmt.Sprintf("traefik/http/routers/env-%s", envID)

	rdb.HSet(ctx, prefix, "rule", fmt.Sprintf("Host(`%s.%s`)", envID, domain))
	rdb.HSet(ctx, prefix, "service", fmt.Sprintf("env-%s", envID))
	if domain != "localhost" {
		rdb.HSet(ctx, prefix, "entrypoints", "websecure")
		rdb.HSet(ctx, prefix, "tls.certresolver", "myresolver")
	} else {
		rdb.HSet(ctx, prefix, "entrypoints", "web")
	}
	rdb.HSet(ctx, fmt.Sprintf("traefik/http/services/env-%s/loadbalancer/servers/0", envID), "url", fmt.Sprintf("http://%s:%s", ip, exposedPort))

	// Execute sandbox-start.sh
	envVars := []string{
		fmt.Sprintf("PORT=%s", exposedPort),
		"HOST=0.0.0.0",
	}
	if dbURL != "" {
		envVars = append(envVars, fmt.Sprintf("DATABASE_URL=%s", dbURL), fmt.Sprintf("MONGO_URI=%s", dbURL))
		if u, err := url.Parse(dbURL); err == nil {
			envVars = append(envVars, fmt.Sprintf("DB_HOST=%s", u.Hostname()))
			envVars = append(envVars, fmt.Sprintf("DB_PORT=%s", u.Port()))
			envVars = append(envVars, fmt.Sprintf("DB_USER=%s", u.User.Username()))
			if pwd, ok := u.User.Password(); ok {
				envVars = append(envVars, fmt.Sprintf("DB_PASSWORD=%s", pwd))
			}
			envVars = append(envVars, fmt.Sprintf("DB_NAME=%s", strings.TrimPrefix(u.Path, "/")))
		}
	}

	workingDir := fmt.Sprintf("/workspaces/%s%s", envID, strings.TrimPrefix(config.WorkDir, "/app"))
	if config.RuntimeType == "" || config.RuntimeType == "docker" || config.RuntimeType == "devcontainer" {
		workingDir = config.WorkDir
	}
	// Fix: If the workspace is mounted at /app in the container, workingDir should be /app
	// The workspace is mounted at /app via the bind mount: fmt.Sprintf("%s:/app", hostWorkspaceDir)
	if strings.HasPrefix(workingDir, "/workspaces/") {
		workingDir = "/app" + strings.TrimPrefix(workingDir, fmt.Sprintf("/workspaces/%s", envID))
	}

	execOpts := docker.CreateExecOptions{
		Container:    containerID,
		Cmd:          []string{"/bin/sh", "sandbox-start.sh"},
		WorkingDir:   workingDir,
		Env:          envVars,
		AttachStdout: true,
		AttachStderr: true,
	}

	exec, err := dockerClient.CreateExec(execOpts)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create exec: %v", err)
	}

	err = dockerClient.StartExec(exec.ID, docker.StartExecOptions{
		Detach: true,
	})
	if err != nil {
		return "", 0, fmt.Errorf("failed to start exec: %v", err)
	}

	createLog(envID, fmt.Sprintf("Dev Sandbox started successfully on port %d.", assignedPort), models.LogLevelInfo)
	return containerID, assignedPort, nil
}
