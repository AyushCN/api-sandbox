package provider

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

func CheckContainerHealth(ctx context.Context, containerID string) (bool, string, error) {
	inspect, err := dockerClient.InspectContainerWithContext(containerID, ctx)
	if err != nil {
		return false, "", err
	}
	if !inspect.State.Running {
		var outBuf bytes.Buffer
		err := dockerClient.Logs(docker.LogsOptions{
			Context:      ctx,
			Container:    containerID,
			OutputStream: &outBuf,
			ErrorStream:  &outBuf,
			Stdout:       true,
			Stderr:       true,
			Tail:         "100",
		})
		if err != nil {
			return false, "Could not fetch crash logs", nil
		}
		return false, outBuf.String(), nil
	}
	return true, "", nil
}

func IsHTTPReadyStatus(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

func CheckContainerTCP(ctx context.Context, containerID, port string) error {
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("invalid runtime TCP port %q", port)
	}
	inspect, err := dockerClient.InspectContainerWithContext(containerID, ctx)
	if err != nil {
		return err
	}
	if !inspect.State.Running {
		return fmt.Errorf("runtime container %s is not running", containerID)
	}
	var dialErr error
	for _, endpoint := range inspect.NetworkSettings.Networks {
		if endpoint.IPAddress == "" {
			continue
		}
		dialCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", net.JoinHostPort(endpoint.IPAddress, strconv.Itoa(portNumber)))
		cancel()
		if err == nil {
			_ = conn.Close()
			return nil
		}
		dialErr = err
	}
	if dialErr != nil {
		return fmt.Errorf("connect to runtime port %d: %w", portNumber, dialErr)
	}
	return fmt.Errorf("runtime container %s has no network address", containerID)
}
