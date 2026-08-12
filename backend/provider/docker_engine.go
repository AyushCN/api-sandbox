package provider

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
		slog.Error("Failed to initialize docker client", "error", err)
		os.Exit(1)
	}
}

func createLog(entityID string, entityType string, message string, level models.LogLevel) {
	log := models.Log{
		Message: message,
		Level:   level,
	}
	if entityType == "deployment" {
		log.DeploymentID = &entityID
	} else {
		log.EnvironmentID = &entityID
	}
	db.DB.Create(&log)
}

type buildTiming struct {
	Stage    string
	Duration time.Duration
}

func recordBenchmark(id, kind string, timings []buildTiming, totalDuration time.Duration, imageTag, repo string) {
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
			DeploymentID:   id,
			Repo:           repo,
			Condition:      condition,
			Stage:          t.Stage,
			DurationMs:     t.Duration.Milliseconds(),
			ImageSizeBytes: imageSizeBytes,
		}
		db.DB.Create(&run)
	}

	totalRun := models.BenchmarkRun{
		DeploymentID:   id,
		Repo:           repo,
		Condition:      condition,
		Stage:          "total",
		DurationMs:     totalDuration.Milliseconds(),
		ImageSizeBytes: imageSizeBytes,
	}
	db.DB.Create(&totalRun)
}

func cloneOrFetch(ctx context.Context, dir, gitURL, branch string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		exec.CommandContext(ctx, "git", "-C", dir, "fetch", "--depth", "1", "origin", branch).Run()
		return exec.CommandContext(ctx, "git", "-C", dir, "reset", "--hard", "origin/"+branch).Run()
	}
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", branch, gitURL, dir)
	if _, err := cmd.CombinedOutput(); err != nil {
		cmd = exec.CommandContext(ctx, "git", "clone", "--depth", "1", gitURL, dir)
		if out2, err2 := cmd.CombinedOutput(); err2 != nil {
			return fmt.Errorf("git clone failed: %s - %v", string(out2), err2)
		}
	}
	return nil
}

func CloneAndBuildImage(ctx context.Context, envID string, entityType string, gitURL string, branch string) (string, error) {
	var timings []buildTiming
	start := time.Now()

	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %v", err)
	}
	workspacesRoot := filepath.Join(wd, "workspaces")
	_ = os.MkdirAll(workspacesRoot, 0755)
	tmpDir := filepath.Join(workspacesRoot, envID)
	imageTag := fmt.Sprintf("api-sandbox-%s", strings.ToLower(envID))

	subDir := ""
	gitURL = strings.TrimSuffix(gitURL, "/")

	re := regexp.MustCompile(`^https://github\.com/([^/]+)/([^/]+)(?:/tree/([^/]+)/(.*))?$`)
	matches := re.FindStringSubmatch(gitURL)
	if len(matches) == 5 && matches[3] != "" {
		gitURL = fmt.Sprintf("https://github.com/%s/%s", matches[1], strings.TrimSuffix(matches[2], ".git"))
		branch = matches[3]
		subDir = matches[4]
	}

	repoForBenchmark := gitURL

	createLog(envID, entityType, fmt.Sprintf("Cloning repository %s (branch: %s, subdir: %s)...", gitURL, branch, subDir), models.LogLevelInfo)

	// clone/fetch stage
	t0 := time.Now()

	// 1. Clone Repo
	if err := cloneOrFetch(ctx, tmpDir, gitURL, branch); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", err
	}
	timings = append(timings, buildTiming{"clone", time.Since(t0)})

	createLog(envID, entityType, fmt.Sprintf("Building Docker image %s...", imageTag), models.LogLevelInfo)

	// detect stage
	t1 := time.Now()

	// Set the build directory to the subdirectory if specified
	buildDir := tmpDir
	if subDir != "" {
		buildDir = filepath.Clean(filepath.Join(tmpDir, subDir))

		// Prevent path traversal
		if !strings.HasPrefix(buildDir, filepath.Clean(tmpDir)+string(os.PathSeparator)) && buildDir != filepath.Clean(tmpDir) {
			errMsg := fmt.Sprintf("Invalid subdirectory path: %s", subDir)
			createLog(envID, entityType, errMsg, models.LogLevelError)
			return "", fmt.Errorf("%s", errMsg)
		}

		// Check if the subdirectory actually exists in the cloned repo
		if info, err := os.Stat(buildDir); os.IsNotExist(err) || !info.IsDir() {
			errMsg := fmt.Sprintf("Subdirectory '%s' does not exist in the repository.", subDir)
			createLog(envID, entityType, errMsg, models.LogLevelError)
			return "", fmt.Errorf("%s", errMsg)
		}
	}

	// Pre-build analysis: Check if Dockerfile exists in the build directory
	dockerfilePath := filepath.Join(buildDir, "Dockerfile")
	dockerfileInfo, err := os.Stat(dockerfilePath)
	hasDockerfile := err == nil && !dockerfileInfo.IsDir()

	if !hasDockerfile {
		// Use Buildpack Manager to auto-detect and generate a Dockerfile
		manager := NewBuildpackManager()
		bp, bpErr := manager.Detect(buildDir)

		if bpErr == nil {
			createLog(envID, entityType, "Detected language buildpack. Generating optimized Dockerfile...", models.LogLevelInfo)

			_, buildErr := bp.Build(ctx, buildDir)
			if buildErr != nil {
				return "", fmt.Errorf("buildpack generation failed: %v", buildErr)
			}
			dockerfilePath = filepath.Join(buildDir, "Dockerfile")
		} else {
			// Fallback to Nixpacks
			createLog(envID, entityType, "No supported buildpack found. Generating build plan using Nixpacks...", models.LogLevelInfo)

			nixpacksPath := "nixpacks"
			if home, err := os.UserHomeDir(); err == nil {
				localBin := filepath.Join(home, ".local", "bin", "nixpacks")
				if _, err := os.Stat(localBin); err == nil {
					nixpacksPath = localBin
				}
			}

			cmd := exec.CommandContext(ctx, nixpacksPath, "build", buildDir, "--out", buildDir, "--no-error-without-start")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return "", fmt.Errorf("nixpacks build failed: %s - %v", string(out), err)
			}

			createLog(envID, entityType, "Nixpacks build plan generated successfully.", models.LogLevelInfo)

			// Move .nixpacks/Dockerfile to root Dockerfile so we don't need to specify opts.Dockerfile
			err = os.Rename(filepath.Join(buildDir, ".nixpacks", "Dockerfile"), filepath.Join(buildDir, "Dockerfile"))
			if err != nil {
				return "", fmt.Errorf("failed to move Nixpacks Dockerfile: %v", err)
			}
			dockerfilePath = filepath.Join(buildDir, "Dockerfile")
		}
	}

	if content, err := os.ReadFile(dockerfilePath); err == nil {
		if !strings.Contains(strings.ToUpper(string(content)), "EXPOSE ") {
			// Append EXPOSE 5000 as a fallback so PublishAllPorts works
			newContent := string(content) + "\n# Auto-injected by API Sandbox\nEXPOSE 5000\n"
			os.WriteFile(dockerfilePath, []byte(newContent), 0644)
			createLog(envID, entityType, "No EXPOSE instruction found in Dockerfile. Auto-injecting 'EXPOSE 5000'...", models.LogLevelWarn)
		}
	}

	timings = append(timings, buildTiming{"detect", time.Since(t1)})

	// build stage
	t2 := time.Now()
	// 2. Tar the directory for build context
	tarStream, err := tarballDir(buildDir)
	if err != nil {
		return "", fmt.Errorf("failed to tar build context: %v", err)
	}

	// 3. Build Image
	buf := new(bytes.Buffer)
	opts := docker.BuildImageOptions{
		Name:         imageTag,
		InputStream:  tarStream,
		OutputStream: buf,
		ContextDir:   "", // Root of the tarball
		Version:      docker.BuilderBuildKit,
	}

	if os.Getenv("BERTH_CACHE_MODE") == "warm" {
		opts.CacheFrom = []string{imageTag}
		opts.BuildArgs = []docker.BuildArg{
			{Name: "BUILDKIT_INLINE_CACHE", Value: "1"},
		}
	}

	if err := dockerClient.BuildImage(opts); err != nil {
		return "", fmt.Errorf("failed to build image: %v", err)
	}

	timings = append(timings, buildTiming{"build", time.Since(t2)})

	// Log output
	createLog(envID, entityType, buf.String(), models.LogLevelInfo)

	createLog(envID, entityType, "Image built successfully.", models.LogLevelInfo)

	recordBenchmark(envID, entityType, timings, time.Since(start), imageTag, repoForBenchmark)

	return imageTag, nil
}

func StartContainer(ctx context.Context, envID string, entityType string, imageTag string, orgID string, dbURL string) (string, int, error) {
	createLog(envID, entityType, fmt.Sprintf("Starting container for image %s...", imageTag), models.LogLevelInfo)

	// Pre-cleanup in case a zombie container with this name exists from a previous failed run
	_ = CleanupContainer(ctx, fmt.Sprintf("api-sandbox-env-%s", envID), entityType)

	// Inspect image to find the exposed port
	imageInfo, err := dockerClient.InspectImage(imageTag)
	var exposedPort string
	if err == nil && imageInfo.Config != nil {
		for port := range imageInfo.Config.ExposedPorts {
			exposedPort = port.Port()
			break
		}
	}
	if exposedPort == "" {
		exposedPort = "5000"
	}

	domain := os.Getenv("DOMAIN")
	if domain == "" {
		domain = "localhost"
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
		createLog(envID, entityType, err.Error(), models.LogLevelError)
		return "", 0, err
	}

	// Always ensure Traefik proxy is connected to this user's network for routing
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
			Image: imageTag,
			Env: func() []string {
				e := []string{fmt.Sprintf("PORT=%s", exposedPort), "HOST=0.0.0.0"}
				if dbURL != "" {
					e = append(e, fmt.Sprintf("DATABASE_URL=%s", dbURL), fmt.Sprintf("MONGO_URI=%s", dbURL))
					if u, err := url.Parse(dbURL); err == nil {
						e = append(e, fmt.Sprintf("DB_HOST=%s", u.Hostname()))
						if u.Port() != "" {
							e = append(e, fmt.Sprintf("DB_PORT=%s", u.Port()))
						}
						if u.User != nil {
							e = append(e, fmt.Sprintf("DB_USER=%s", u.User.Username()))
							if p, ok := u.User.Password(); ok {
								e = append(e, fmt.Sprintf("DB_PASSWORD=%s", p))
							}
						}
						dbName := strings.TrimPrefix(u.Path, "/")
						if dbName != "" {
							e = append(e, fmt.Sprintf("DB_NAME=%s", dbName))
						}
					}
				}
				return e
			}(),
			Labels: labels,
		},
		HostConfig: &docker.HostConfig{
			Memory:          512 * 1024 * 1024,
			MemorySwap:      -1,
			CPUQuota:        100000,
			CPUPeriod:       100000,
			CPUShares:       1024,
			PidsLimit:       &pidsLimit,
			RestartPolicy:   docker.RestartOnFailure(3),
			PublishAllPorts: true,
			SecurityOpt:     []string{"no-new-privileges:true"},
			CapDrop:         []string{"ALL"},
			CapAdd:          []string{"NET_BIND_SERVICE", "CHOWN", "SETUID", "SETGID", "DAC_OVERRIDE"},
			Binds: []string{
				fmt.Sprintf("%s:/app", workspaceDir),
				"/app/node_modules",
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

	// Inspect to get dynamic port
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

	if assignedPort == 0 {
		return container.ID, 0, fmt.Errorf("container started but no ports were mapped")
	}

	createLog(envID, entityType, fmt.Sprintf("Container started successfully on port %d (Container ID: %s).", assignedPort, container.ID[:12]), models.LogLevelInfo)

	return container.ID, assignedPort, nil
}

func CleanupContainer(ctx context.Context, containerID string, entityType string) error {
	_ = dockerClient.StopContainer(containerID, 10)
	return dockerClient.RemoveContainer(docker.RemoveContainerOptions{
		ID:    containerID,
		Force: true,
	})
}

// Helper to create tarball from a directory
func tarballDir(src string) (io.Reader, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	err := filepath.Walk(src, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}
		header.Name = strings.TrimPrefix(strings.Replace(file, src, "", -1), string(filepath.Separator))
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})

	if err != nil {
		return nil, err
	}
	return buf, nil
}

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

func StartSidecarDatabase(ctx context.Context, envID string, entityType string, orgID string, dbType DBType) (string, error) {
	if dbType == DBTypeNone {
		return "", nil
	}

	networkName, _, err := EnsureOrgNetwork(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to ensure network: %v", err)
	}

	containerName := fmt.Sprintf("api-sandbox-db-%s", envID)
	_ = CleanupContainer(ctx, containerName, entityType)

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

	createLog(envID, entityType, fmt.Sprintf("Pulling %s database image (this may take a minute on first run)...", string(dbType)), models.LogLevelInfo)

	pullOpts := docker.PullImageOptions{
		Repository: image,
	}
	_ = dockerClient.PullImage(pullOpts, docker.AuthConfiguration{})

	createLog(envID, entityType, fmt.Sprintf("Starting sidecar database container (%s)...", containerName), models.LogLevelInfo)

	opts := docker.CreateContainerOptions{
		Name: containerName,
		Config: &docker.Config{
			Image: image,
			Env:   env,
		},
		HostConfig: &docker.HostConfig{
			Memory: 256 * 1024 * 1024, // 256MB for DB
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

	createLog(envID, entityType, "Waiting for database to initialize and accept connections...", models.LogLevelInfo)

	err = waitForDatabaseReady(ctx, container.ID, dbType, envID, entityType)
	if err != nil {
		return "", fmt.Errorf("database readiness check failed: %v", err)
	}

	return dbURL, nil
}

func waitForDatabaseReady(ctx context.Context, containerID string, dbType DBType, envID string, entityType string) error {
	var cmd []string
	switch dbType {
	case DBTypeMySQL:
		cmd = []string{"mysqladmin", "ping", "-h", "localhost", "-u", "root", "-prootpass123"}
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
				createLog(envID, entityType, fmt.Sprintf("Database %s is fully initialized and ready.", string(dbType)), models.LogLevelInfo)
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
	return networkName, networkID, nil
}
