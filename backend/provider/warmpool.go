package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	pullCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := dockerClient.PullImage(docker.PullImageOptions{Repository: image, Context: pullCtx, InactivityTimeout: time.Minute}, docker.AuthConfiguration{}); err != nil {
		return "", err
	}
	return image, nil
}

func createRuntimeContainer(ctx context.Context, envID, image, networkName, hostWorkspaceDir, cacheDir, workingDir string, envVars, entrypoint, command []string) (string, error) {
	pidsLimit := int64(256)
	opts := docker.CreateContainerOptions{
		Name:    fmt.Sprintf("api-sandbox-env-%s", envID),
		Context: ctx,
		Config: &docker.Config{
			Entrypoint: entrypoint,
			Image:      image,
			Cmd:        command,
			Env:        envVars,
			WorkingDir: workingDir,
		},
		HostConfig: &docker.HostConfig{
			Memory:      512 * 1024 * 1024,
			MemorySwap:  512 * 1024 * 1024,
			CPUQuota:    100000,
			CPUPeriod:   100000,
			CPUShares:   1024,
			PidsLimit:   &pidsLimit,
			SecurityOpt: []string{"no-new-privileges:true"},
			CapDrop:     []string{"ALL"},
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
	if err := dockerClient.StartContainerWithContext(container.ID, nil, ctx); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cleanupErr := CleanupContainer(cleanupCtx, container.ID)
		return "", errors.Join(fmt.Errorf("start environment container: %w", err), cleanupErr)
	}
	inspect, err := dockerClient.InspectContainerWithContext(container.ID, ctx)
	if err != nil || !inspect.State.Running {
		if err == nil {
			err = fmt.Errorf("runtime process exited immediately with status %d", inspect.State.ExitCode)
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cleanupErr := CleanupContainer(cleanupCtx, container.ID)
		return "", errors.Join(fmt.Errorf("runtime did not remain running after start: %w", err), cleanupErr)
	}
	return container.ID, nil
}

func ProvisionDevSandbox(ctx context.Context, envID string, config DevRuntimeConfig, userID string, dbURL string) (resultContainerID string, resultPort int, retErr error) {
	createLog(envID, fmt.Sprintf("Provisioning Dev Sandbox (Image: %s)...", config.BaseImage), models.LogLevelInfo)
	if err := CleanupContainer(ctx, fmt.Sprintf("api-sandbox-env-%s", envID)); err != nil {
		return "", 0, fmt.Errorf("remove previous environment container: %w", err)
	}

	networkName, networkID, err := EnsureUserNetwork(ctx, userID)
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
	config.ExposedPort = exposedPort

	// Consume a pre-pulled image marker. Warm containers cannot be reused safely:
	// their mounts are immutable and their writable state would cross tenants.
	var containerID string
	provisioned := false
	defer func() {
		if provisioned {
			return
		}
		cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelCleanup()
		if containerID != "" {
			if err := CleanupContainer(cleanupCtx, containerID); err != nil {
				slog.Error("Failed to clean partially provisioned runtime", "environment_id", envID, "container_id", containerID, "error", err)
				retErr = errors.Join(retErr, fmt.Errorf("cleanup partially provisioned runtime: %w", err))
			}
		}
		if err := ClearRuntimeRoute(cleanupCtx, envID); err != nil {
			slog.Error("Failed to clear partial Traefik route", "environment_id", envID, "error", err)
			retErr = errors.Join(retErr, fmt.Errorf("clear partial Traefik route: %w", err))
		}
	}()
	if config.RuntimeType == "node" || config.RuntimeType == "python" || config.RuntimeType == "go" {
		if warmImage, err := db.RedisClient.LPop(ctx, "warm-pool:"+config.RuntimeType).Result(); err == nil && warmImage == config.BaseImage {
			createLog(envID, "Using pre-pulled runtime image", models.LogLevelInfo)
		}
	}

	if containerID == "" {
		// Cold start
		pullCtx, cancelPull := context.WithTimeout(ctx, 10*time.Minute)
		defer cancelPull()
		if err := dockerClient.PullImage(docker.PullImageOptions{
			Repository:        config.BaseImage,
			Context:           pullCtx,
			InactivityTimeout: time.Minute,
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
		workingDir, err := NormalizeRuntimeWorkDir(config.WorkDir, envID)
		if err != nil {
			return "", 0, err
		}
		scriptPath := config.StartScriptPath
		if scriptPath == "" {
			scriptPath = "/app/sandbox-start.sh"
		}
		entrypoint := []string{"/bin/sh"}
		command := []string{scriptPath}
		if (config.RuntimeType == "docker" || config.RuntimeType == "devcontainer") && strings.TrimSpace(config.StartCmd) == "" {
			// Preserve the image's ENTRYPOINT/CMD for explicit container contracts.
			entrypoint = nil
			command = nil
		}
		envVars := runtimeEnvironment(config, dbURL)
		containerID, err = createRuntimeContainer(ctx, envID, config.BaseImage, networkName, hostWorkspaceDir, cacheSourceDir, workingDir, envVars, entrypoint, command)
		if err != nil {
			return "", 0, err
		}
		createLog(envID, "Created cold start container", models.LogLevelInfo)
	}

	if err := connectContainerToNetwork(networkName, networkID, containerID); err != nil {
		return "", 0, fmt.Errorf("connect runtime container to user network: %w", err)
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
	routeConfig := traefikRuntimeConfig(envID, domain, ip, exposedPort)
	pipe := rdb.Pipeline()
	for key, value := range routeConfig {
		pipe.Set(ctx, key, value, 0)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return "", 0, fmt.Errorf("write Traefik configuration: %w", err)
	}

	createLog(envID, fmt.Sprintf("Runtime container started as the application process on port %d.", assignedPort), models.LogLevelInfo)
	provisioned = true
	return containerID, assignedPort, nil
}

func traefikRuntimeConfig(envID, domain, ip, port string) map[string]string {
	routerPrefix := fmt.Sprintf("traefik/http/routers/env-%s", envID)
	config := map[string]string{
		routerPrefix + "/rule":          fmt.Sprintf("Host(`%s.%s`)", envID, domain),
		routerPrefix + "/service":       "env-" + envID,
		routerPrefix + "/entrypoints/0": "web",
		"traefik/http/services/env-" + envID + "/loadbalancer/servers/0/url": fmt.Sprintf("http://%s:%s", ip, port),
	}
	if domain != "localhost" {
		config[routerPrefix+"/entrypoints/0"] = "websecure"
		config[routerPrefix+"/tls/certresolver"] = "myresolver"
	}
	return config
}

func traefikRuntimeConfigKeys(envID string) []string {
	routerPrefix := fmt.Sprintf("traefik/http/routers/env-%s", envID)
	servicePrefix := fmt.Sprintf("traefik/http/services/env-%s", envID)
	return []string{
		routerPrefix,
		routerPrefix + "/rule",
		routerPrefix + "/service",
		routerPrefix + "/entrypoints/0",
		routerPrefix + "/tls/certresolver",
		servicePrefix + "/loadbalancer/servers/0",
		servicePrefix + "/loadbalancer/servers/0/url",
	}
}

func ClearRuntimeRoute(ctx context.Context, envID string) error {
	if db.RedisClient == nil {
		return nil
	}
	return db.RedisClient.Del(ctx, traefikRuntimeConfigKeys(envID)...).Err()
}

func runtimeEnvironment(config DevRuntimeConfig, dbURL string) []string {
	envVars := []string{fmt.Sprintf("PORT=%s", config.ExposedPort), "HOST=0.0.0.0"}
	if dbURL == "" {
		return envVars
	}
	envVars = append(envVars, "DATABASE_URL="+dbURL, "MONGO_URI="+dbURL)
	if u, err := url.Parse(dbURL); err == nil {
		envVars = append(envVars, "DB_HOST="+u.Hostname(), "DB_PORT="+u.Port())
		if u.User != nil {
			envVars = append(envVars, "DB_USER="+u.User.Username())
			if pwd, ok := u.User.Password(); ok {
				envVars = append(envVars, "DB_PASSWORD="+pwd)
			}
		}
		envVars = append(envVars, "DB_NAME="+strings.TrimPrefix(u.Path, "/"))
	}
	return envVars
}
