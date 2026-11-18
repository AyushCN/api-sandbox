package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DetectorResult struct {
	Config     DevRuntimeConfig
	Confidence int // 0 to 100
	Evidence   string
}

type Detector func(appDir, subDir string) (DetectorResult, error)

func getWorkDir(subDir string) string {
	if subDir != "" {
		return "/app/" + subDir
	}
	return "/app"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// 1. Dockerfile or devcontainer.json
func ExactContractDetector(appDir, subDir string) (DetectorResult, error) {
	if fileExists(filepath.Join(appDir, "Dockerfile")) {
		return DetectorResult{
			Config: DevRuntimeConfig{
				RuntimeType: "docker",
				BaseImage:   "docker-build", // Special flag for worker to build it
				WorkDir:     getWorkDir(subDir),
			},
			Confidence: 100,
			Evidence:   "Dockerfile found",
		}, nil
	}
	if fileExists(filepath.Join(appDir, ".devcontainer", "devcontainer.json")) {
		return DetectorResult{
			Config: DevRuntimeConfig{
				RuntimeType: "devcontainer",
				BaseImage:   "devcontainer-build",
				WorkDir:     getWorkDir(subDir),
			},
			Confidence: 100,
			Evidence:   "devcontainer.json found",
		}, nil
	}
	return DetectorResult{}, fmt.Errorf("no exact contract")
}

// 2. HTML/CSS/JS only
func StaticHTMLDetector(appDir, subDir string) (DetectorResult, error) {
	hasPackageJson := fileExists(filepath.Join(appDir, "package.json"))
	if hasPackageJson {
		return DetectorResult{}, fmt.Errorf("has package.json")
	}

	hasIndexRoot := fileExists(filepath.Join(appDir, "index.html"))
	hasIndexPublic := fileExists(filepath.Join(appDir, "public", "index.html"))

	if hasIndexRoot || hasIndexPublic {
		startCmd := "npx serve -l 3000 ."
		if hasIndexPublic {
			startCmd = "npx serve -l 3000 public"
		}
		return DetectorResult{
			Config: DevRuntimeConfig{
				RuntimeType: "static",
				BaseImage:   "node:20-alpine",
				InstallCmd:  "npm install -g serve",
				StartCmd:    startCmd,
				WatchHint:   "Static HTML. Refresh browser to see changes.",
				WorkDir:     getWorkDir(subDir),
				ExposedPort: "3000",
			},
			Confidence: 90,
			Evidence:   "index.html found without package.json",
		}, nil
	}

	return DetectorResult{}, fmt.Errorf("no index.html found")
}

// 3. Node.js
func NodeDetector(appDir, subDir string) (DetectorResult, error) {
	packageJsonPath := filepath.Join(appDir, "package.json")
	content, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return DetectorResult{}, err
	}

	installCmd := "npm install"
	if fileExists(filepath.Join(appDir, "yarn.lock")) {
		installCmd = "yarn install"
	} else if fileExists(filepath.Join(appDir, "pnpm-lock.yaml")) {
		installCmd = "pnpm install"
	} else if fileExists(filepath.Join(appDir, "bun.lockb")) {
		installCmd = "bun install"
	}

	var pkg map[string]interface{}
	_ = json.Unmarshal(content, &pkg)

	// Check monorepo
	if workspaces, ok := pkg["workspaces"]; ok && workspaces != nil {
		return DetectorResult{}, fmt.Errorf("monorepo detected: please select a workspace package manually")
	}

	startCmd := ""
	if scripts, ok := pkg["scripts"].(map[string]interface{}); ok {
		if start, ok := scripts["start"].(string); ok && start != "" {
			startCmd = "npm run start"
		} else if dev, ok := scripts["dev"].(string); ok && dev != "" {
			startCmd = "npm run dev"
		}
	}

	if startCmd == "" {
		if main, ok := pkg["main"].(string); ok && main != "" {
			startCmd = "node " + main
		} else if fileExists(filepath.Join(appDir, "index.js")) {
			startCmd = "node index.js"
		} else if fileExists(filepath.Join(appDir, "server.js")) {
			startCmd = "node server.js"
		} else {
			startCmd = "node index.js" // fallback
		}
	}

	if strings.HasPrefix(startCmd, "npm run") {
		if strings.HasPrefix(installCmd, "yarn") {
			startCmd = strings.Replace(startCmd, "npm run", "yarn", 1)
		} else if strings.HasPrefix(installCmd, "pnpm") {
			startCmd = strings.Replace(startCmd, "npm run", "pnpm", 1)
		} else if strings.HasPrefix(installCmd, "bun") {
			startCmd = strings.Replace(startCmd, "npm run", "bun run", 1)
		}
	}

	baseImage := "node:20-alpine"
	if strings.HasPrefix(installCmd, "bun") {
		baseImage = "oven/bun:1-alpine"
	}

	return DetectorResult{
		Config: DevRuntimeConfig{
			RuntimeType: "node",
			BaseImage:   baseImage,
			InstallCmd:  installCmd,
			StartCmd:    startCmd,
			WatchHint:   "Node.js detected.",
			WorkDir:     getWorkDir(subDir),
			ExposedPort: "3000",
		},
		Confidence: 80,
		Evidence:   "package.json found",
	}, nil
}

// 4. Python
func PythonDetector(appDir, subDir string) (DetectorResult, error) {
	installCmd := ""
	evidence := ""

	hasReqTxt := fileExists(filepath.Join(appDir, "requirements.txt"))
	hasPyProject := fileExists(filepath.Join(appDir, "pyproject.toml"))
	hasPipfile := fileExists(filepath.Join(appDir, "Pipfile"))
	hasEnvYml := fileExists(filepath.Join(appDir, "environment.yml"))

	if hasReqTxt {
		installCmd = "pip install -r requirements.txt"
		evidence = "requirements.txt found"
	} else if hasPyProject {
		installCmd = "pip install ."
		evidence = "pyproject.toml found"
	} else if hasPipfile {
		installCmd = "pip install pipenv && pipenv install --system"
		evidence = "Pipfile found"
	} else if hasEnvYml {
		installCmd = "conda env create -f environment.yml && conda activate" // simplified
		evidence = "environment.yml found"
	}

	files, _ := filepath.Glob(filepath.Join(appDir, "*.py"))
	if installCmd == "" {
		if len(files) == 1 {
			installCmd = "echo 'No dependencies'"
			evidence = "single .py file found"
		} else {
			return DetectorResult{}, fmt.Errorf("no python dependencies or single script found")
		}
	}

	startCmd := "python main.py"

	reqStr := ""
	if hasReqTxt {
		b, _ := os.ReadFile(filepath.Join(appDir, "requirements.txt"))
		reqStr = strings.ToLower(string(b))
	} else if hasPyProject {
		b, _ := os.ReadFile(filepath.Join(appDir, "pyproject.toml"))
		reqStr = strings.ToLower(string(b))
	}

	if strings.Contains(reqStr, "django") || fileExists(filepath.Join(appDir, "manage.py")) {
		startCmd = "python manage.py runserver 0.0.0.0:8000"
		installCmd += " && pip install django"
	} else if strings.Contains(reqStr, "fastapi") || (len(files) > 0 && containsText(files, "FastAPI()")) {
		if fileExists(filepath.Join(appDir, "main.py")) {
			startCmd = "uvicorn main:app --host 0.0.0.0 --reload"
		} else {
			startCmd = "uvicorn app.main:app --host 0.0.0.0 --reload"
		}
		installCmd += " && pip install uvicorn[standard] fastapi"
	} else if strings.Contains(reqStr, "flask") || (len(files) > 0 && containsText(files, "Flask(__name__)")) {
		if fileExists(filepath.Join(appDir, "app.py")) {
			startCmd = "FLASK_APP=app.py flask run --host=0.0.0.0 --port=8000 --reload"
		} else if fileExists(filepath.Join(appDir, "main.py")) {
			startCmd = "FLASK_APP=main.py flask run --host=0.0.0.0 --port=8000 --reload"
		}
		installCmd += " && pip install flask"
	} else if len(files) == 1 {
		startCmd = "python " + filepath.Base(files[0])
	} else if fileExists(filepath.Join(appDir, "app.py")) {
		startCmd = "python app.py"
	}

	return DetectorResult{
		Config: DevRuntimeConfig{
			RuntimeType: "python",
			BaseImage:   "python:3.11-slim",
			InstallCmd:  installCmd,
			StartCmd:    startCmd,
			WatchHint:   "Python detected.",
			WorkDir:     getWorkDir(subDir),
			ExposedPort: "8000",
		},
		Confidence: 80,
		Evidence:   evidence,
	}, nil
}

// 5. Go
func GoDetector(appDir, subDir string) (DetectorResult, error) {
	if fileExists(filepath.Join(appDir, "go.mod")) {
		return DetectorResult{
			Config: DevRuntimeConfig{
				RuntimeType: "go",
				BaseImage:   "golang:alpine",
				InstallCmd:  "go mod download",
				StartCmd:    "go run .",
				WatchHint:   "Go detected.",
				WorkDir:     getWorkDir(subDir),
				ExposedPort: "8080",
			},
			Confidence: 80,
			Evidence:   "go.mod found",
		}, nil
	}
	return DetectorResult{}, fmt.Errorf("no go.mod found")
}

// 6. Unsupported Language
func UnsupportedDetector(appDir, subDir string) (DetectorResult, error) {
	if fileExists(filepath.Join(appDir, "Cargo.toml")) {
		return DetectorResult{}, fmt.Errorf("Detected Rust - not yet supported")
	}
	if fileExists(filepath.Join(appDir, "pom.xml")) || fileExists(filepath.Join(appDir, "build.gradle")) {
		return DetectorResult{}, fmt.Errorf("Detected Java - not yet supported")
	}
	if fileExists(filepath.Join(appDir, "composer.json")) {
		return DetectorResult{}, fmt.Errorf("Detected PHP - not yet supported")
	}
	return DetectorResult{}, fmt.Errorf("unknown")
}

func containsText(files []string, text string) bool {
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err == nil && strings.Contains(string(b), text) {
			return true
		}
	}
	return false
}
