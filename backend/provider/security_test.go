package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoHardcodedSandboxNetwork(t *testing.T) {
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || path == "security_test.go" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if strings.Contains(string(content), "api-sandbox-network") {
			t.Errorf("File %s contains hardcoded 'api-sandbox-network' string. This is a security violation of multi-tenant isolation.", path)
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk provider directory: %v", err)
	}
}
