package api

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/fsnotify/fsnotify"
)

func WatchAllEnvironments() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("Failed to create file watcher", "error", err)
		return
	}
	defer watcher.Close()

	wd, err := os.Getwd()
	if err != nil {
		slog.Error("Failed to get working directory for watcher", "error", err)
		return
	}

	workspacesDir := filepath.Join(wd, "workspaces")

	// Create workspaces dir if it doesn't exist
	if _, err := os.Stat(workspacesDir); os.IsNotExist(err) {
		os.MkdirAll(workspacesDir, 0755)
	}

	err = watcher.Add(workspacesDir)
	if err != nil {
		slog.Error("Failed to watch workspaces directory", "error", err)
		return
	}

	// Watch existing environment directories
	entries, _ := os.ReadDir(workspacesDir)
	for _, entry := range entries {
		if entry.IsDir() {
			envDir := filepath.Join(workspacesDir, entry.Name())
			addWatchRecursively(watcher, envDir)
		}
	}

	slog.Info("File watcher started on workspaces directory")

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// If it's a new directory, watch it
			if event.Op&fsnotify.Create == fsnotify.Create {
				info, err := os.Stat(event.Name)
				if err == nil && info.IsDir() {
					addWatchRecursively(watcher, event.Name)
				}
			}

			// Ignore .git directory changes to avoid noise
			if strings.Contains(event.Name, "/.git/") || strings.HasSuffix(event.Name, ".git") {
				continue
			}

			// Extract envID and relative path
			relPath, err := filepath.Rel(workspacesDir, event.Name)
			if err != nil {
				continue
			}
			parts := strings.Split(relPath, string(os.PathSeparator))
			if len(parts) < 2 {
				continue
			}
			envID := parts[0]
			filePath := filepath.Join(parts[1:]...)

			var changeType string
			if event.Op&fsnotify.Write == fsnotify.Write {
				changeType = "modified"
			} else if event.Op&fsnotify.Create == fsnotify.Create {
				changeType = "created"
			} else if event.Op&fsnotify.Remove == fsnotify.Remove {
				changeType = "deleted"
			} else {
				continue // Ignore Chmod, Rename (unless we want to handle Rename specially)
			}

			data := map[string]interface{}{
				"type":      "file_changed",
				"file_path": filePath,
				"action":    changeType,
				"user_name": "System/External",
				"user_id":   "system",
			}
			dataBytes, _ := json.Marshal(data)

			// Create Activity
			db.DB.Create(&models.Activity{
				EnvironmentID: &envID,
				Type:          "file_edit",
				Data:          string(dataBytes),
				UserID:        nil, // System user
			})

			// Broadcast
			BroadcastToProjectMembers(envID, data)

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			slog.Error("File watcher error", "error", err)
		}
	}
}

func addWatchRecursively(watcher *fsnotify.Watcher, dir string) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && !strings.Contains(path, "/.git") {
			watcher.Add(path)
		}
		return nil
	})
}
