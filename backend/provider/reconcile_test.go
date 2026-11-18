package provider

import (
	"testing"
	"time"

	"github.com/api-sandbox/backend/models"
)

func TestNeedsRuntimeFailure(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name           string
		status         models.EnvironmentStatus
		found          bool
		containerState string
		updatedAt      time.Time
		want           bool
	}{
		{name: "running container remains running", status: models.StatusRunning, found: true, containerState: "running", updatedAt: now},
		{name: "running container missing", status: models.StatusRunning, updatedAt: now, want: true},
		{name: "running container stopped", status: models.StatusRunning, found: true, containerState: "exited", updatedAt: now, want: true},
		{name: "recent building with old container does not fail", status: models.StatusBuilding, found: true, containerState: "running", updatedAt: now.Add(-time.Minute)},
		{name: "stale building without container fails", status: models.StatusBuilding, updatedAt: now.Add(-61 * time.Minute), want: true},
		{name: "terminal environment is not classified as active failure", status: models.StatusFailed, found: true, containerState: "running", updatedAt: now},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsRuntimeFailure(tt.status, tt.found, tt.containerState, tt.updatedAt, now)
			if got != tt.want {
				t.Fatalf("needsRuntimeFailure() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsOrphanEnvironment(t *testing.T) {
	for _, tt := range []struct {
		name   string
		env    models.Environment
		orphan bool
	}{
		{name: "running record", env: models.Environment{ID: "e1", Status: models.StatusRunning}},
		{name: "building record", env: models.Environment{ID: "e1", Status: models.StatusBuilding}},
		{name: "missing record", env: models.Environment{}, orphan: true},
		{name: "terminal record", env: models.Environment{ID: "e1", Status: models.StatusFailed}, orphan: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOrphanEnvironment(tt.env); got != tt.orphan {
				t.Fatalf("isOrphanEnvironment() = %v, want %v", got, tt.orphan)
			}
		})
	}
}
