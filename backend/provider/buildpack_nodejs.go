package provider

import (
	"context"
	"os"
	"path/filepath"
)

type NodeJSBuildpack struct{}

func (b *NodeJSBuildpack) Detect(repoPath string) bool {
	_, err := os.Stat(filepath.Join(repoPath, "package.json"))
	return err == nil
}

func (b *NodeJSBuildpack) Build(ctx context.Context, repoPath string) (string, error) {
	dockerfile := `FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --production || npm install --production
COPY . .
EXPOSE 3000
CMD ["npm", "start"]`

	err := os.WriteFile(filepath.Join(repoPath, "Dockerfile"), []byte(dockerfile), 0644)
	if err != nil {
		return "", err
	}
	return "Dockerfile", nil
}

func (b *NodeJSBuildpack) GetPort(repoPath string) int {
	return 3000
}
