package provider

import (
	"os"
	"path/filepath"
	"strings"
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
			expectedStrt: "npm run start",
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
			expectedInst: "pip install -r requirements.txt && pip install uvicorn[standard] fastapi",
			expectedStrt: "uvicorn main:app --host 0.0.0.0 --reload",
		},
		{
			name: "Python Django",
			setupFiles: map[string]string{
				"requirements.txt": "django\n",
				"manage.py":        "",
			},
			expectedBase: "python:3.11-slim",
			expectedInst: "pip install -r requirements.txt && pip install django",
			expectedStrt: "python manage.py runserver 0.0.0.0:8000",
		},
		{
			name: "Go Module",
			setupFiles: map[string]string{
				"go.mod": "module example.com/m\n\ngo 1.22\n",
			},
			expectedBase: "golang:alpine",
			expectedInst: "go mod download",
			expectedStrt: "go run .",
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

			configs, err := DetectDevRuntimes(repoDir, "")
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			if len(configs) == 0 {
				t.Fatalf("Expected at least one config, got none")
			}
			config := configs[0]

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

func TestGenerateSandboxStartScriptRunsInstallThenExecsApplication(t *testing.T) {
	script := GenerateSandboxStartScript(DevRuntimeConfig{
		WorkDir:    "/app/project's source",
		InstallCmd: "npm install && echo ready",
		StartCmd:   "npm run dev",
	})
	installAt := strings.Index(script, "/bin/sh -c 'npm install && echo ready'")
	startAt := strings.Index(script, "exec /bin/sh -c 'npm run dev'")
	if installAt < 0 || startAt < 0 || installAt >= startAt {
		t.Fatalf("script must finish dependency installation before execing the app:\n%s", script)
	}
	if !strings.Contains(script, `cd '/app/project'\''s source'`) {
		t.Fatalf("working directory was not safely quoted:\n%s", script)
	}
}

func TestNormalizeRuntimeWorkDirStaysInsideApp(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
		bad   bool
	}{
		{input: "", want: "/app"},
		{input: "src", want: "/app/src"},
		{input: "/app/web", want: "/app/web"},
		{input: "/workspaces/env-1/repo", want: "/app/repo"},
		{input: "../../etc", bad: true},
		{input: "/etc", bad: true},
	} {
		got, err := NormalizeRuntimeWorkDir(tc.input, "env-1")
		if tc.bad {
			if err == nil {
				t.Errorf("NormalizeRuntimeWorkDir(%q) unexpectedly succeeded with %q", tc.input, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("NormalizeRuntimeWorkDir(%q) = %q, %v; want %q", tc.input, got, err, tc.want)
		}
	}
}
