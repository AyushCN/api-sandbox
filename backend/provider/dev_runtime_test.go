package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectDevRuntime(t *testing.T) {
	// Create a temporary directory for our mock repos
	tmpDir, err := os.MkdirTemp("", "dev_runtime_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name         string
		setupFiles   map[string]string
		subDir       string
		expectedBase string
		expectedInst string
		expectedStrt string
	}{
		{
			name: "Node.js Vanilla with npm",
			setupFiles: map[string]string{
				"package.json": `{"scripts": {"start": "node index.js"}}`,
			},
			expectedBase: "node:20-alpine",
			expectedInst: "npm install",
			expectedStrt: "node --watch index.js",
		},
		{
			name: "Node.js Next.js with yarn",
			setupFiles: map[string]string{
				"package.json": `{"dependencies": {"next": "latest"}, "scripts": {"dev": "next dev"}}`,
				"yarn.lock":    "",
			},
			expectedBase: "node:20-alpine",
			expectedInst: "yarn install",
			expectedStrt: "yarn dev",
		},
		{
			name: "Node.js Bun",
			setupFiles: map[string]string{
				"package.json": `{"scripts": {"dev": "bun index.ts"}}`,
				"bun.lockb":    "",
			},
			expectedBase: "oven/bun:1-alpine",
			expectedInst: "bun install",
			expectedStrt: "bun run dev",
		},
		{
			name: "Python FastAPI",
			setupFiles: map[string]string{
				"requirements.txt": "fastapi\nuvicorn\n",
				"main.py":          "",
			},
			expectedBase: "python:3.11-slim",
			expectedInst: "pip install -r requirements.txt && pip install uvicorn[standard]",
			expectedStrt: "uvicorn main:app --host 0.0.0.0 --reload --reload-dir .",
		},
		{
			name: "Python Django",
			setupFiles: map[string]string{
				"requirements.txt": "django\n",
				"manage.py":        "",
			},
			expectedBase: "python:3.11-slim",
			expectedInst: "pip install -r requirements.txt && pip install uvicorn[standard]",
			expectedStrt: "python manage.py runserver 0.0.0.0:8000",
		},
		{
			name: "Go Module",
			setupFiles: map[string]string{
				"go.mod": "module example.com/m\n\ngo 1.22\n",
			},
			expectedBase: "golang:1.22-alpine",
			expectedInst: "go mod download && go install github.com/air-verse/air@latest",
			expectedStrt: "if [ ! -f .air.toml ]; then air init; fi && air || go run .",
		},
		{
			name: "Ruby on Rails",
			setupFiles: map[string]string{
				"Gemfile": "source 'https://rubygems.org'\ngem 'rails'\n",
			},
			expectedBase: "ruby:3.3-alpine",
			expectedInst: "bundle install",
			expectedStrt: "bin/rails server -b 0.0.0.0",
		},
		{
			name: "PHP Laravel",
			setupFiles: map[string]string{
				"composer.json": `{"require": {"laravel/framework": "^10.0"}}`,
			},
			expectedBase: "php:8.2-cli-alpine",
			expectedInst: "apk add composer && composer install",
			expectedStrt: "php artisan serve --host=0.0.0.0 --port=8000",
		},
		{
			name: "Rust Cargo",
			setupFiles: map[string]string{
				"Cargo.toml": "[package]\nname = \"example\"\n",
			},
			expectedBase: "rust:1-slim",
			expectedInst: "cargo install cargo-watch",
			expectedStrt: "cargo watch -x run",
		},
		{
			name: "sandbox.toml Override",
			setupFiles: map[string]string{
				"package.json": `{}`,
				"sandbox.toml": `
base_image = "custom-node:18"
install_cmd = "npm ci"
start_cmd = "npm run custom"
`,
			},
			expectedBase: "custom-node:18",
			expectedInst: "npm ci",
			expectedStrt: "npm run custom",
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoDir := filepath.Join(tmpDir, "repo"+string(rune('A'+i)))
			err := os.MkdirAll(repoDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create repo dir: %v", err)
			}

			for file, content := range tc.setupFiles {
				err := os.WriteFile(filepath.Join(repoDir, file), []byte(content), 0644)
				if err != nil {
					t.Fatalf("Failed to write mock file %s: %v", file, err)
				}
			}

			config, err := DetectDevRuntime(repoDir, "")
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if config.BaseImage != tc.expectedBase {
				t.Errorf("Expected BaseImage %q, got %q", tc.expectedBase, config.BaseImage)
			}
			if config.InstallCmd != tc.expectedInst {
				t.Errorf("Expected InstallCmd %q, got %q", tc.expectedInst, config.InstallCmd)
			}
			if config.StartCmd != tc.expectedStrt {
				t.Errorf("Expected StartCmd %q, got %q", tc.expectedStrt, config.StartCmd)
			}
		})
	}
}
