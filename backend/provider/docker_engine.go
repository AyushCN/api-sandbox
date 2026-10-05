package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	docker "github.com/fsouza/go-dockerclient"
	"gorm.io/gorm"
)

var dockerClient *docker.Client

const dockerRequestTimeout = 10 * time.Minute

func newDockerClient() (*docker.Client, error) {
	client, err := docker.NewVersionedClientFromEnv("1.41")
	if err != nil {
		return nil, err
	}
	client.SetTimeout(dockerRequestTimeout)
	return client, nil
}

func InitDocker() {
	var err error
	dockerClient, err = newDockerClient()
	if err != nil {
		slog.Warn("Failed to initialize docker client", "error", err)
		if os.Getenv("MODE") == "worker" {
			slog.Error("Worker mode requires Docker daemon access. Exiting.")
			os.Exit(1)
		}
	}
}

func getDockerClient() (*docker.Client, error) {
	if dockerClient != nil {
		return dockerClient, nil
	}
	var err error
	dockerClient, err = newDockerClient()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize docker client: %w", err)
	}
	return dockerClient, nil
}

func GetContainerLogs(ctx context.Context, containerID string, tail string) (string, error) {
	cli, err := getDockerClient()
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	opts := docker.LogsOptions{
		Context:      ctx,
		Container:    containerID,
		OutputStream: &buf,
		ErrorStream:  &buf,
		Stdout:       true,
		Stderr:       true,
		Tail:         tail,
	}

	err = cli.Logs(opts)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func createLog(entityID string, message string, level models.LogLevel) {
	log := models.Log{
		Message: message,
		Level:   level,
	}
	log.EnvironmentID = &entityID
	db.DB.Create(&log)
}

func clearPersistedGitHubTokenRewrite(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return nil
	}
	cmd := exec.Command("git", "-C", dir, "config", "--local", "--null", "--name-only", "--list")
	keys, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("inspect local Git config: %w", err)
	}
	for _, key := range strings.Split(string(keys), "\x00") {
		lowerKey := strings.ToLower(key)
		if !strings.HasPrefix(lowerKey, "url.https://x-access-token:") || !strings.Contains(lowerKey, "@github.com/.insteadof") {
			continue
		}
		if err := exec.Command("git", "-C", dir, "config", "--local", "--unset-all", key).Run(); err != nil {
			return fmt.Errorf("remove persisted GitHub credential rewrite")
		}
	}
	return nil
}

func CloneOrFetch(ctx context.Context, dir, gitURL, branch, baseCommit, githubToken string) error {
	// Keep the credential in process-scoped Git configuration. Unlike --local,
	// this cannot leave a token in .git/config when any operation fails.
	git := func(gitCtx context.Context, args ...string) error {
		cmd := exec.CommandContext(gitCtx, "git", append([]string{"-C", dir}, args...)...)
		if githubToken != "" {
			env := make([]string, 0, len(os.Environ())+3)
			for _, entry := range os.Environ() {
				key, _, _ := strings.Cut(entry, "=")
				if key == "GIT_CONFIG_COUNT" || key == "GIT_CONFIG_PARAMETERS" || strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_") {
					continue
				}
				env = append(env, entry)
			}
			cmd.Env = append(env,
				"GIT_CONFIG_COUNT=1",
				"GIT_CONFIG_KEY_0=url.https://x-access-token:"+githubToken+"@github.com/.insteadOf",
				"GIT_CONFIG_VALUE_0=https://github.com/",
			)
		}
		return cmd.Run()
	}
	if err := clearPersistedGitHubTokenRewrite(dir); err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		// Just fetch everything we might need
		if err := git(ctx, "fetch", "--all"); err != nil {
			return fmt.Errorf("git fetch failed: %w", err)
		}

		targetRef := branch
		if baseCommit != "" {
			targetRef = baseCommit
		}

		if err := git(ctx, "checkout", "-B", branch, targetRef); err != nil {
			return fmt.Errorf("git checkout failed: %w", err)
		}
		return nil
	}

	// For a fresh clone
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := git(ctx, "init"); err != nil {
		return err
	}
	if err := git(ctx, "remote", "add", "origin", gitURL); err != nil {
		return err
	}

	if err := git(ctx, "fetch", "--all"); err != nil {
		return fmt.Errorf("git fetch failed: %w", err)
	}

	targetRef := "FETCH_HEAD"
	if baseCommit != "" {
		targetRef = baseCommit
	} else {
		// Try to fetch specific branch if no base commit is provided
		if err := git(ctx, "fetch", "--depth", "1", "origin", branch); err == nil {
			targetRef = "origin/" + branch
		}
	}

	if err := git(ctx, "checkout", "-B", branch, targetRef); err != nil {
		return fmt.Errorf("git checkout failed: %w", err)
	}
	return nil
}

// ProvisionDevSandbox has been moved to warmpool.go
func CleanupContainer(ctx context.Context, containerID string) error {
	cli, err := getDockerClient()
	if err != nil {
		return err
	}
	if _, err := cli.InspectContainerWithContext(containerID, ctx); err != nil {
		var missing *docker.NoSuchContainer
		if errors.As(err, &missing) {
			return nil
		}
		return fmt.Errorf("inspect container %q before cleanup: %w", containerID, err)
	}
	stopErr := cli.StopContainerWithContext(containerID, 10, ctx)
	removeErr := cli.RemoveContainer(docker.RemoveContainerOptions{
		ID:      containerID,
		Force:   true,
		Context: ctx,
	})
	if removeErr != nil && strings.Contains(removeErr.Error(), "removal of container") && strings.Contains(removeErr.Error(), "already in progress") {
		removeErr = waitForContainerRemoval(ctx, cli, containerID, removeErr)
	}
	if removeErr != nil {
		slog.Error("Docker container removal failed", "container", containerID, "stop_error", stopErr, "remove_error", removeErr)
		return fmt.Errorf("remove container %q (stop error: %v): %w", containerID, stopErr, removeErr)
	}
	return nil
}

func waitForContainerRemoval(ctx context.Context, cli *docker.Client, containerID string, removeErr error) error {
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		_, inspectErr := cli.InspectContainerWithContext(containerID, waitCtx)
		if inspectErr != nil {
			var missing *docker.NoSuchContainer
			if errors.As(inspectErr, &missing) {
				return nil
			}
			if waitCtx.Err() != nil {
				return errors.Join(removeErr, waitCtx.Err())
			}
			return errors.Join(removeErr, fmt.Errorf("inspect container during removal: %w", inspectErr))
		}
		select {
		case <-waitCtx.Done():
			return errors.Join(removeErr, waitCtx.Err())
		case <-ticker.C:
		}
	}
}

// Helper to create tarball from a directory

func CleanupWorkspace(envID string) error {
	workspaceDir := GetWorkspacePath(envID)
	workspaceErr := os.RemoveAll(workspaceDir)
	if workspaceErr != nil {
		// Fallback to docker if permission denied
		slog.Warn("Workspace removal failed; trying scoped Docker cleanup", "dir", workspaceDir, "error", workspaceErr)

		if cli, clientErr := getDockerClient(); clientErr == nil {
			hostRoot := os.Getenv("HOST_WORKSPACES_DIR")
			if hostRoot == "" {
				hostRoot = GetWorkspacesRootDir()
			}
			hostWorkspaceDir := filepath.Join(hostRoot, envID)
			if err := os.MkdirAll(workspaceDir, 0755); err != nil {
				workspaceErr = errors.Join(workspaceErr, fmt.Errorf("create cleanup mount source: %w", err))
			} else if err := cli.PullImage(docker.PullImageOptions{Repository: "alpine:3.20"}, docker.AuthConfiguration{}); err != nil {
				workspaceErr = errors.Join(workspaceErr, fmt.Errorf("pull cleanup image: %w", err))
			} else {
				opts := docker.CreateContainerOptions{
					Config: &docker.Config{
						Image: "alpine:3.20",
						Cmd:   []string{"sh", "-c", "find /workspace -mindepth 1 -delete"},
					},
					HostConfig: &docker.HostConfig{
						NetworkMode: "none",
						Binds: []string{
							fmt.Sprintf("%s:/workspace", hostWorkspaceDir),
						},
						AutoRemove: true,
					},
				}
				container, cErr := cli.CreateContainer(opts)
				if cErr != nil {
					workspaceErr = errors.Join(workspaceErr, fmt.Errorf("create cleanup container: %w", cErr))
				} else if startErr := cli.StartContainer(container.ID, nil); startErr != nil {
					workspaceErr = errors.Join(workspaceErr, fmt.Errorf("start cleanup container: %w", startErr))
					_ = CleanupContainer(context.Background(), container.ID)
				} else {
					code, waitErr := cli.WaitContainerWithContext(container.ID, context.Background())
					if waitErr != nil {
						workspaceErr = errors.Join(workspaceErr, fmt.Errorf("wait for cleanup container: %w", waitErr))
					} else if code != 0 {
						workspaceErr = errors.Join(workspaceErr, fmt.Errorf("cleanup container exited with status %d", code))
					} else if removeErr := os.RemoveAll(hostWorkspaceDir); removeErr != nil {
						workspaceErr = errors.Join(workspaceErr, fmt.Errorf("remove workspace after scoped cleanup: %w", removeErr))
					}
					if cleanupErr := CleanupContainer(context.Background(), container.ID); cleanupErr != nil {
						workspaceErr = errors.Join(workspaceErr, fmt.Errorf("remove cleanup container: %w", cleanupErr))
					}
				}
			}
		} else {
			workspaceErr = errors.Join(workspaceErr, clientErr)
		}
	}
	cacheRoot := os.Getenv("HOST_CACHE_DIR")
	if cacheRoot == "" {
		cacheRoot = filepath.Join(GetWorkspacesRootDir(), ".cache")
	}
	if cacheErr := os.RemoveAll(filepath.Join(cacheRoot, envID)); cacheErr != nil {
		workspaceErr = errors.Join(workspaceErr, fmt.Errorf("remove environment cache: %w", cacheErr))
	}
	if workspaceErr != nil {
		slog.Error("Environment workspace cleanup incomplete", "environment_id", envID, "error", workspaceErr)
	}
	return workspaceErr
}

func RestartContainer(ctx context.Context, containerID string) error {
	return dockerClient.RestartContainer(containerID, 2)
}

func GetContainerPort(containerID string) (int, error) {
	inspect, err := dockerClient.InspectContainer(containerID)
	if err != nil {
		return 0, err
	}
	var assignedPort int
	for _, bindings := range inspect.NetworkSettings.Ports {
		if len(bindings) > 0 {
			port, _ := strconv.Atoi(bindings[0].HostPort)
			assignedPort = port
			break
		}
	}
	return assignedPort, nil
}

func WaitForAppReady(ctx context.Context, envID string, domain string) error {
	host := fmt.Sprintf("%s.%s", envID, domain)
	client := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			traefikURL := os.Getenv("TRAEFIK_URL")
			if traefikURL == "" {
				traefikURL = "http://api-sandbox-traefik"
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(traefikURL, "/")+"/", nil)
			if err != nil {
				continue
			}
			req.Host = host

			resp, err := client.Do(req)
			if err != nil {
				slog.Warn("WaitForAppReady HTTP error", "err", err)
				continue
			}

			statusCode := resp.StatusCode
			resp.Body.Close()

			slog.Info("WaitForAppReady HTTP response", "statusCode", statusCode)

			if IsHTTPReadyStatus(statusCode) {
				return nil
			}
		}
	}
}

func StartSidecarDatabase(ctx context.Context, envID string, userID string, dbType DBType) (string, error) {
	if dbType == DBTypeNone {
		return "", nil
	}

	networkName, _, err := EnsureUserNetwork(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to ensure network: %v", err)
	}

	containerName := fmt.Sprintf("api-sandbox-db-%s", envID)
	if err := CleanupContainer(ctx, containerName); err != nil {
		return "", fmt.Errorf("remove previous database sidecar %s: %w", containerName, err)
	}

	var image, dbURL string
	var env []string

	// Generate a secure random password for sidecar
	passwordBytes := make([]byte, 8)
	if _, err := rand.Read(passwordBytes); err != nil {
		return "", fmt.Errorf("generate database sidecar password: %w", err)
	}
	securePassword := hex.EncodeToString(passwordBytes)

	switch dbType {
	case DBTypeMySQL:
		image = "mysql:8.0"
		env = []string{
			"MYSQL_ROOT_PASSWORD=" + securePassword,
			"MYSQL_DATABASE=myapp",
			"MYSQL_USER=appuser",
			"MYSQL_PASSWORD=" + securePassword,
		}
		dbURL = fmt.Sprintf("mysql://appuser:%s@%s:3306/myapp", securePassword, containerName)
	case DBTypePostgres:
		image = "postgres:15"
		env = []string{
			"POSTGRES_DB=myapp",
			"POSTGRES_USER=appuser",
			"POSTGRES_PASSWORD=" + securePassword,
		}
		dbURL = fmt.Sprintf("postgresql://appuser:%s@%s:5432/myapp", securePassword, containerName)
	case DBTypeMongo:
		image = "mongo:6.0"
		env = []string{
			"MONGO_INITDB_DATABASE=myapp",
			"MONGO_INITDB_ROOT_USERNAME=admin",
			"MONGO_INITDB_ROOT_PASSWORD=" + securePassword,
		}
		dbURL = fmt.Sprintf("mongodb://admin:%s@%s:27017/myapp?authSource=admin", securePassword, containerName)
	}

	createLog(envID, fmt.Sprintf("Pulling %s database image (this may take a minute on first run)...", string(dbType)), models.LogLevelInfo)

	pullCtx, cancelPull := context.WithTimeout(ctx, 10*time.Minute)
	defer cancelPull()
	pullOpts := docker.PullImageOptions{Repository: image, Context: pullCtx, InactivityTimeout: time.Minute}
	if err := dockerClient.PullImage(pullOpts, docker.AuthConfiguration{}); err != nil {
		return "", fmt.Errorf("pull database image %s: %w", image, err)
	}

	createLog(envID, fmt.Sprintf("Starting sidecar database container (%s)...", containerName), models.LogLevelInfo)

	pidsLimit := int64(256)

	opts := docker.CreateContainerOptions{
		Name:    containerName,
		Context: ctx,
		Config: &docker.Config{
			Image: image,
			Env:   env,
		},
		HostConfig: &docker.HostConfig{
			Memory:      512 * 1024 * 1024,  // 512MB for DB
			MemorySwap:  1024 * 1024 * 1024, // 1GB Swap
			CPUQuota:    100000,
			CPUPeriod:   100000,
			CPUShares:   512,
			PidsLimit:   &pidsLimit,
			SecurityOpt: []string{"no-new-privileges:true"},
			CapDrop:     []string{"ALL"},
			CapAdd:      []string{"CHOWN", "SETUID", "SETGID", "DAC_OVERRIDE"}, // DBs usually need these to initialize
		},
		NetworkingConfig: &docker.NetworkingConfig{
			EndpointsConfig: map[string]*docker.EndpointConfig{
				networkName: {},
			},
		},
	}

	container, err := dockerClient.CreateContainer(opts)
	if err != nil {
		return "", fmt.Errorf("failed to create db container: %v", err)
	}

	if err := dockerClient.StartContainerWithContext(container.ID, nil, ctx); err != nil {
		cleanupErr := CleanupContainer(ctx, container.ID)
		return "", errors.Join(fmt.Errorf("failed to start db container: %w", err), cleanupErr)
	}

	createLog(envID, "Waiting for database to initialize and accept connections...", models.LogLevelInfo)

	err = waitForDatabaseReady(ctx, container.ID, dbType, envID, securePassword)
	if err != nil {
		cleanupErr := CleanupContainer(ctx, container.ID)
		return "", errors.Join(fmt.Errorf("database readiness check failed: %w", err), cleanupErr)
	}

	return dbURL, nil
}

func waitForDatabaseReady(ctx context.Context, containerID string, dbType DBType, envID string, securePassword string) error {
	var cmd []string
	switch dbType {
	case DBTypeMySQL:
		cmd = []string{"mysqladmin", "ping", "-h", "localhost", "-u", "root", "-p" + securePassword}
	case DBTypePostgres:
		cmd = []string{"pg_isready", "-U", "appuser", "-d", "myapp"}
	case DBTypeMongo:
		cmd = []string{"mongosh", "--quiet", "--eval", "db.adminCommand('ping')"}
	default:
		return nil
	}

	timeout := time.After(120 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timed out waiting for %s database to be ready", string(dbType))
		case <-ticker.C:
			execOpts := docker.CreateExecOptions{
				Container:    containerID,
				Context:      ctx,
				AttachStdout: true,
				AttachStderr: true,
				Cmd:          cmd,
			}
			exec, err := dockerClient.CreateExec(execOpts)
			if err != nil {
				continue
			}

			var stdout, stderr bytes.Buffer
			startOpts := docker.StartExecOptions{
				OutputStream: &stdout,
				ErrorStream:  &stderr,
				Context:      ctx,
			}
			err = dockerClient.StartExec(exec.ID, startOpts)
			if err != nil {
				continue
			}

			inspect, err := dockerClient.InspectExec(exec.ID)
			if err == nil && inspect.ExitCode == 0 {
				createLog(envID, fmt.Sprintf("Database %s is fully initialized and ready.", string(dbType)), models.LogLevelInfo)
				return nil
			}
		}
	}
}

func EnsureUserNetwork(ctx context.Context, userID string) (string, string, error) {
	networkName := fmt.Sprintf("api-sandbox-net-%s", userID)
	networks, err := dockerClient.ListNetworks()
	if err != nil {
		return "", "", fmt.Errorf("list Docker networks: %w", err)
	}
	var networkFound bool
	var networkID string
	for _, net := range networks {
		if net.Name == networkName {
			networkFound = true
			networkID = net.ID
			break
		}
	}

	if !networkFound {
		net, err := dockerClient.CreateNetwork(docker.CreateNetworkOptions{
			Name:           networkName,
			Driver:         "bridge",
			CheckDuplicate: true,
			EnableIPv6:     false,
		})
		if err != nil && err != docker.ErrNetworkAlreadyExists {
			return "", "", fmt.Errorf("failed to create network %s: %w", networkName, err)
		}
		if net != nil {
			networkID = net.ID
		} else if err == docker.ErrNetworkAlreadyExists {
			networks, err = dockerClient.ListNetworks()
			if err != nil {
				return "", "", fmt.Errorf("find concurrently created network %s: %w", networkName, err)
			}
			for _, existing := range networks {
				if existing.Name == networkName {
					networkID = existing.ID
					break
				}
			}
		}
	}
	if networkID == "" {
		return "", "", fmt.Errorf("Docker network %s has no ID after lookup/create", networkName)
	}

	// Traefik is intentionally attached to each user's network for routing.
	if err := connectContainerToNetwork(networkName, networkID, "api-sandbox-traefik"); err != nil {
		return "", "", fmt.Errorf("connect Traefik to user network %s: %w", networkName, err)
	}

	return networkName, networkID, nil
}

func connectContainerToNetwork(networkName, networkID, containerName string) error {
	info, err := dockerClient.InspectContainer(containerName)
	if err != nil {
		return err
	}
	if _, connected := info.NetworkSettings.Networks[networkName]; connected {
		return nil
	}
	return dockerClient.ConnectNetwork(networkID, docker.NetworkConnectionOptions{Container: containerName})
}

// TouchFileInContainer creates an empty file or updates the timestamp of a file
// inside the container namespace. This triggers native file watchers (like inotify)
// instantly, which host-side bind-mount writes sometimes fail to do reliably.
func TouchFileInContainer(ctx context.Context, envID string, filePath string) error {
	containerName := "api-sandbox-env-" + envID

	exec, err := dockerClient.CreateExec(docker.CreateExecOptions{
		Container:    containerName,
		Cmd:          []string{"touch", filePath},
		AttachStdout: false,
		AttachStderr: false,
		Context:      ctx,
	})
	if err != nil {
		return err
	}

	err = dockerClient.StartExec(exec.ID, docker.StartExecOptions{
		Detach:  true,
		Context: ctx,
	})
	return err
}

func ReapOrphanContainers(ctx context.Context) error {
	cli, err := getDockerClient()
	if err != nil {
		return err
	}
	containers, err := cli.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		return err
	}
	mainContainers := make(map[string]docker.APIContainers)
	var failures []error
	reconcileTime := time.Now()

	for _, c := range containers {
		for _, rawName := range c.Names {
			name := strings.TrimPrefix(rawName, "/")
			isMain := strings.HasPrefix(name, "api-sandbox-env-")
			isDB := strings.HasPrefix(name, "api-sandbox-db-")
			if !isMain && !isDB {
				continue
			}
			envID := strings.TrimPrefix(strings.TrimPrefix(name, "api-sandbox-env-"), "api-sandbox-db-")
			if isMain {
				mainContainers[envID] = c
			}

			var env models.Environment
			lookupErr := db.DB.Where("id = ?", envID).First(&env).Error
			if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
				lookupErr = nil
			} else if lookupErr != nil {
				failures = append(failures, fmt.Errorf("load environment %s for container %s: %w", envID, name, lookupErr))
				continue
			}
			orphan := lookupErr == nil && isOrphanEnvironment(env)
			deadRuntime := isMain && lookupErr == nil && env.Status == models.StatusRunning && needsRuntimeFailure(env.Status, true, c.State, env.UpdatedAt, reconcileTime)
			staleBuild := isMain && lookupErr == nil && env.Status == models.StatusBuilding && needsRuntimeFailure(env.Status, true, c.State, env.UpdatedAt, reconcileTime)
			if orphan || deadRuntime || staleBuild {
				slog.Info("Removing stale Docker container", "name", name, "environment_id", envID, "state", c.State, "orphan", orphan, "stale_build", staleBuild)
				if removeErr := CleanupContainer(ctx, c.ID); removeErr != nil {
					failures = append(failures, fmt.Errorf("remove stale container %s: %w", name, removeErr))
				}
				if isMain {
					if routeErr := ClearRuntimeRoute(ctx, envID); routeErr != nil {
						failures = append(failures, fmt.Errorf("clear stale Traefik route for %s: %w", envID, routeErr))
					}
				}
			}
			if deadRuntime || staleBuild {
				message := fmt.Sprintf("Runtime container was %s; marked environment failed by Docker reconciliation.", c.State)
				if staleBuild {
					message = "Environment remained BUILDING for over 60 minutes; marked failed by Docker reconciliation."
				}
				if dbErr := failReconciledEnvironment(envID, message); dbErr != nil {
					failures = append(failures, dbErr)
				}
			}
			if orphan && isMain {
				if cleanupErr := CleanupWorkspace(envID); cleanupErr != nil {
					failures = append(failures, fmt.Errorf("cleanup orphan workspace %s: %w", envID, cleanupErr))
				}
			}
		}
	}

	var activeEnvs []models.Environment
	if err := db.DB.Where("status IN ?", []models.EnvironmentStatus{models.StatusRunning, models.StatusBuilding}).Find(&activeEnvs).Error; err != nil {
		return errors.Join(append(failures, fmt.Errorf("list active environments for Docker reconciliation: %w", err))...)
	}
	for _, env := range activeEnvs {
		if _, exists := mainContainers[env.ID]; exists {
			continue
		}
		if needsRuntimeFailure(env.Status, false, "", env.UpdatedAt, reconcileTime) && env.Status == models.StatusRunning {
			if err := failReconciledEnvironment(env.ID, "Runtime container is missing; marked environment failed by Docker reconciliation."); err != nil {
				failures = append(failures, err)
			}
			if err := ClearRuntimeRoute(ctx, env.ID); err != nil {
				failures = append(failures, fmt.Errorf("clear missing-runtime Traefik route for %s: %w", env.ID, err))
			}
		} else if env.Status == models.StatusBuilding && needsRuntimeFailure(env.Status, false, "", env.UpdatedAt, reconcileTime) {
			if err := failReconciledEnvironment(env.ID, "Provisioning exceeded 60 minutes without a runtime container; marked environment failed by Docker reconciliation."); err != nil {
				failures = append(failures, err)
			}
			if err := ClearRuntimeRoute(ctx, env.ID); err != nil {
				failures = append(failures, fmt.Errorf("clear stale-build Traefik route for %s: %w", env.ID, err))
			}
		}
	}
	return errors.Join(failures...)
}

func isOrphanEnvironment(env models.Environment) bool {
	return env.ID == "" || (env.Status != models.StatusRunning && env.Status != models.StatusBuilding)
}

func needsRuntimeFailure(status models.EnvironmentStatus, containerFound bool, containerState string, updatedAt, now time.Time) bool {
	switch status {
	case models.StatusRunning:
		return !containerFound || containerState != "running"
	case models.StatusBuilding:
		return now.Sub(updatedAt) > 60*time.Minute
	default:
		return false
	}
}

func failReconciledEnvironment(envID, message string) error {
	updates := map[string]interface{}{"status": models.StatusFailed, "container_id": nil, "public_url": nil}
	if err := db.DB.Model(&models.Environment{}).Where("id = ? AND status IN ?", envID, []models.EnvironmentStatus{models.StatusRunning, models.StatusBuilding}).Updates(updates).Error; err != nil {
		return fmt.Errorf("mark environment %s failed after Docker reconciliation: %w", envID, err)
	}
	level := models.LogLevelError
	log := models.Log{EnvironmentID: &envID, Message: message, Level: level}
	if err := db.DB.Create(&log).Error; err != nil {
		return fmt.Errorf("write Docker reconciliation log for environment %s: %w", envID, err)
	}
	slog.Error(message, "environment_id", envID)
	return nil
}
