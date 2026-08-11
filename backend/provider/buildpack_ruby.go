package provider

import (
	"context"
	"os"
	"path/filepath"
)

type RubyBuildpack struct{}

func (b *RubyBuildpack) Detect(repoPath string) bool {
	if _, err := os.Stat(filepath.Join(repoPath, "Gemfile")); err == nil {
		return true
	}
	return false
}

func (b *RubyBuildpack) Build(ctx context.Context, repoPath string) (string, error) {
	dockerfile := `FROM ruby:3.2-alpine
WORKDIR /app
RUN apk add --no-cache build-base
COPY Gemfile* ./
RUN bundle install
COPY . .
EXPOSE 3000
CMD ["bundle", "exec", "ruby", "app.rb"]`

	err := os.WriteFile(filepath.Join(repoPath, "Dockerfile"), []byte(dockerfile), 0644)
	if err != nil {
		return "", err
	}
	return "Dockerfile", nil
}

func (b *RubyBuildpack) GetPort(repoPath string) int {
	return 3000
}
