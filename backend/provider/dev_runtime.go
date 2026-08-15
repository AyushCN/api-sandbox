package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type DevRuntimeConfig struct {
	BaseImage  string
	InstallCmd string
	StartCmd   string
	WatchHint   string
	WorkDir     string
	ExposedPort string
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
	} else if _, errStat := os.Stat(filepath.Join(appDir, "Gemfile")); errStat == nil {
		config, err = detectRubyRuntime(appDir, subDir)
	} else if _, errStat := os.Stat(filepath.Join(appDir, "composer.json")); errStat == nil {
		config, err = detectPHPRuntime(appDir, subDir)
	} else if _, errStat := os.Stat(filepath.Join(appDir, "Cargo.toml")); errStat == nil {
		config, err = detectRustRuntime(appDir, subDir)
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
	startCmd := "npx nodemon -L index.js" // fallback
	
	// Detect package manager
	if _, err := os.Stat(filepath.Join(appDir, "yarn.lock")); err == nil {
		installCmd = "yarn install"
	} else if _, err := os.Stat(filepath.Join(appDir, "pnpm-lock.yaml")); err == nil {
		installCmd = "pnpm install"
	} else if _, err := os.Stat(filepath.Join(appDir, "bun.lockb")); err == nil {
		installCmd = "bun install"
	}

	isNextJs := false
	if err == nil {
		var pkg map[string]interface{}
		if err := json.Unmarshal(content, &pkg); err == nil {
			// Check dependencies for next
			if deps, ok := pkg["dependencies"].(map[string]interface{}); ok {
				if _, hasNext := deps["next"]; hasNext {
					isNextJs = true
				}
			}
			
			if scripts, ok := pkg["scripts"].(map[string]interface{}); ok {
				if isNextJs {
					startCmd = "npm run dev"
					if strings.HasPrefix(installCmd, "yarn") { startCmd = "yarn dev" }
					if strings.HasPrefix(installCmd, "pnpm") { startCmd = "pnpm dev" }
					if strings.HasPrefix(installCmd, "bun") { startCmd = "bun run dev" }
				} else if dev, ok := scripts["dev"].(string); ok && dev != "" {
					startCmd = "npm run dev"
					if strings.HasPrefix(installCmd, "yarn") { startCmd = "yarn dev" }
					if strings.HasPrefix(installCmd, "pnpm") { startCmd = "pnpm dev" }
					if strings.HasPrefix(installCmd, "bun") { startCmd = "bun run dev" }
				} else if start, ok := scripts["start"].(string); ok && start != "" {
					startCmd = "npx nodemon -L --exec \"npm start\""
					if strings.HasPrefix(installCmd, "yarn") { startCmd = "npx nodemon -L --exec \"yarn start\"" }
					if strings.HasPrefix(installCmd, "pnpm") { startCmd = "npx nodemon -L --exec \"pnpm start\"" }
					if strings.HasPrefix(installCmd, "bun") { startCmd = "npx nodemon -L --exec \"bun start\"" }
				}
			}
		}
	}

	// Check if typescript and not nextjs, maybe we need ts-node
	if !isNextJs {
		if _, err := os.Stat(filepath.Join(appDir, "tsconfig.json")); err == nil {
			if startCmd == "npx nodemon -L index.js" {
				// Try to find index.ts or src/index.ts
				if _, err := os.Stat(filepath.Join(appDir, "src", "index.ts")); err == nil {
					startCmd = "npx nodemon -L src/index.ts"
				} else if _, err := os.Stat(filepath.Join(appDir, "index.ts")); err == nil {
					startCmd = "npx nodemon -L index.ts"
				}
			}
		} else {
			if startCmd == "npx nodemon -L index.js" {
				if _, err := os.Stat(filepath.Join(appDir, "src", "index.js")); err == nil {
					startCmd = "npx nodemon -L src/index.js"
				}
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
		WatchHint:   "Node.js detected. Polling enforced via nodemon -L where applicable.",
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

	installCmd += " watchdog" // Ensure polling works

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
		WatchHint:   "Python detected. Watchdog installed for polling.",
		WorkDir:     getWorkDir(subDir),
		ExposedPort: "8000",
	}, nil
}

func detectGoRuntime(appDir, subDir string) (DevRuntimeConfig, error) {
	return DevRuntimeConfig{
		BaseImage:   "golang:1.22-alpine",
		InstallCmd:  "go mod download && go install github.com/air-verse/air@latest",
		StartCmd:    "if [ ! -f .air.toml ]; then air init && sed -i 's/poll = false/poll = true/' .air.toml; fi && air || go run .",
		WatchHint:   "Go detected. Air configured with polling enabled.",
		WorkDir:     getWorkDir(subDir),
		ExposedPort: "8080",
	}, nil
}

func detectRubyRuntime(appDir, subDir string) (DevRuntimeConfig, error) {
	startCmd := "ruby main.rb"
	gemfile, _ := os.ReadFile(filepath.Join(appDir, "Gemfile"))
	gemStr := strings.ToLower(string(gemfile))
	
	if strings.Contains(gemStr, "rails") {
		startCmd = "bin/rails server -b 0.0.0.0"
	}

	return DevRuntimeConfig{
		BaseImage:   "ruby:3.3-alpine",
		InstallCmd:  "bundle install",
		StartCmd:    startCmd,
		WatchHint:   "Ruby/Rails detected.",
		WorkDir:     getWorkDir(subDir),
		ExposedPort: "3000",
	}, nil
}

func detectPHPRuntime(appDir, subDir string) (DevRuntimeConfig, error) {
	startCmd := "php -S 0.0.0.0:8000"
	composer, _ := os.ReadFile(filepath.Join(appDir, "composer.json"))
	compStr := strings.ToLower(string(composer))
	
	if strings.Contains(compStr, "laravel/framework") {
		startCmd = "php artisan serve --host=0.0.0.0 --port=8000"
	}

	return DevRuntimeConfig{
		BaseImage:   "php:8.2-cli-alpine",
		InstallCmd:  "apk add composer && composer install",
		StartCmd:    startCmd,
		WatchHint:   "PHP/Laravel detected.",
		WorkDir:     getWorkDir(subDir),
		ExposedPort: "8000",
	}, nil
}

func detectRustRuntime(appDir, subDir string) (DevRuntimeConfig, error) {
	return DevRuntimeConfig{
		BaseImage:   "rust:1-slim",
		InstallCmd:  "cargo install cargo-watch",
		StartCmd:    "cargo watch -x run",
		WatchHint:   "Rust detected. cargo-watch used for live reloading.",
		WorkDir:     getWorkDir(subDir),
		ExposedPort: "8000",
	}, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
