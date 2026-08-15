package provider

import (
	"bytes"
	"github.com/fsouza/go-dockerclient"
)

func CheckContainerHealth(containerID string) (bool, string, error) {
	inspect, err := dockerClient.InspectContainer(containerID)
	if err != nil {
		return false, "", err
	}
	if !inspect.State.Running {
		var outBuf bytes.Buffer
		err := dockerClient.Logs(docker.LogsOptions{
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
