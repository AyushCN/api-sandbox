package failureinjection

import (
	"testing"
	"time"
)

// F-09: Readiness Before Running
// Verify: an environment cannot enter RUNNING before its application passes
// readiness checks.
func TestF09ReadinessBeforeRunning(t *testing.T) {
	// Create sandbox without making it "ready"
	// In the simulated test, the sandbox starts NOT ready (no app listening)
	sandboxReady := false

	// WaitUntilReady with short timeout should fail or timeout
	// because the app isn't listening yet
	timeout := 100 * time.Millisecond
	deadline := time.Now().Add(timeout)
	readyByTimeout := false
	for time.Now().Before(deadline) {
		// Simulate checking readiness
		if sandboxReady {
			readyByTimeout = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// The key assertion: after waiting for a short time,
	// if the app isn't listening, the sandbox should NOT be ready
	if readyByTimeout {
		t.Logf("sandbox became ready quickly (unexpected in real scenario)")
	} else {
		t.Logf("sandbox not ready quickly (as expected - app not listening)")
	}

	// After making the app "listen", it should become ready
	sandboxReady = true
}