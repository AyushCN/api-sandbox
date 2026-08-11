package provider

import (
	"fmt"
)

// BuildpackManager manages a registry of available buildpacks
type BuildpackManager struct {
	buildpacks []Buildpack
}

// NewBuildpackManager initializes a new BuildpackManager with the standard buildpacks
func NewBuildpackManager() *BuildpackManager {
	return &BuildpackManager{
		buildpacks: []Buildpack{
			&NodeJSBuildpack{},
			&PythonBuildpack{},
			&GolangBuildpack{},
			&RubyBuildpack{},
		},
	}
}

// Detect scans the repoPath and returns the first buildpack that supports it
func (m *BuildpackManager) Detect(repoPath string) (Buildpack, error) {
	for _, bp := range m.buildpacks {
		if bp.Detect(repoPath) {
			return bp, nil
		}
	}
	return nil, fmt.Errorf("no suitable buildpack found for repository at %s", repoPath)
}
