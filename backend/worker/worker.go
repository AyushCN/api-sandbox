package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/api-sandbox/backend/api"
	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/api-sandbox/backend/provider"
	"github.com/hibiken/asynq"
)

var (
	ProviderCleanupContainer                                                                                                = provider.CleanupContainer
	ProviderCloneOrFetch               func(ctx context.Context, dir, gitURL, branch, baseCommit, githubToken string) error = provider.CloneOrFetch
	ProviderDetectDatabaseRequirements                                                                                      = provider.DetectDatabaseRequirements
	ProviderStartSidecarDatabase                                                                                            = provider.StartSidecarDatabase
	ProviderCheckContainerHealth       func(context.Context, string) (bool, string, error)                                  = provider.CheckContainerHealth
)

func HandleBuildEnvironmentTask(ctx context.Context, t *asynq.Task) error {
	var payload map[string]string
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	envID, ok := payload["environmentId"]
	if !ok {
		return fmt.Errorf("missing environmentId: %w", asynq.SkipRetry)
	}

	slog.Info("Processing build job", "environment_id", envID)

	retryCount, _ := asynq.GetRetryCount(ctx)
	maxRetry, _ := asynq.GetMaxRetry(ctx)
	if retryCount > 0 {
		db.DB.Create(&models.Log{
			EnvironmentID: &envID,
			Message:       fmt.Sprintf("⚠️ Attempt %d of %d: Retrying failed build job...", retryCount+1, maxRetry+1),
			Level:         models.LogLevelInfo,
		})
	}

	var env models.Environment
	if err := db.DB.First(&env, "id = ?", envID).Error; err != nil {
		return fmt.Errorf("environment not found: %w", err)
	}
	if env.Status != models.StatusBuilding {
		return fmt.Errorf("environment %s is %s, not BUILDING: %w", envID, env.Status, asynq.SkipRetry)
	}
	claim := db.DB.Model(&models.Environment{}).Where("id = ? AND status = ?", env.ID, models.StatusBuilding).Update("updated_at", time.Now())
	if claim.Error != nil {
		return fmt.Errorf("refresh BUILDING lease for environment %s: %w", env.ID, claim.Error)
	}
	if claim.RowsAffected == 0 {
		return fmt.Errorf("environment %s left BUILDING before worker claim: %w", env.ID, asynq.SkipRetry)
	}

	buildCtx, cancelBuild := context.WithTimeout(ctx, 45*time.Minute)
	defer cancelBuild()
	ctx = buildCtx

	var user models.User
	if err := db.DB.First(&user, "id = ?", env.UserID).Error; err != nil {
		slog.Warn("User not found for environment", "user_id", env.UserID)
	}

	// Try to get token, decrypt it
	githubToken := ""
	if user.GithubToken != "" {
		decrypted, err := api.Decrypt(user.GithubToken)
		if err == nil {
			githubToken = decrypted
		} else {
			slog.Error("Failed to decrypt github token", "error", err)
		}
	}

	// Idempotency: cleanup existing container if retrying
	if env.ContainerID != nil && *env.ContainerID != "" {
		slog.Info("Cleaning up existing container", "container_id", *env.ContainerID)
		if err := provider.CleanupContainer(ctx, *env.ContainerID); err != nil {
			return fmt.Errorf("remove previous environment container: %w", err)
		}
	}
	buildSucceeded := false
	finalAttempt := retryCount >= maxRetry
	defer func() {
		if buildSucceeded {
			return
		}
		cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelCleanup()
		for _, name := range []string{fmt.Sprintf("api-sandbox-env-%s", env.ID), fmt.Sprintf("api-sandbox-db-%s", env.ID)} {
			if err := ProviderCleanupContainer(cleanupCtx, name); err != nil {
				slog.Error("Failed to remove Docker resource after build failure", "environment_id", env.ID, "container", name, "error", err)
			}
		}
		if err := provider.ClearRuntimeRoute(cleanupCtx, env.ID); err != nil {
			slog.Error("Failed to clear Traefik route after build failure", "environment_id", env.ID, "error", err)
		}
		if finalAttempt {
			if err := db.DB.Model(&models.Environment{}).Where("id = ? AND status = ?", env.ID, models.StatusBuilding).Update("status", models.StatusFailed).Error; err != nil {
				slog.Error("Failed to mark exhausted build as FAILED", "environment_id", env.ID, "error", err)
			}
		}
	}()

	workspaceDir := provider.GetWorkspacePath(env.ID)

	// 1. Resolve Workspace and Repositories
	var workspace models.Workspace
	db.DB.Where("environment_id = ?", env.ID).First(&workspace)

	var workspaceRepos []models.WorkspaceRepository
	if workspace.ID != "" {
		db.DB.Preload("ProjectRepository").Where("workspace_id = ?", workspace.ID).Find(&workspaceRepos)
	}

	if len(workspaceRepos) > 0 {
		for _, wRepo := range workspaceRepos {
			cloneDir := provider.GetWorkspaceRepositoryPath(env.ID, len(workspaceRepos), wRepo.WorkingDirectory, wRepo.ProjectRepository.Name, wRepo.ID)

			db.DB.Create(&models.Log{
				EnvironmentID: &env.ID,
				Message:       fmt.Sprintf("Synchronizing repository %s (branch: %s)...", wRepo.ProjectRepository.GitURL, wRepo.Branch),
				Level:         models.LogLevelInfo,
			})

			err := ProviderCloneOrFetch(ctx, cloneDir, wRepo.ProjectRepository.GitURL, wRepo.Branch, wRepo.BaseCommit, githubToken)
			if err != nil {
				slog.Error("Clone failed", "env_id", envID, "repo", wRepo.ProjectRepository.GitURL, "error", err)
				db.DB.Create(&models.Log{
					EnvironmentID: &env.ID,
					Message:       fmt.Sprintf("Git clone failed for %s: %v", wRepo.ProjectRepository.GitURL, err),
					Level:         models.LogLevelError,
				})
				return err
			}

			cmdHash := exec.Command("git", "rev-parse", "HEAD")
			cmdHash.Dir = cloneDir
			if hashOut, err := cmdHash.Output(); err == nil {
				hashStr := strings.TrimSpace(string(hashOut))
				db.DB.Model(&wRepo).Updates(map[string]interface{}{
					"base_commit":    hashStr,
					"current_commit": hashStr,
				})
				db.DB.Model(&env).Update("commit_hash", hashStr)
			}
		}
	} else {
		// Legacy behavior
		db.DB.Create(&models.Log{
			EnvironmentID: &env.ID,
			Message:       fmt.Sprintf("Synchronizing repository %s (branch: %s)...", env.GitURL, env.GithubBranch),
			Level:         models.LogLevelInfo,
		})

		err := ProviderCloneOrFetch(ctx, workspaceDir, env.GitURL, env.GithubBranch, "", githubToken)
		if err != nil {
			slog.Error("Clone failed", "env_id", envID, "error", err)
			db.DB.Create(&models.Log{
				EnvironmentID: &env.ID,
				Message:       fmt.Sprintf("Git clone failed: %v", err),
				Level:         models.LogLevelError,
			})
			return err
		}

		cmdHash := exec.Command("git", "rev-parse", "HEAD")
		cmdHash.Dir = workspaceDir
		if hashOut, err := cmdHash.Output(); err == nil {
			hashStr := strings.TrimSpace(string(hashOut))
			db.DB.Model(&env).Update("commit_hash", hashStr)
		}
	}

	// 2. Database Provisioning
	// For multi-repo workspaces, DB detection runs against the primary repo dir.
	// The primary repo is the first WorkspaceRepository (by stable order); for
	// single-repo workspaces this is the workspace root itself.
	var repoDirs []string
	if len(workspaceRepos) > 0 {
		for _, wRepo := range workspaceRepos {
			d := provider.GetWorkspaceRepositoryPath(env.ID, len(workspaceRepos), wRepo.WorkingDirectory, wRepo.ProjectRepository.Name, wRepo.ID)
			repoDirs = append(repoDirs, d)
		}
	}
	primaryDir := provider.GetPrimaryRepositoryPath(env.ID, repoDirs)

	var dbURL string
	if env.UserProvidedDBURL != nil && *env.UserProvidedDBURL != "" {
		dbURL = *env.UserProvidedDBURL
		db.DB.Create(&models.Log{
			EnvironmentID: &env.ID,
			Message:       "Using user-provided DATABASE_URL",
			Level:         models.LogLevelInfo,
		})
	} else {
		dbType, _ := provider.DetectDatabaseRequirements(primaryDir)
		if dbType != provider.DBTypeNone {
			db.DB.Create(&models.Log{
				EnvironmentID: &env.ID,
				Message:       fmt.Sprintf("Auto-detected database requirement: %s", string(dbType)),
				Level:         models.LogLevelInfo,
			})

			userNetworkID := env.UserID

			url, err := provider.StartSidecarDatabase(ctx, env.ID, userNetworkID, dbType)
			if err != nil {
				slog.Error("Failed to start sidecar db", "env_id", envID, "error", err)
				db.DB.Create(&models.Log{
					EnvironmentID: &env.ID,
					Message:       fmt.Sprintf("Failed to provision database: %v", err),
					Level:         models.LogLevelError,
				})
				return err
			} else {
				dbURL = url
			}
		}
	}

	// 3. Detect Runtime or Use Overrides
	// Resolution runs against the primary repository directory so that
	// language-specific files (package.json, go.mod, etc.) are found
	// regardless of whether this is a single or multi-repo workspace.
	configs, err := provider.ResolveRuntimes(&env, primaryDir, "")
	if err != nil {
		slog.Error("Failed to detect Dev Runtime", "env_id", envID, "error", err)
		db.DB.Create(&models.Log{
			EnvironmentID: &env.ID,
			Message:       fmt.Sprintf("Runtime detection failed: %v", err),
			Level:         models.LogLevelError,
		})
		return err
	}

	userNetworkID := env.UserID

	domain := os.Getenv("DOMAIN")
	if domain == "" {
		domain = "localhost"
	}

	var finalContainerID string
	var finalPort int
	var finalCrashLogs string
	var checkErr error

	maxAttempts := 3
	if len(configs) < maxAttempts {
		maxAttempts = len(configs)
	}

	var devConfig provider.DevRuntimeConfig

	for i := 0; i < maxAttempts; i++ {
		devConfig = configs[i]
		workingDir, workDirErr := provider.NormalizeRuntimeWorkDir(devConfig.WorkDir, env.ID)
		if workDirErr != nil {
			checkErr = workDirErr
			finalCrashLogs = workDirErr.Error()
			continue
		}
		devConfig.WorkDir = workingDir

		db.DB.Create(&models.Log{
			EnvironmentID: &env.ID,
			Message:       fmt.Sprintf("Attempting to boot using %s runtime (Attempt %d/%d)...", devConfig.BaseImage, i+1, maxAttempts),
			Level:         models.LogLevelInfo,
		})

		scriptContent := provider.GenerateSandboxStartScript(devConfig)
		// Write the start script into the primary repo directory, which is the
		// directory the container will mount as its working directory.
		scriptPath := filepath.Join(primaryDir, "sandbox-start.sh")
		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
			checkErr = fmt.Errorf("write runtime start script: %w", err)
			finalCrashLogs = checkErr.Error()
			continue
		}
		relScriptPath, err := filepath.Rel(workspaceDir, scriptPath)
		if err != nil || relScriptPath == ".." || strings.HasPrefix(relScriptPath, ".."+string(filepath.Separator)) {
			checkErr = fmt.Errorf("runtime start script is outside the environment workspace")
			finalCrashLogs = checkErr.Error()
			continue
		}
		devConfig.StartScriptPath = "/app/" + filepath.ToSlash(relScriptPath)

		containerID, port, provErr := provider.ProvisionDevSandbox(ctx, env.ID, devConfig, userNetworkID, dbURL)
		if provErr != nil {
			slog.Error("Dev Sandbox start failed", "env_id", envID, "error", provErr)
			continue
		}

		healthType := "http"
		if env.HealthCheckType != nil {
			healthType = *env.HealthCheckType
		}

		readinessCtx, cancelReadiness := context.WithTimeout(ctx, runtimeStartupTimeout)
		finalCrashLogs, checkErr = waitForRuntimeReadiness(
			readinessCtx,
			healthType,
			func(checkCtx context.Context) (bool, string, error) {
				return ProviderCheckContainerHealth(checkCtx, containerID)
			},
			func(probeCtx context.Context) (int, error) {
				return probeRuntimeHTTP(probeCtx, envID, domain)
			},
			func(probeCtx context.Context) error {
				return provider.CheckContainerTCP(probeCtx, containerID, devConfig.ExposedPort)
			},
			runtimeReadinessPoll,
		)
		cancelReadiness()

		if checkErr == nil {
			finalContainerID = containerID
			finalPort = port
			break
		}

		// Clean up failed container before attempting next fallback
		if cleanupErr := ProviderCleanupContainer(ctx, containerID); cleanupErr != nil {
			slog.Error("Failed to remove unhealthy runtime container", "environment_id", env.ID, "container_id", containerID, "error", cleanupErr)
		}
		if routeErr := provider.ClearRuntimeRoute(ctx, env.ID); routeErr != nil {
			slog.Error("Failed to clear unhealthy runtime route", "environment_id", env.ID, "error", routeErr)
		}
	}

	if finalContainerID == "" {
		slog.Error("Dev Sandbox failed to start after fallbacks", "env_id", envID)

		msg := "Couldn't auto-detect how to run this repo. Please manually configure the runtime and entry command."
		if finalCrashLogs != "" {
			msg = fmt.Sprintf("Last attempt crashed:\n%s\nCouldn't auto-detect how to run this repo.", finalCrashLogs)
		}

		if err := db.DB.Create(&models.Log{
			EnvironmentID: &env.ID,
			Message:       msg,
			Level:         models.LogLevelError,
		}).Error; err != nil {
			return fmt.Errorf("write failed runtime log: %w", err)
		}
		return fmt.Errorf("container crashed or failed healthcheck on boot")
	}

	// 6. Update DB to RUNNING
	protocol := "https"
	if domain == "localhost" {
		protocol = "http"
	}

	publicURL := fmt.Sprintf("%s://%s.%s", protocol, env.ID, domain)
	updateResult := db.DB.Model(&models.Environment{}).Where("id = ? AND status = ?", env.ID, models.StatusBuilding).Updates(map[string]interface{}{
		"status":       models.StatusRunning,
		"container_id": finalContainerID,
		"port":         finalPort,
		"public_url":   publicURL,
	})
	if err := updateResult.Error; err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cleanupErr := ProviderCleanupContainer(cleanupCtx, finalContainerID)
		return errors.Join(fmt.Errorf("persist running environment state: %w", err), cleanupErr)
	}
	if updateResult.RowsAffected == 0 {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cleanupErr := ProviderCleanupContainer(cleanupCtx, finalContainerID)
		return errors.Join(fmt.Errorf("environment %s left BUILDING before runtime became ready: %w", env.ID, asynq.SkipRetry), cleanupErr)
	}
	buildSucceeded = true

	slog.Info("Environment is now RUNNING", "env_id", env.ID, "port", finalPort)
	return nil
}
