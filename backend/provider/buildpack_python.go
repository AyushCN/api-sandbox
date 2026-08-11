package provider

import (
	"context"
	"os"
	"path/filepath"
)

type PythonBuildpack struct{}

func (b *PythonBuildpack) Detect(repoPath string) bool {
	files := []string{"requirements.txt", "Pipfile", "pyproject.toml", "setup.py"}
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(repoPath, f)); err == nil {
			return true
		}
	}
	return false
}

func (b *PythonBuildpack) Build(ctx context.Context, repoPath string) (string, error) {
	dockerfile := `FROM python:3.11-slim
WORKDIR /app
COPY . .
RUN if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; fi
EXPOSE 8000
CMD ["python", "app.py"]`

	err := os.WriteFile(filepath.Join(repoPath, "Dockerfile"), []byte(dockerfile), 0644)
	if err != nil {
		return "", err
	}
	return "Dockerfile", nil
}

func (b *PythonBuildpack) GetPort(repoPath string) int {
	return 8000
}
