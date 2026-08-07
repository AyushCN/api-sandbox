import re

with open('backend/worker/docker.go', 'r') as f:
    content = f.read()

# 1. Update StartContainer signature
content = content.replace(
    'func StartContainer(ctx context.Context, envID string, imageTag string, orgID string) (string, int, error) {',
    'func StartContainer(ctx context.Context, envID string, imageTag string, orgID string, dbURL string) (string, int, error) {'
)

# 2. Update Env inside StartContainer
env_block_old = '''			Env: []string{
				fmt.Sprintf("PORT=%s", exposedPort),
				"HOST=0.0.0.0",
			},'''
env_block_new = '''			Env: func() []string {
				e := []string{fmt.Sprintf("PORT=%s", exposedPort), "HOST=0.0.0.0"}
				if dbURL != "" {
					e = append(e, fmt.Sprintf("DATABASE_URL=%s", dbURL), fmt.Sprintf("MONGO_URI=%s", dbURL))
				}
				return e
			}(),'''
content = content.replace(env_block_old, env_block_new)

# 3. Add StartSidecarDatabase function
sidecar_func = """
func StartSidecarDatabase(ctx context.Context, envID string, orgID string, dbType DBType) (string, error) {
	if dbType == DBTypeNone {
		return "", nil
	}

	networkName := fmt.Sprintf("api-sandbox-net-%s", orgID)
	networks, err := dockerClient.ListNetworks()
	var networkFound bool
	if err == nil {
		for _, net := range networks {
			if net.Name == networkName {
				networkFound = true
				break
			}
		}
	}

	if !networkFound {
		_, _ = dockerClient.CreateNetwork(docker.CreateNetworkOptions{
			Name:           networkName,
			Driver:         "bridge",
			CheckDuplicate: true,
			EnableIPv6:     false,
		})
	}

	containerName := fmt.Sprintf("api-sandbox-db-%s", envID)
	_ = CleanupContainer(ctx, containerName)

	var image, dbURL string
	var env []string

	switch dbType {
	case DBTypeMySQL:
		image = "mysql:8.0"
		env = []string{
			"MYSQL_ROOT_PASSWORD=rootpass123",
			"MYSQL_DATABASE=myapp",
			"MYSQL_USER=appuser",
			"MYSQL_PASSWORD=apppassword",
		}
		dbURL = fmt.Sprintf("mysql://appuser:apppassword@%s:3306/myapp", containerName)
	case DBTypePostgres:
		image = "postgres:15"
		env = []string{
			"POSTGRES_DB=myapp",
			"POSTGRES_USER=appuser",
			"POSTGRES_PASSWORD=apppassword",
		}
		dbURL = fmt.Sprintf("postgresql://appuser:apppassword@%s:5432/myapp", containerName)
	case DBTypeMongo:
		image = "mongo:6.0"
		env = []string{
			"MONGO_INITDB_DATABASE=myapp",
			"MONGO_INITDB_ROOT_USERNAME=admin",
			"MONGO_INITDB_ROOT_PASSWORD=adminpass",
		}
		dbURL = fmt.Sprintf("mongodb://admin:adminpass@%s:27017/myapp?authSource=admin", containerName)
	}

	db.DB.Create(&models.Log{
		EnvironmentID: envID,
		Message:       fmt.Sprintf("Pulling %s database image (this may take a minute on first run)...", string(dbType)),
		Level:         models.LogLevelInfo,
	})

	pullOpts := docker.PullImageOptions{
		Repository: image,
	}
	_ = dockerClient.PullImage(pullOpts, docker.AuthConfiguration{})

	db.DB.Create(&models.Log{
		EnvironmentID: envID,
		Message:       fmt.Sprintf("Starting sidecar database container (%s)...", containerName),
		Level:         models.LogLevelInfo,
	})

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

	return dbURL, nil
}
"""
content += sidecar_func

with open('backend/worker/docker.go', 'w') as f:
    f.write(content)

