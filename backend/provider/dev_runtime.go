package provider

import (
	"fmt"
	"os"
	"path/filepath"

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
	RuntimeType string
}

func ResolveRuntimes(env *models.Environment, repoPath string, subDir string) ([]DevRuntimeConfig, error) {
	configs, err := DetectDevRuntimes(repoPath, subDir)

	// Apply DB overrides for all candidates
	for i := range configs {
		if env.StartCommand != nil && *env.StartCommand != "" {
			configs[i].StartCmd = *env.StartCommand
			err = nil // Clear heuristic errors if explicit command given
		}
		if env.Port != nil && *env.Port > 0 {
			configs[i].ExposedPort = fmt.Sprintf("%d", *env.Port)
		}
	}

	return configs, err
}

func DetectDevRuntimes(repoPath string, subDir string) ([]DevRuntimeConfig, error) {
	appDir := filepath.Join(repoPath, subDir)
	var configs []DevRuntimeConfig

	// 1. Exact contract
	if res, err := ExactContractDetector(appDir, subDir); err == nil {
		return []DevRuntimeConfig{res.Config}, nil
	}

	// 2. Unsupported detector explicitly fails
	if _, err := UnsupportedDetector(appDir, subDir); err != nil && err.Error() != "unknown" {
		return nil, err
	}

	// 3. Fallbacks
	detectors := []Detector{
		StaticHTMLDetector,
		NodeDetector,
		PythonDetector,
		GoDetector,
	}

	var fallbackErr error
	for _, d := range detectors {
		res, err := d(appDir, subDir)
		if err == nil {
			configs = append(configs, res.Config)
		} else if err.Error() == "monorepo detected: please select a workspace package manually" {
			fallbackErr = err
			break
		}
	}

	if fallbackErr != nil {
		return nil, fallbackErr
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf("no supported language detected")
	}

	// Read sandbox.toml for overrides (apply to the first candidate)
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
				configs[0].BaseImage = override.BaseImage
			}
			if override.InstallCmd != "" {
				configs[0].InstallCmd = override.InstallCmd
			}
			if override.StartCmd != "" {
				configs[0].StartCmd = override.StartCmd
			}
			if override.WorkDir != "" {
				configs[0].WorkDir = override.WorkDir
			}
			if override.ExposedPort != "" {
				configs[0].ExposedPort = override.ExposedPort
			}
		}
	}

	return configs, nil
}

// Old detect* functions replaced by plugins

func GenerateSandboxStartScript(config DevRuntimeConfig) string {
	script := `#!/bin/sh
set -e

# Change to the application directory
cd ` + config.WorkDir + `

echo "========================================="
echo "🛠️  Setting up Dev Sandbox Runtime"
echo "========================================="
echo "Working Directory: ` + config.WorkDir + `"

export HOST=0.0.0.0
export PORT=` + config.ExposedPort + `

echo "📦 Installing dependencies..."
` + config.InstallCmd + `

echo "🚀 Starting application..."
` + config.StartCmd + `
`
	return script
}
