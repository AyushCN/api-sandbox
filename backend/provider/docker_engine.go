package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"net/url"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	docker "github.com/fsouza/go-dockerclient"
)

var dockerClient *docker.Client

func InitDocker() {
	var err error
	dockerClient, err = docker.NewVersionedClientFromEnv("1.41")
	if err != nil {
		slog.Error("Failed to initialize docker client", "error", err)
		os.Exit(1)
	}
}

func createLog(entityID string, message string, level models.LogLevel) {
	log := models.Log{
		Message: message,
		Level:   level,
	}
	log.EnvironmentID = &entityID
	db.DB.Create(&log)
}

type buildTiming struct {
	Stage    string
	Duration time.Duration
}

func recordBenchmark(id string, timings []buildTiming, totalDuration time.Duration, imageTag, repo string) {
	condition := os.Getenv("BERTH_CACHE_MODE")
	if condition == "" {
		condition = "cold"
	}

	var imageSizeBytes int64
	if inspect, err := dockerClient.InspectImage(imageTag); err == nil {
		imageSizeBytes = inspect.Size
	}

	for _, t := range timings {
		run := models.BenchmarkRun{
			EnvironmentID:  id,
			Repo:           repo,
			Condition:      condition,
			Stage:          t.Stage,
			DurationMs:     t.Duration.Milliseconds(),
			ImageSizeBytes: imageSizeBytes,
		}
		db.DB.Create(&run)
	}

	totalRun := models.BenchmarkRun{
		EnvironmentID:  id,
		Repo:           repo,
		Condition:      condition,
		Stage:          "total",
		DurationMs:     totalDuration.Milliseconds(),
		ImageSizeBytes: imageSizeBytes,
	}
	db.DB.Create(&totalRun)
}

func CloneOrFetch(ctx context.Context, dir, gitURL, branch, githubToken string) error {
	// Securely inject token via insteadOf if provided
	configToken := func() {
		if githubToken != "" {
			// Ensure it uses x-access-token
			exec.CommandContext(ctx, "git", "-C", dir, "config", "--local", "url.https://x-access-token:"+githubToken+"@github.com/.insteadOf", "https://github.com/").Run()
		}
	}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		configToken()
		exec.CommandContext(ctx, "git", "-C", dir, "fetch", "--depth", "1", "origin", branch).Run()
		return exec.CommandContext(ctx, "git", "-C", dir, "reset", "--hard", "origin/"+branch).Run()
	}

	// For a fresh clone, if private, we need the token. The safest way without leaking it in the process list
	// is to init, config, and then fetch/checkout.
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

	fetchCmd := exec.CommandContext(ctx, "git", "-C", dir, "fetch", "--depth", "1", "origin", branch)
	if out, err := fetchCmd.CombinedOutput(); err != nil {
		// Fallback to fetch all if branch isn't found
		fetchCmd = exec.CommandContext(ctx, "git", "-C", dir, "fetch", "--depth", "1", "origin")
		if out2, err2 := fetchCmd.CombinedOutput(); err2 != nil {
			return fmt.Errorf("git fetch failed: %s - %v (fallback: %s - %v)", string(out), err, string(out2), err2)
		}
		// Checkout default branch (whatever was fetched)
		return exec.CommandContext(ctx, "git", "-C", dir, "checkout", "FETCH_HEAD").Run()
	}
	return exec.CommandContext(ctx, "git", "-C", dir, "checkout", branch).Run()
}

func ProvisionDevSandbox(ctx context.Context, envID string, config DevRuntimeConfig, orgID string, dbURL string) (string, int, error) {
	createLog(envID, fmt.Sprintf("Provisioning Dev Sandbox (Image: %s)...", config.BaseImage), models.LogLevelInfo)

	_ = CleanupContainer(ctx, fmt.Sprintf("api-sandbox-env-%s", envID))

	// Pull image if not exists
	err := dockerClient.PullImage(docker.PullImageOptions{
		Repository: config.BaseImage,
	}, docker.AuthConfiguration{})
	if err != nil {
		createLog(envID, fmt.Sprintf("Pulling base image %s...", config.BaseImage), models.LogLevelInfo)
	}

	domain := os.Getenv("DOMAIN")
	if domain == "" {
		domain = "localhost"
	}

	exposedPort := config.ExposedPort
	if exposedPort == "" {
		exposedPort = "5000" // Fallback
	}

	labels := map[string]string{
		"traefik.enable": "true",
		fmt.Sprintf("traefik.http.routers.env-%s.rule", envID):                      fmt.Sprintf("Host(`%s.%s`)", envID, domain),
		fmt.Sprintf("traefik.http.services.env-%s.loadbalancer.server.port", envID): exposedPort,
		"traefik.docker.network": fmt.Sprintf("api-sandbox-net-%s", orgID),
	}

	if domain != "localhost" {
		labels[fmt.Sprintf("traefik.http.routers.env-%s.entrypoints", envID)] = "websecure"
		labels[fmt.Sprintf("traefik.http.routers.env-%s.tls.certresolver", envID)] = "myresolver"
	} else {
		labels[fmt.Sprintf("traefik.http.routers.env-%s.entrypoints", envID)] = "web"
	}

	networkName, networkID, err := EnsureOrgNetwork(ctx, orgID)
	if err != nil {
		createLog(envID, err.Error(), models.LogLevelError)
		return "", 0, err
	}

	if networkID != "" {
		_ = dockerClient.ConnectNetwork(networkID, docker.NetworkConnectionOptions{
			Container: "traefik-proxy",
		})
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", 0, fmt.Errorf("failed to get working directory: %v", err)
	}
	workspaceDir := filepath.Join(wd, "workspaces", envID)

	pidsLimit := int64(256)
	opts := docker.CreateContainerOptions{
		Name: fmt.Sprintf("api-sandbox-env-%s", envID),
		Config: &docker.Config{
			Image:      config.BaseImage,
			WorkingDir: config.WorkDir,
			Env: func() []string {
				e := []string{fmt.Sprintf("PORT=%s", exposedPort), "HOST=0.0.0.0"}
				if dbURL != "" {
					e = append(e, fmt.Sprintf("DATABASE_URL=%s", dbURL), fmt.Sprintf("MONGO_URI=%s", dbURL))
					if u, err := url.Parse(dbURL); err == nil {
						e = append(e, fmt.Sprintf("DB_HOST=%s", u.Hostname()))
						e = append(e, fmt.Sprintf("DB_PORT=%s", u.Port()))
						e = append(e, fmt.Sprintf("DB_USER=%s", u.User.Username()))
						if pwd, ok := u.User.Password(); ok {
							e = append(e, fmt.Sprintf("DB_PASSWORD=%s", pwd))
						}
						e = append(e, fmt.Sprintf("DB_NAME=%s", strings.TrimPrefix(u.Path, "/")))
					}
				}
				return e
			}(),
			Labels: labels,
			Cmd:    []string{"/bin/sh", "sandbox-start.sh"},
		},
		HostConfig: &docker.HostConfig{
			Memory:          512 * 1024 * 1024,
			MemorySwap:      512 * 1024 * 1024,
			CPUQuota:        100000,
			CPUPeriod:       100000,
			CPUShares:       1024,
			PidsLimit:       &pidsLimit,
			RestartPolicy:   docker.RestartOnFailure(3),
			SecurityOpt:     []string{"no-new-privileges:true"},
			CapDrop:         []string{"ALL"},
			Binds: []string{
				fmt.Sprintf("%s:%s", workspaceDir, config.WorkDir),
			},
		},
		NetworkingConfig: &docker.NetworkingConfig{
			EndpointsConfig: map[string]*docker.EndpointConfig{
				networkName: {},
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

	inspect, err := dockerClient.InspectContainer(container.ID)
	if err != nil {
		return container.ID, 0, fmt.Errorf("failed to inspect container: %v", err)
	}

	var assignedPort int
	for _, bindings := range inspect.NetworkSettings.Ports {
		if len(bindings) > 0 {
			port, _ := strconv.Atoi(bindings[0].HostPort)
			assignedPort = port
			break
		}
	}

	createLog(envID, fmt.Sprintf("Dev Sandbox started successfully on port %d (Container ID: %s).", assignedPort, container.ID[:12]), models.LogLevelInfo)

	return container.ID, assignedPort, nil
}

func CleanupContainer(ctx context.Context, containerID string) error {
	_ = dockerClient.StopContainer(containerID, 10)
	return dockerClient.RemoveContainer(docker.RemoveContainerOptions{
		ID:    containerID,
		Force: true,
	})
}

// Helper to create tarball from a directory

func CleanupWorkspace(envID string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	workspaceDir := filepath.Join(wd, "workspaces", envID)
	return os.RemoveAll(workspaceDir)
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
			Memory:      256 * 1024 * 1024, // 256MB for DB
			MemorySwap:  256 * 1024 * 1024,
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
