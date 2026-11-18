package provider

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

func TestCloneOrFetchDoesNotPersistGitCredential(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	seed := filepath.Join(root, "seed")
	runGit := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	if out, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %v (%s)", err, out)
	}
	if err := os.MkdirAll(seed, 0755); err != nil {
		t.Fatal(err)
	}
	runGit(seed, "init")
	runGit(seed, "config", "user.email", "test@example.invalid")
	runGit(seed, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(seed, "README"), []byte("test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(seed, "add", "README")
	runGit(seed, "commit", "-m", "initial")
	runGit(seed, "branch", "-M", "main")
	runGit(seed, "remote", "add", "origin", remote)
	runGit(seed, "push", "origin", "main")

	for _, tc := range []struct {
		name, remote string
		wantErr      bool
	}{
		{name: "success", remote: remote},
		{name: "fetch failure", remote: filepath.Join(root, "missing.git"), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(root, strings.ReplaceAll(tc.name, " ", "-"))
			err := CloneOrFetch(context.Background(), dir, tc.remote, "main", "", "test-token-must-not-persist")
			if (err != nil) != tc.wantErr {
				t.Fatalf("CloneOrFetch error = %v, wantErr %v", err, tc.wantErr)
			}
			runGit(dir, "config", "--local", "url.https://x-access-token:stale-token@github.com/.insteadOf", "https://github.com/")
			err = CloneOrFetch(context.Background(), dir, tc.remote, "main", "", "test-token-must-not-persist")
			if (err != nil) != tc.wantErr {
				t.Fatalf("CloneOrFetch with stale credential error = %v, wantErr %v", err, tc.wantErr)
			}
			config, readErr := os.ReadFile(filepath.Join(dir, ".git", "config"))
			if readErr != nil {
				t.Fatalf("read local Git config: %v", readErr)
			}
			if strings.Contains(string(config), "token") || strings.Contains(string(config), "insteadOf") {
				t.Fatal("credential rewrite persisted in .git/config")
			}
		})
	}
}

func TestDockerRuntimeContainerIsolation(t *testing.T) {
	if os.Getenv("API_SANDBOX_DOCKER_INTEGRATION") != "1" {
		t.Skip("set API_SANDBOX_DOCKER_INTEGRATION=1 to run against a local Docker daemon")
	}
	cli, err := getDockerClient()
	if err != nil {
		t.Fatal(err)
	}
	dockerClient = cli
	for _, name := range []string{"api-sandbox-env-integration-a", "api-sandbox-env-integration-b", "api-sandbox-env-integration-failed-start", "api-sandbox-env-integration-app-exit", "api-sandbox-env-integration-tcp"} {
		if err := CleanupContainer(context.Background(), name); err != nil {
			t.Fatalf("remove stale integration container %s: %v", name, err)
		}
	}
	root := t.TempDir()
	networkName := fmt.Sprintf("api-sandbox-test-%d", time.Now().UnixNano())
	network, err := cli.CreateNetwork(docker.CreateNetworkOptions{Name: networkName, Driver: "bridge"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cli.RemoveNetwork(network.ID) })

	type runtimeEnv struct{ id, root, marker, contents, otherMarker string }
	envs := []runtimeEnv{
		{id: "integration-a", root: filepath.Join(root, "a"), marker: "A-private.txt", contents: "environment-A", otherMarker: "B-private.txt"},
		{id: "integration-b", root: filepath.Join(root, "b"), marker: "B-private.txt", contents: "environment-B", otherMarker: "A-private.txt"},
	}
	ids := make([]string, 0, len(envs))
	t.Cleanup(func() {
		for _, id := range ids {
			_ = CleanupContainer(context.Background(), "api-sandbox-env-"+id)
		}
	})
	for _, env := range envs {
		if err := os.MkdirAll(filepath.Join(env.root, ".cache"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(env.root, env.marker), []byte(env.contents), 0644); err != nil {
			t.Fatal(err)
		}
		id, err := createRuntimeContainer(context.Background(), env.id, "alpine:3.20", networkName, env.root, filepath.Join(env.root, ".cache"), "/app", nil, nil, []string{"sleep", "infinity"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, env.id)
		inspect, err := cli.InspectContainer(id)
		if err != nil {
			t.Fatal(err)
		}
		if inspect.HostConfig.Memory != 512*1024*1024 || inspect.HostConfig.MemorySwap != 512*1024*1024 || inspect.HostConfig.CPUQuota != 100000 || inspect.HostConfig.CPUPeriod != 100000 || inspect.HostConfig.PidsLimit == nil || *inspect.HostConfig.PidsLimit != 256 {
			t.Fatalf("unexpected runtime limits: memory=%d swap=%d quota=%d period=%d pids=%v", inspect.HostConfig.Memory, inspect.HostConfig.MemorySwap, inspect.HostConfig.CPUQuota, inspect.HostConfig.CPUPeriod, inspect.HostConfig.PidsLimit)
		}
		if inspect.Config.User != "" || inspect.HostConfig.Privileged || inspect.HostConfig.PidMode == "host" || inspect.HostConfig.IpcMode == "host" || len(inspect.HostConfig.Devices) != 0 || len(inspect.HostConfig.CapDrop) != 1 || inspect.HostConfig.CapDrop[0] != "ALL" {
			t.Fatalf("unexpected privilege configuration: user=%q privileged=%v pid=%q ipc=%q devices=%v capDrop=%v", inspect.Config.User, inspect.HostConfig.Privileged, inspect.HostConfig.PidMode, inspect.HostConfig.IpcMode, inspect.HostConfig.Devices, inspect.HostConfig.CapDrop)
		}
		if !containsString(inspect.HostConfig.SecurityOpt, "no-new-privileges:true") {
			t.Fatalf("no-new-privileges missing: %v", inspect.HostConfig.SecurityOpt)
		}
		if len(inspect.Mounts) != 5 {
			t.Fatalf("expected /app and four environment cache mounts; got %+v", inspect.Mounts)
		}
		for _, mount := range inspect.Mounts {
			if mount.Destination == "/app" && mount.Source != env.root {
				t.Fatalf("/app source = %q, want %q", mount.Source, env.root)
			}
			if !strings.HasPrefix(mount.Source, env.root) {
				t.Fatalf("runtime has unexpected host mount: %+v", mount)
			}
		}
		if inspect.NetworkSettings == nil || len(inspect.NetworkSettings.Networks) != 1 || inspect.NetworkSettings.Networks[networkName].NetworkID != network.ID {
			t.Fatalf("runtime is attached to unexpected Docker networks: %+v", inspect.NetworkSettings)
		}
		if got := dockerExecOutput(t, cli, id, "cat", "/app/"+env.marker); strings.TrimSpace(got) != env.contents {
			t.Fatalf("container did not see its own marker: %q", got)
		}
		if got := dockerExecOutput(t, cli, id, "id", "-u"); strings.TrimSpace(got) != "0" {
			t.Fatalf("effective runtime UID = %q, want current image default root (0)", got)
		}
		if dockerExec(t, cli, id, "test", "-e", "/app/"+env.otherMarker) == nil {
			t.Fatalf("container %s can read the other environment's marker", env.id)
		}
		if dockerExec(t, cli, id, "test", "-e", "/workspaces") == nil {
			t.Fatalf("container %s unexpectedly has /workspaces", env.id)
		}
	}

	// A failing start occurs after Docker has created the container; the helper
	// must leave worker-level readiness able to detect the exit and clean up.
	failedID := "integration-failed-start"
	failedContainerID, startErr := createRuntimeContainer(context.Background(), failedID, "alpine:3.20", networkName, envs[0].root, filepath.Join(envs[0].root, ".cache"), "/app", nil, []string{"/bin/sh"}, []string{"-c", "exit 23"})
	if startErr == nil {
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		exitCode, waitErr := cli.WaitContainerWithContext(failedContainerID, waitCtx)
		cancel()
		if waitErr != nil || exitCode != 23 {
			t.Fatalf("failed application process was not observed: exit=%d err=%v", exitCode, waitErr)
		}
		if cleanupErr := CleanupContainer(context.Background(), failedContainerID); cleanupErr != nil {
			t.Fatal(cleanupErr)
		}
	}
	if _, err := cli.InspectContainer("api-sandbox-env-" + failedID); err == nil {
		t.Fatal("partially provisioned container remains after failed start")
	}

	// The application command is PID 1: when it exits, the runtime container
	// exits too, allowing reconciliation to observe the failure.
	lifecycleID := "integration-app-exit"
	startScript := filepath.Join(envs[0].root, "sandbox-start.sh")
	if err := os.WriteFile(startScript, []byte("#!/bin/sh\nsleep 1\nexit 17\n"), 0755); err != nil {
		t.Fatal(err)
	}
	containerID, err := createRuntimeContainer(context.Background(), lifecycleID, "alpine:3.20", networkName, envs[0].root, filepath.Join(envs[0].root, ".cache"), "/app", nil, []string{"/bin/sh"}, []string{"/app/sandbox-start.sh"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = CleanupContainer(context.Background(), containerID) })
	inspect, err := cli.InspectContainer(containerID)
	if err != nil {
		t.Fatal(err)
	}
	if inspect.Path != "/bin/sh" || len(inspect.Args) != 1 || inspect.Args[0] != "/app/sandbox-start.sh" {
		t.Fatalf("unexpected PID 1 command: path=%q args=%v", inspect.Path, inspect.Args)
	}
	time.Sleep(1500 * time.Millisecond)
	running, _, err := CheckContainerHealth(context.Background(), containerID)
	if err != nil {
		t.Fatal(err)
	}
	if running {
		t.Fatal("container remained running after its PID 1 application exited")
	}

	// TCP readiness checks the configured container port, while HTTP readiness
	// is separately covered through the live Compose preview route.
	tcpID := "integration-tcp"
	tcpContainerID, err := createRuntimeContainer(context.Background(), tcpID, "alpine:3.20", networkName, envs[0].root, filepath.Join(envs[0].root, ".cache"), "/app", nil, []string{"busybox"}, []string{"nc", "-l", "-p", "3000"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = CleanupContainer(context.Background(), tcpContainerID) })
	time.Sleep(250 * time.Millisecond)
	if err := CheckContainerTCP(context.Background(), tcpContainerID, "3000"); err != nil {
		t.Fatalf("open runtime TCP port was not ready: %v", err)
	}
	if err := CheckContainerTCP(context.Background(), tcpContainerID, "3001"); err == nil {
		t.Fatal("closed runtime TCP port unexpectedly passed readiness")
	}
	for _, env := range envs {
		if err := CleanupContainer(context.Background(), "api-sandbox-env-"+env.id); err != nil {
			t.Fatalf("cleanup %s: %v", env.id, err)
		}
		if _, err := cli.InspectContainer("api-sandbox-env-" + env.id); err == nil {
			t.Fatalf("container for %s remains after cleanup", env.id)
		}
	}
}

func dockerExec(t *testing.T, cli *docker.Client, containerID string, args ...string) error {
	t.Helper()
	execConfig, err := cli.CreateExec(docker.CreateExecOptions{Container: containerID, Cmd: args, AttachStdout: true, AttachStderr: true})
	if err != nil {
		return err
	}
	if err := cli.StartExec(execConfig.ID, docker.StartExecOptions{OutputStream: &bytes.Buffer{}, ErrorStream: &bytes.Buffer{}}); err != nil {
		return err
	}
	result, err := cli.InspectExec(execConfig.ID)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("docker exec %v exited %d", args, result.ExitCode)
	}
	return nil
}

func dockerExecOutput(t *testing.T, cli *docker.Client, containerID string, args ...string) string {
	t.Helper()
	var output bytes.Buffer
	execConfig, err := cli.CreateExec(docker.CreateExecOptions{Container: containerID, Cmd: args, AttachStdout: true, AttachStderr: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.StartExec(execConfig.ID, docker.StartExecOptions{OutputStream: &output, ErrorStream: &output}); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
