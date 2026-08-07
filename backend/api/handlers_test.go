package api

import (
	"testing"
)

func TestValidateWorkspacePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"Valid File", "src/index.js", false},
		{"Valid Folder", "src", false},
		{"Valid Deep File", "components/ui/button.tsx", false},

		{"Path Traversal (Simple)", "../src", true},
		{"Path Traversal (Nested)", "src/../../package.json", true},
		{"Absolute Path", "/etc/passwd", true},

		{"Root Directory Attempt (.)", ".", true},
		{"Root Directory Attempt (/)", "/", true},
		{"Empty Path", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkspacePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkspacePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}
