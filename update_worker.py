import re

with open('backend/worker/worker.go', 'r') as f:
    content = f.read()

# 1. Update StartContainer call
content = content.replace(
    'containerID, port, err := StartContainer(ctx, env.ID, imageTag, netID)',
    'containerID, port, err := StartContainer(ctx, env.ID, imageTag, netID, dbURL)'
)

# 2. Add Database Provisioning logic
db_logic = """
	// 1.5 Database Provisioning
	var dbURL string
	if env.UserProvidedDBURL != nil && *env.UserProvidedDBURL != "" {
		dbURL = *env.UserProvidedDBURL
		db.DB.Create(&models.Log{
			EnvironmentID: env.ID,
			Message:       "Using user-provided DATABASE_URL",
			Level:         models.LogLevelInfo,
		})
	} else {
		wd, _ := os.Getwd()
		workspaceDir := filepath.Join(wd, "workspaces", env.ID)
		
		dbType, _ := DetectDatabaseRequirements(workspaceDir)
		if dbType != DBTypeNone {
			db.DB.Create(&models.Log{
				EnvironmentID: env.ID,
				Message:       fmt.Sprintf("Auto-detected database requirement: %s", string(dbType)),
				Level:         models.LogLevelInfo,
			})
			
			netID := env.OrganizationID
			if netID == "" {
				netID = env.UserID
			}
			
			url, err := StartSidecarDatabase(ctx, env.ID, netID, dbType)
			if err != nil {
				slog.Error("Failed to start sidecar db", "env_id", envID, "error", err)
				db.DB.Create(&models.Log{
					EnvironmentID: env.ID,
					Message:       fmt.Sprintf("Failed to provision database: %v", err),
					Level:         models.LogLevelError,
				})
			} else {
				dbURL = url
			}
		}
	}

	// 2. Start Container"""

content = content.replace('	// 2. Start Container', db_logic)

with open('backend/worker/worker.go', 'w') as f:
    f.write(content)

