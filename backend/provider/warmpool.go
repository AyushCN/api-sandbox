package provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	docker "github.com/fsouza/go-dockerclient"
)

// ResetWarmPool drains pre-migration warm containers and Redis IDs. Those
// containers may still have the former shared /workspaces bind; new pool
// entries are image references and are never reusable containers.
func ResetWarmPool(ctx context.Context) error {
	containers, err := dockerClient.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		return fmt.Errorf("list legacy warm containers: %w", err)
	}
	for _, container := range containers {
		for _, name := range container.Names {
			if strings.HasPrefix(strings.TrimPrefix(name, "/"), "api-sandbox-warm-") {
				if err := CleanupContainer(ctx, container.ID); err != nil {
					return fmt.Errorf("remove legacy warm container: %w", err)
				}
				break
			}
		}
	}
	if err := db.RedisClient.Del(ctx, "warm-pool:node", "warm-pool:python", "warm-pool:go").Err(); err != nil {
		return fmt.Errorf("clear legacy warm pool entries: %w", err)
	}
	return nil
}

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

	// Docker mounts are immutable after container creation. Warm the image only;
	// a per-environment container is always created later with that environment's
	// workspace mount and fresh writable cache paths.
	if err := dockerClient.PullImage(docker.PullImageOptions{Repository: image}, docker.AuthConfiguration{}); err != nil {
		return "", err
	}
	return image, nil
}

func createRuntimeContainer(envID, image, networkName, hostWorkspaceDir, cacheDir string, command []string) (string, error) {
	pidsLimit := int64(256)
	opts := docker.CreateContainerOptions{
		Name: fmt.Sprintf("api-sandbox-env-%s", envID),
		Config: &docker.Config{
			Image: image,
			Cmd:   command,
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
				fmt.Sprintf("%s:/app", hostWorkspaceDir),
				fmt.Sprintf("%s/npm:/root/.npm", cacheDir),
				fmt.Sprintf("%s/pnpm:/root/.local/share/pnpm/store", cacheDir),
				fmt.Sprintf("%s/pip:/root/.cache/pip", cacheDir),
				fmt.Sprintf("%s/go:/go/pkg/mod", cacheDir),
			},
		},
		NetworkingConfig: &docker.NetworkingConfig{
			EndpointsConfig: map[string]*docker.EndpointConfig{networkName: {}},
		},
	}
	container, err := dockerClient.CreateContainer(opts)
	if err != nil {
		return "", fmt.Errorf("create environment container: %w", err)
	}
	if err := dockerClient.StartContainer(container.ID, nil); err != nil {
		cleanupErr := CleanupContainer(context.Background(), container.ID)
		return "", errors.Join(fmt.Errorf("start environment container: %w", err), cleanupErr)
	}
	return container.ID, nil
}

func ProvisionDevSandbox(ctx context.Context, envID string, config DevRuntimeConfig, orgID string, dbURL string) (string, int, error) {
	createLog(envID, fmt.Sprintf("Provisioning Dev Sandbox (Image: %s)...", config.BaseImage), models.LogLevelInfo)
	if err := CleanupContainer(ctx, fmt.Sprintf("api-sandbox-env-%s", envID)); err != nil {
		return "", 0, fmt.Errorf("remove previous environment container: %w", err)
	}

	networkName, networkID, err := EnsureOrgNetwork(ctx, orgID)
	if err != nil {
		createLog(envID, err.Error(), models.LogLevelError)
		return "", 0, err
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

	// Consume a pre-pulled image marker. Warm containers cannot be reused safely:
	// their mounts are immutable and their writable state would cross tenants.
	var containerID string
	provisioned := false
	defer func() {
		if provisioned {
			return
		}
		if containerID != "" {
			_ = CleanupContainer(context.Background(), containerID)
		}
		_ = db.RedisClient.Del(context.Background(),
			fmt.Sprintf("traefik/http/routers/env-%s", envID),
			fmt.Sprintf("traefik/http/services/env-%s/loadbalancer/servers/0", envID),
		).Err()
	}()
	if config.RuntimeType == "node" || config.RuntimeType == "python" || config.RuntimeType == "go" {
		if warmImage, err := db.RedisClient.LPop(ctx, "warm-pool:"+config.RuntimeType).Result(); err == nil && warmImage == config.BaseImage {
			createLog(envID, "Using pre-pulled runtime image", models.LogLevelInfo)
		}
	}

	if containerID == "" {
		// Cold start
		if err := dockerClient.PullImage(docker.PullImageOptions{
			Repository: config.BaseImage,
		}, docker.AuthConfiguration{}); err != nil {
			return "", 0, fmt.Errorf("pull runtime image %s: %w", config.BaseImage, err)
		}

		wd, _ := os.Getwd()
		hostWorkspacesDir := os.Getenv("HOST_WORKSPACES_DIR")
		if hostWorkspacesDir == "" {
			hostWorkspacesDir = filepath.Join(wd, "workspaces")
		}

		cacheHostRoot := os.Getenv("HOST_CACHE_DIR")
		cacheVisibleRoot := cacheHostRoot
		if cacheHostRoot == "" {
			// Docker interprets bind sources on the daemon host, not in this
			// backend container. Cache files remain reachable through the already
			// mounted workspace root, while Docker receives the host-side source.
			cacheHostRoot = filepath.Join(hostWorkspacesDir, ".cache")
			cacheVisibleRoot = filepath.Join(GetWorkspacesRootDir(), ".cache")
		}
		cacheSourceDir := filepath.Join(cacheHostRoot, envID)
		if err := os.MkdirAll(filepath.Join(cacheVisibleRoot, envID), 0755); err != nil {
			return "", 0, fmt.Errorf("failed to create isolated cache directory: %w", err)
		}

		hostWorkspaceDir := filepath.Join(hostWorkspacesDir, envID)

		containerID, err = createRuntimeContainer(envID, config.BaseImage, networkName, hostWorkspaceDir, cacheSourceDir, []string{"sleep", "infinity"})
		if err != nil {
			return "", 0, err
		}
		createLog(envID, "Created cold start container", models.LogLevelInfo)
	}

	if err := connectContainerToNetwork(networkName, networkID, containerID); err != nil {
		return "", 0, fmt.Errorf("connect runtime container to organization network: %w", err)
	}

	containerInfo, err := dockerClient.InspectContainer(containerID)
	if err != nil {
		return "", 0, fmt.Errorf("failed to inspect container: %v", err)
	}

	endpoint, connected := containerInfo.NetworkSettings.Networks[networkName]
	if !connected || endpoint.IPAddress == "" {
		return "", 0, fmt.Errorf("runtime container %s has no IP on network %s", containerID, networkName)
	}
	ip := endpoint.IPAddress

	// Write Traefik configuration to Redis
	rdb := db.RedisClient
	prefix := fmt.Sprintf("traefik/http/routers/env-%s", envID)

	routerFields := map[string]string{
		"rule":    fmt.Sprintf("Host(`%s.%s`)", envID, domain),
		"service": fmt.Sprintf("env-%s", envID),
	}
	if domain != "localhost" {
		routerFields["entrypoints"] = "websecure"
		routerFields["tls.certresolver"] = "myresolver"
	} else {
		routerFields["entrypoints"] = "web"
	}
	pipe := rdb.Pipeline()
	pipe.HSet(ctx, prefix, routerFields)
	pipe.HSet(ctx, fmt.Sprintf("traefik/http/services/env-%s/loadbalancer/servers/0", envID), "url", fmt.Sprintf("http://%s:%s", ip, exposedPort))
	if _, err := pipe.Exec(ctx); err != nil {
		return "", 0, fmt.Errorf("write Traefik configuration: %w", err)
	}

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

	workingDir := "/app" + strings.TrimPrefix(config.WorkDir, "/app")
	if config.RuntimeType == "" || config.RuntimeType == "docker" || config.RuntimeType == "devcontainer" {
		workingDir = config.WorkDir
	}
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
	provisioned = true
	return containerID, assignedPort, nil
}
