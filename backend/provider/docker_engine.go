package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
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
)

var dockerClient *docker.Client

func InitDocker() {
	var err error
	dockerClient, err = docker.NewVersionedClientFromEnv("1.41")
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
	dockerClient, err = docker.NewVersionedClientFromEnv("1.41")
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

func CloneOrFetch(ctx context.Context, dir, gitURL, branch, baseCommit, githubToken string) error {
	// Securely inject token via insteadOf if provided
	configToken := func() {
		if githubToken != "" {
			// Ensure it uses x-access-token
			exec.CommandContext(ctx, "git", "-C", dir, "config", "--local", "url.https://x-access-token:"+githubToken+"@github.com/.insteadOf", "https://github.com/").Run()
		}
	}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		configToken()
		// Just fetch everything we might need
		exec.CommandContext(ctx, "git", "-C", dir, "fetch", "--all").Run()
		
		targetRef := branch
		if baseCommit != "" {
			targetRef = baseCommit
		}
		
		return exec.CommandContext(ctx, "git", "-C", dir, "checkout", "-B", branch, targetRef).Run()
	}

	// For a fresh clone
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := exec.CommandContext(ctx, "git", "-C", dir, "init").Run(); err != nil {
		return err
	}
	configToken()
	if err := exec.CommandContext(ctx, "git", "-C", dir, "remote", "add", "origin", gitURL).Run(); err != nil {
		return err
	}

	exec.CommandContext(ctx, "git", "-C", dir, "fetch", "--all").Run()

	targetRef := "FETCH_HEAD"
	if baseCommit != "" {
		targetRef = baseCommit
	} else {
		// Try to fetch specific branch if no base commit is provided
		if err := exec.CommandContext(ctx, "git", "-C", dir, "fetch", "--depth", "1", "origin", branch).Run(); err == nil {
			targetRef = "origin/" + branch
		}
	}

	return exec.CommandContext(ctx, "git", "-C", dir, "checkout", "-B", branch, targetRef).Run()
}

// ProvisionDevSandbox has been moved to warmpool.go
func CleanupContainer(ctx context.Context, containerID string) error {
	_ = dockerClient.StopContainer(containerID, 10)
	return dockerClient.RemoveContainer(docker.RemoveContainerOptions{
		ID:    containerID,
		Force: true,
	})
}

// Helper to create tarball from a directory

func CleanupWorkspace(envID string) error {
	workspaceDir := GetWorkspacePath(envID)
	err := os.RemoveAll(workspaceDir)
	if err != nil {
		// Fallback to docker if permission denied
		slog.Warn("os.RemoveAll failed, trying docker rm -rf", "dir", workspaceDir, "error", err)
		
		hostWorkspacesDir := os.Getenv("HOST_WORKSPACES_DIR")
		if hostWorkspacesDir == "" {
			hostWorkspacesDir = GetWorkspacesRootDir()
		}
		
		opts := docker.CreateContainerOptions{
			Config: &docker.Config{
				Image: "alpine",
				Cmd:   []string{"rm", "-rf", fmt.Sprintf("/workspaces/%s", envID)},
			},
			HostConfig: &docker.HostConfig{
				Binds: []string{
					fmt.Sprintf("%s:/workspaces", hostWorkspacesDir),
				},
				AutoRemove: true,
			},
		}
		container, cErr := dockerClient.CreateContainer(opts)
		if cErr == nil {
			_ = dockerClient.StartContainer(container.ID, nil)
		}
	}
	return nil
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
	client := &http.Client{} // Removed 500ms timeout which aborted connections prematurely

	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timed out waiting for app %s to be reachable", envID)
		case <-ticker.C:
			traefikURL := os.Getenv("TRAEFIK_URL")
			if traefikURL == "" {
				traefikURL = "http://api-sandbox-traefik"
			}
			req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(traefikURL, "/")+"/", nil)
			if err != nil {
				continue
			}
			req.Host = host

			resp, err := client.Do(req)
			if err != nil {
				slog.Warn("WaitForAppReady HTTP error", "err", err)
				continue
			}

			// Must close immediately, not defer, to prevent connection leaks
			statusCode := resp.StatusCode
			resp.Body.Close()

			slog.Info("WaitForAppReady HTTP response", "statusCode", statusCode)

			if statusCode != http.StatusBadGateway {
				return nil
			}
		}
	}
}

func StartSidecarDatabase(ctx context.Context, envID string, orgID string, dbType DBType) (string, error) {
	if dbType == DBTypeNone {
		return "", nil
	}

	networkName, _, err := EnsureOrgNetwork(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to ensure network: %v", err)
	}

	containerName := fmt.Sprintf("api-sandbox-db-%s", envID)
	_ = CleanupContainer(ctx, containerName)

	var image, dbURL string
	var env []string

	// Generate a secure random password for sidecar
	passwordBytes := make([]byte, 8)
	rand.Read(passwordBytes)
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

	pullOpts := docker.PullImageOptions{
		Repository: image,
	}
	_ = dockerClient.PullImage(pullOpts, docker.AuthConfiguration{})

	createLog(envID, fmt.Sprintf("Starting sidecar database container (%s)...", containerName), models.LogLevelInfo)

	pidsLimit := int64(256)

	opts := docker.CreateContainerOptions{
		Name: containerName,
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

	if err := dockerClient.StartContainer(container.ID, nil); err != nil {
		return "", fmt.Errorf("failed to start db container: %v", err)
	}

	createLog(envID, "Waiting for database to initialize and accept connections...", models.LogLevelInfo)

	err = waitForDatabaseReady(ctx, container.ID, dbType, envID, securePassword)
	if err != nil {
		return "", fmt.Errorf("database readiness check failed: %v", err)
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

func EnsureOrgNetwork(ctx context.Context, orgID string) (string, string, error) {
	networkName := fmt.Sprintf("api-sandbox-net-%s", orgID)
	networks, err := dockerClient.ListNetworks()
	var networkFound bool
	var networkID string
	if err == nil {
		for _, net := range networks {
			if net.Name == networkName {
				networkFound = true
				networkID = net.ID
				break
			}
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
			return "", "", fmt.Errorf("failed to create network %s: %v", networkName, err)
		}
		if net != nil {
			networkID = net.ID
		}
	}

	// Ensure Traefik is connected to this network so it can route traffic to the sandbox
	_ = dockerClient.ConnectNetwork(networkID, docker.NetworkConnectionOptions{
		Container: "api-sandbox-traefik",
	})

	return networkName, networkID, nil
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
	containers, err := dockerClient.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		return err
	}

	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		var envID string
		isEnv := strings.HasPrefix(name, "api-sandbox-env-")
		isDB := strings.HasPrefix(name, "api-sandbox-db-")

		if isEnv {
			envID = strings.TrimPrefix(name, "api-sandbox-env-")
		} else if isDB {
			envID = strings.TrimPrefix(name, "api-sandbox-db-")
		} else {
			continue
		}

		var env models.Environment
		err := db.DB.Where("id = ?", envID).First(&env).Error

		// We reap if it's not in the DB, OR if the DB says it's STOPPED/FAILED
		shouldReap := false
		if err != nil {
			shouldReap = true
		} else if env.Status != models.StatusRunning && env.Status != models.StatusBuilding {
			shouldReap = true
		}

		if shouldReap {
			slog.Info("Reaper: Removing orphan container", "name", name, "env_id", envID)
			_ = CleanupContainer(ctx, name)
			if isEnv {
				_ = CleanupWorkspace(envID)
			}
		}
	}
	return nil
}
