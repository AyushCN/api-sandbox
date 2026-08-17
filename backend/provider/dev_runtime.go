package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/api-sandbox/backend/models"
	"github.com/pelletier/go-toml/v2"
)

type DevRuntimeConfig struct {
	BaseImage   string
	InstallCmd  string
	StartCmd    string
	WatchHint   string
	WorkDir     string
	ExposedPort string
}

func ResolveRuntime(env *models.Environment, repoPath string, subDir string) (DevRuntimeConfig, error) {
	config, err := DetectDevRuntime(repoPath, subDir)

	// Apply DB overrides
	if env.StartCommand != nil && *env.StartCommand != "" {
		config.StartCmd = *env.StartCommand
		err = nil // Clear heuristic errors if explicit command given
	}
	if env.Port != nil && *env.Port > 0 {
		config.ExposedPort = fmt.Sprintf("%d", *env.Port)
	}

	return config, err
}

func DetectDevRuntime(repoPath string, subDir string) (DevRuntimeConfig, error) {
	appDir := filepath.Join(repoPath, subDir)
	var config DevRuntimeConfig
	var err error

	// Node.js detection
	packageJsonPath := filepath.Join(appDir, "package.json")
	if _, errStat := os.Stat(packageJsonPath); errStat == nil {
		config, err = detectNodeRuntime(appDir, subDir)
	} else if _, errStat := os.Stat(filepath.Join(appDir, "requirements.txt")); errStat == nil {
		config, err = detectPythonRuntime(appDir, subDir)
	} else if _, errStat := os.Stat(filepath.Join(appDir, "pyproject.toml")); errStat == nil {
		config, err = detectPythonRuntime(appDir, subDir)
	} else if _, errStat := os.Stat(filepath.Join(appDir, "Pipfile")); errStat == nil {
		config, err = detectPythonRuntime(appDir, subDir)
	} else if _, errStat := os.Stat(filepath.Join(appDir, "go.mod")); errStat == nil {
		config, err = detectGoRuntime(appDir, subDir)
	} else {
		err = fmt.Errorf("no supported language detected")
	}

	// Read sandbox.toml for overrides
	sandboxTomlPath := filepath.Join(appDir, "sandbox.toml")
	if b, readErr := os.ReadFile(sandboxTomlPath); readErr == nil {
		var override struct {
			BaseImage   string `toml:"base_image"`
			InstallCmd  string `toml:"install_cmd"`
			StartCmd    string `toml:"start_cmd"`
			WorkDir     string `toml:"work_dir"`
			ExposedPort string `toml:"exposed_port"`
		}
		if tomlErr := toml.Unmarshal(b, &override); tomlErr == nil {
			if override.BaseImage != "" {
				config.BaseImage = override.BaseImage
			}
			if override.InstallCmd != "" {
				config.InstallCmd = override.InstallCmd
			}
			if override.StartCmd != "" {
				config.StartCmd = override.StartCmd
			}
			if override.WorkDir != "" {
				config.WorkDir = override.WorkDir
			}
			if override.ExposedPort != "" {
				config.ExposedPort = override.ExposedPort
			}
			// If we had no detected config but they provided sandbox.toml, we clear the error if start cmd is provided
			if override.StartCmd != "" {
				err = nil
			}
		}
	}

	return config, err
}

func getWorkDir(subDir string) string {
	if subDir != "" {
		return "/app/" + subDir
	}
	return "/app"
}

func detectNodeRuntime(appDir, subDir string) (DevRuntimeConfig, error) {
	packageJsonPath := filepath.Join(appDir, "package.json")
	content, err := os.ReadFile(packageJsonPath)

	installCmd := "npm install"
	startCmd := "node --watch index.js" // fallback (native fast watch)

	// Detect package manager
	if _, err := os.Stat(filepath.Join(appDir, "yarn.lock")); err == nil {
		installCmd = "yarn install"
	} else if _, err := os.Stat(filepath.Join(appDir, "pnpm-lock.yaml")); err == nil {
		installCmd = "pnpm install"
	} else if _, err := os.Stat(filepath.Join(appDir, "bun.lockb")); err == nil {
		installCmd = "bun install"
	}

	hasDevScript := false
	if err == nil {
		var pkg map[string]interface{}
		if err := json.Unmarshal(content, &pkg); err == nil {
			if scripts, ok := pkg["scripts"].(map[string]interface{}); ok {
				if dev, ok := scripts["dev"].(string); ok && dev != "" {
					hasDevScript = true
				}
			}
		}
	}

	if hasDevScript {
		startCmd = "npm run dev"
		if strings.HasPrefix(installCmd, "yarn") {
			startCmd = "yarn dev"
		}
		if strings.HasPrefix(installCmd, "pnpm") {
			startCmd = "pnpm dev"
		}
		if strings.HasPrefix(installCmd, "bun") {
			startCmd = "bun run dev"
		}
	} else {
		// Fallback chain
		if _, err := os.Stat(filepath.Join(appDir, "tsconfig.json")); err == nil {
			if _, err := os.Stat(filepath.Join(appDir, "src", "index.ts")); err == nil {
				startCmd = "npx tsx --watch src/index.ts"
			} else if _, err := os.Stat(filepath.Join(appDir, "index.ts")); err == nil {
				startCmd = "npx tsx --watch index.ts"
			} else {
				startCmd = "npx tsx --watch src/main.ts 2>/dev/null || npx tsx --watch main.ts"
			}
		} else {
			if _, err := os.Stat(filepath.Join(appDir, "src", "index.js")); err == nil {
				startCmd = "node --watch src/index.js"
			} else {
				startCmd = "node --watch index.js"
			}
		}
	}

	baseImage := "node:20-alpine"
	if strings.HasPrefix(installCmd, "bun") {
		baseImage = "oven/bun:1-alpine"
	}

	return DevRuntimeConfig{
		BaseImage:   baseImage,
		InstallCmd:  installCmd,
		StartCmd:    startCmd,
		WatchHint:   "Node.js detected. Native file events via touch-on-save.",
		WorkDir:     getWorkDir(subDir),
		ExposedPort: "3000", // Default to 3000 for Node.js (Next.js, Express, etc)
	}, nil
}

func detectPythonRuntime(appDir, subDir string) (DevRuntimeConfig, error) {
	installCmd := "pip install -r requirements.txt"
	if _, err := os.Stat(filepath.Join(appDir, "requirements.txt")); err != nil {
		if _, err := os.Stat(filepath.Join(appDir, "pyproject.toml")); err == nil {
			installCmd = "pip install ."
		} else if _, err := os.Stat(filepath.Join(appDir, "Pipfile")); err == nil {
			installCmd = "pip install pipenv && pipenv install --system"
		}
	}

	installCmd += " && pip install uvicorn[standard]" // Ensure uvicorn available

	startCmd := "python main.py"

	// Scan requirements for frameworks
	reqs, _ := os.ReadFile(filepath.Join(appDir, "requirements.txt"))
	reqStr := strings.ToLower(string(reqs))

	if strings.Contains(reqStr, "fastapi") {
		startCmd = "uvicorn main:app --host 0.0.0.0 --reload --reload-dir ."
		if _, err := os.Stat(filepath.Join(appDir, "app", "main.py")); err == nil {
			startCmd = "uvicorn app.main:app --host 0.0.0.0 --reload --reload-dir ."
		}
	} else if strings.Contains(reqStr, "django") || fileExists(filepath.Join(appDir, "manage.py")) {
		startCmd = "python manage.py runserver 0.0.0.0:8000"
	} else if strings.Contains(reqStr, "flask") {
		if fileExists(filepath.Join(appDir, "app.py")) {
			startCmd = "FLASK_APP=app.py flask run --host=0.0.0.0 --port=8000 --reload"
		} else if fileExists(filepath.Join(appDir, "main.py")) {
			startCmd = "FLASK_APP=main.py flask run --host=0.0.0.0 --port=8000 --reload"
		} else {
			// Try to find a single .py file
			files, _ := filepath.Glob(filepath.Join(appDir, "*.py"))
			if len(files) == 1 {
				startCmd = fmt.Sprintf("FLASK_APP=%s flask run --host=0.0.0.0 --port=8000 --reload", filepath.Base(files[0]))
			} else {
				startCmd = "flask run --host=0.0.0.0 --port=8000 --reload"
			}
		}
	} else {
		if fileExists(filepath.Join(appDir, "app.py")) {
			startCmd = "python app.py"
		}
	}

	return DevRuntimeConfig{
		BaseImage:   "python:3.11-slim",
		InstallCmd:  installCmd,
		StartCmd:    startCmd,
		WatchHint:   "Python detected. Native file events via touch-on-save.",
		WorkDir:     getWorkDir(subDir),
		ExposedPort: "8000",
	}, nil
}

func detectGoRuntime(appDir, subDir string) (DevRuntimeConfig, error) {
	return DevRuntimeConfig{
		BaseImage:   "golang:1.22-alpine",
		InstallCmd:  "go mod download && go install github.com/air-verse/air@latest",
		StartCmd:    "if [ ! -f .air.toml ]; then air init; fi && air || go run .",
		WatchHint:   "Go detected. Air uses native file events via touch-on-save.",
		WorkDir:     getWorkDir(subDir),
		ExposedPort: "8080",
	}, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func globFiles(dir, pattern string) []string {
	files, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return nil
	}
	return files
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
