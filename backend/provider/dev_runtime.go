package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type DevRuntimeConfig struct {
	BaseImage  string
	InstallCmd string
	StartCmd   string
	WatchHint  string
	WorkDir    string
}

func DetectDevRuntime(repoPath string, subDir string) (DevRuntimeConfig, error) {
	appDir := filepath.Join(repoPath, subDir)
	
	// Node.js detection
	packageJsonPath := filepath.Join(appDir, "package.json")
	if _, err := os.Stat(packageJsonPath); err == nil {
		return detectNodeRuntime(packageJsonPath, subDir)
	}

	// Python detection
	requirementsTxtPath := filepath.Join(appDir, "requirements.txt")
	if _, err := os.Stat(requirementsTxtPath); err == nil {
		return detectPythonRuntime(appDir, subDir)
	}

	// Go detection
	goModPath := filepath.Join(appDir, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		return detectGoRuntime(appDir, subDir)
	}

	return DevRuntimeConfig{}, fmt.Errorf("no supported language detected (missing package.json, requirements.txt, or go.mod)")
}

func detectNodeRuntime(packageJsonPath, subDir string) (DevRuntimeConfig, error) {
	// Parse package.json
	content, err := os.ReadFile(packageJsonPath)
	var startCmd = "npx nodemon -L index.js" // -L enables legacy watch (polling) for bind mounts
	var installCmd = "npm install" // In production, might want --ignore-scripts

	if err == nil {
		var pkg map[string]interface{}
		if err := json.Unmarshal(content, &pkg); err == nil {
			if scripts, ok := pkg["scripts"].(map[string]interface{}); ok {
				if dev, ok := scripts["dev"].(string); ok && dev != "" {
					startCmd = "npm run dev" // We assume 'dev' script uses nodemon or similar
				} else if start, ok := scripts["start"].(string); ok && start != "" {
					// Wrap start script with nodemon for polling
					startCmd = "npx nodemon -L --exec \"npm run start\""
				}
			}
		}
	}

	workDir := "/app"
	if subDir != "" {
		workDir = "/app/" + subDir
	}

	return DevRuntimeConfig{
		BaseImage:  "node:20-alpine",
		InstallCmd: installCmd,
		StartCmd:   startCmd,
		WatchHint:  "Use nodemon --legacy-watch (-L) if standard file watchers fail over bind mounts.",
		WorkDir:    workDir,
	}, nil
}

func detectPythonRuntime(appDir, subDir string) (DevRuntimeConfig, error) {
	workDir := "/app"
	if subDir != "" {
		workDir = "/app/" + subDir
	}

	startCmd := "python main.py"
	
	if _, err := os.Stat(filepath.Join(appDir, "main.py")); err == nil {
		startCmd = "python main.py"
	} else if _, err := os.Stat(filepath.Join(appDir, "app.py")); err == nil {
		startCmd = "python app.py"
	}

	// We can't automatically install uvicorn if it's not in requirements, 
	// but we'll try to run watchdog or rely on the user's framework

	return DevRuntimeConfig{
		BaseImage:  "python:3.11-slim",
		InstallCmd: "pip install -r requirements.txt",
		StartCmd:   startCmd,
		WatchHint:  "Consider using uvicorn --reload or watchdog for polling over bind mounts.",
		WorkDir:    workDir,
	}, nil
}

func detectGoRuntime(appDir, subDir string) (DevRuntimeConfig, error) {
	workDir := "/app"
	if subDir != "" {
		workDir = "/app/" + subDir
	}

	return DevRuntimeConfig{
		BaseImage:  "golang:1.22-alpine",
		// Download dependencies and install Air for live reloading
		InstallCmd: "go mod download && go install github.com/air-verse/air@latest",
		StartCmd:   "air -c .air.toml || go run .",
		WatchHint:  "Using air for live reloading. Provide .air.toml with poll=true if reload fails.",
		WorkDir:    workDir,
	}, nil
}

func GenerateSandboxStartScript(config DevRuntimeConfig) string {
	script := `#!/bin/sh
set -e

# Change to the application directory
cd ` + config.WorkDir + `

echo "========================================="
echo "🛠️  Setting up Dev Sandbox Runtime"
echo "========================================="
echo "Working Directory: ` + config.WorkDir + `"

echo "📦 Installing dependencies..."
` + config.InstallCmd + `

echo "🚀 Starting application..."
` + config.StartCmd + `
`
	return script
}
