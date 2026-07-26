package worker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	docker "github.com/fsouza/go-dockerclient"
)

var dockerClient *docker.Client

func InitDocker() {
	var err error
	dockerClient, err = docker.NewVersionedClientFromEnv("1.41")
	if err != nil {
		log.Fatalf("Failed to initialize docker client: %v", err)
	}
}

func CloneAndBuildImage(ctx context.Context, envID string, gitURL string, branch string) (string, error) {
	tmpDir := filepath.Join(os.TempDir(), "api-sandbox", envID)
	imageTag := fmt.Sprintf("api-sandbox-%s", strings.ToLower(envID))

	db.DB.Create(&models.Log{
		EnvironmentID: envID,
		Message:       fmt.Sprintf("Cloning repository %s (branch: %s)...", gitURL, branch),
		Level:         models.LogLevelInfo,
	})

	// 1. Clone Repo
	// Ensure temp directory is clean before cloning
	_ = os.RemoveAll(tmpDir)
	
	// Try cloning with the specified branch first
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", branch, gitURL, tmpDir)
	if _, err := cmd.CombinedOutput(); err != nil {
		// If it fails (likely due to branch not found), try again without specifying a branch (uses repository default, e.g. master)
		db.DB.Create(&models.Log{
			EnvironmentID: envID,
			Message:       fmt.Sprintf("Branch '%s' not found, falling back to default branch...", branch),
			Level:         models.LogLevelWarn,
		})
		
		cmd = exec.CommandContext(ctx, "git", "clone", "--depth", "1", gitURL, tmpDir)
		if out2, err2 := cmd.CombinedOutput(); err2 != nil {
			return "", fmt.Errorf("git clone failed: %s - %v", string(out2), err2)
		}
	}
	defer os.RemoveAll(tmpDir) // Cleanup

	db.DB.Create(&models.Log{
		EnvironmentID: envID,
		Message:       fmt.Sprintf("Building Docker image %s...", imageTag),
		Level:         models.LogLevelInfo,
	})

	// Pre-build analysis: Check if Dockerfile exists
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	dockerfileInfo, err := os.Stat(dockerfilePath)
	hasDockerfile := err == nil && !dockerfileInfo.IsDir()

	if !hasDockerfile {
		// Use Nixpacks
		db.DB.Create(&models.Log{
			EnvironmentID: envID,
			Message:       "No Dockerfile found. Generating build plan using Nixpacks...",
			Level:         models.LogLevelInfo,
		})

		nixpacksPath := "nixpacks"
		if home, err := os.UserHomeDir(); err == nil {
			localBin := filepath.Join(home, ".local", "bin", "nixpacks")
			if _, err := os.Stat(localBin); err == nil {
				nixpacksPath = localBin
			}
		}

		cmd := exec.CommandContext(ctx, nixpacksPath, "build", tmpDir, "--out", tmpDir, "--no-error-without-start")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("nixpacks build failed: %s - %v", string(out), err)
		}

		db.DB.Create(&models.Log{
			EnvironmentID: envID,
			Message:       "Nixpacks build plan generated successfully.",
			Level:         models.LogLevelInfo,
		})

		// Move .nixpacks/Dockerfile to root Dockerfile so we don't need to specify opts.Dockerfile
		err = os.Rename(filepath.Join(tmpDir, ".nixpacks", "Dockerfile"), filepath.Join(tmpDir, "Dockerfile"))
		if err != nil {
			return "", fmt.Errorf("failed to move Nixpacks Dockerfile: %v", err)
		}
		dockerfilePath = filepath.Join(tmpDir, "Dockerfile")
	}

	if content, err := os.ReadFile(dockerfilePath); err == nil {
		if !strings.Contains(strings.ToUpper(string(content)), "EXPOSE ") {
			// Append EXPOSE 5000 as a fallback so PublishAllPorts works
			newContent := string(content) + "\n# Auto-injected by API Sandbox\nEXPOSE 5000\n"
			os.WriteFile(dockerfilePath, []byte(newContent), 0644)
			db.DB.Create(&models.Log{
				EnvironmentID: envID,
				Message:       "No EXPOSE instruction found in Dockerfile. Auto-injecting 'EXPOSE 5000'...",
				Level:         models.LogLevelWarn,
			})
		}
	}

	// 2. Tar the directory for build context
	tarStream, err := tarballDir(tmpDir)
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

	if err := dockerClient.BuildImage(opts); err != nil {
		return "", fmt.Errorf("failed to build image: %v", err)
	}

	// Log output
	db.DB.Create(&models.Log{
		EnvironmentID: envID,
		Message:       buf.String(),
		Level:         models.LogLevelInfo,
	})

	db.DB.Create(&models.Log{
		EnvironmentID: envID,
		Message:       "Image built successfully.",
		Level:         models.LogLevelInfo,
	})

	return imageTag, nil
}

func StartContainer(ctx context.Context, envID string, imageTag string) (string, int, error) {
	db.DB.Create(&models.Log{
		EnvironmentID: envID,
		Message:       fmt.Sprintf("Starting container for image %s...", imageTag),
		Level:         models.LogLevelInfo,
	})

	// Pre-cleanup in case a zombie container with this name exists from a previous failed run
	_ = CleanupContainer(ctx, fmt.Sprintf("api-sandbox-env-%s", envID))

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

	opts := docker.CreateContainerOptions{
		Name: fmt.Sprintf("api-sandbox-env-%s", envID),
		Config: &docker.Config{
			Image: imageTag,
			Env: []string{
				fmt.Sprintf("PORT=%s", exposedPort),
				"HOST=0.0.0.0",
			},
			Labels: map[string]string{
				"traefik.enable": "true",
				fmt.Sprintf("traefik.http.routers.env-%s.rule", envID):                      fmt.Sprintf("Host(`%s.localhost`)", envID),
				fmt.Sprintf("traefik.http.services.env-%s.loadbalancer.server.port", envID): exposedPort,
			},
		},
		HostConfig: &docker.HostConfig{
			Memory:          512 * 1024 * 1024,
			CPUShares:       512,
			PublishAllPorts: true,
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

	db.DB.Create(&models.Log{
		EnvironmentID: envID,
		Message:       fmt.Sprintf("Container started successfully on port %d (Container ID: %s).", assignedPort, container.ID[:12]),
		Level:         models.LogLevelInfo,
	})

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
