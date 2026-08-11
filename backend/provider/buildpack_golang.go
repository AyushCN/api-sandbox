package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

type GolangBuildpack struct{}

func (b *GolangBuildpack) Detect(repoPath string) bool {
	if _, err := os.Stat(filepath.Join(repoPath, "go.mod")); err == nil {
		return true
	}

	// Also detect if there are any .go files in the root
	files, err := os.ReadDir(repoPath)
	if err == nil {
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".go") {
				return true
			}
		}
	}

	return false
}

func (b *GolangBuildpack) Build(ctx context.Context, repoPath string) (string, error) {
	dockerfile := `FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN if [ -f go.mod ]; then go mod download; fi
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]`

	err := os.WriteFile(filepath.Join(repoPath, "Dockerfile"), []byte(dockerfile), 0644)
	if err != nil {
		return "", err
	}
	return "Dockerfile", nil
}

func (b *GolangBuildpack) GetPort(repoPath string) int {
	return 8080
}
