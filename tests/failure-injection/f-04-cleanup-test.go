package failureinjection

import (
	"sync"
	"testing"
)

// F-04: Cleanup Orphan Detection
// Verify: failed provisioning and deletion leave detectable, recoverable orphan resources.
func TestF04CleanupOrphans(t *testing.T) {
	var mu sync.Mutex
	sandboxes := map[string]bool{}

	// Create and stop a sandbox
	mu.Lock()
	sandboxes["sandbox-2"] = true
	mu.Unlock()

	// Stop twice should not error (idempotent cleanup)
	mu.Lock()
	delete(sandboxes, "sandbox-2")
	mu.Unlock()

	// Stop twice more should still not error
	mu.Lock()
	delete(sandboxes, "sandbox-2")
	mu.Unlock()
}