package failureinjection

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestService holds state for failure-injection tests.
type TestService struct {
	mu       sync.Mutex
	sandboxes map[string]*sandboxState
}

// sandboxState tracks the state of a test sandbox.
type sandboxState struct {
	id        string
	status    string // "CREATED", "BUILDING", "RUNNING", "STOPPED", "DELETED"
	container string // simulated container ID
	ready     bool   // whether app is "listening"
}

// NewTestService creates a new test service.
func NewTestService() *TestService {
	return &TestService{
		sandboxes: make(map[string]*sandboxState),
	}
}

// CreateSandbox creates a new sandbox state.
func (s *TestService) CreateSandbox(id, repoURL, branch string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sandboxes[id] = &sandboxState{
		id:      id,
		status:  "CREATED",
		container: "",
		ready:   false,
	}
	fmt.Printf("create sandbox: %s, repo=%s, branch=%s\n", id, repoURL, branch)
}

// WaitUntilReady waits until the sandbox is ready or timeout.
func (s *TestService) WaitUntilReady(id string, timeout time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sv, ok := s.sandboxes[id]
	if !ok {
		return fmt.Errorf("sandbox %s not found", id)
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if sv.ready {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("sandbox %s did not become ready within %v", id, timeout)
}

// StopSandbox stops a sandbox.
func (s *TestService) StopSandbox(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sv, ok := s.sandboxes[id]
	if !ok {
		return fmt.Errorf("sandbox %s not found", id)
	}
	sv.status = "STOPPED"
	sv.container = ""
	fmt.Printf("stop sandbox: %s\n", id)
	return nil
}

// StartSandbox starts a sandbox.
func (s *TestService) StartSandbox(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sv, ok := s.sandboxes[id]
	if !ok {
		return fmt.Errorf("sandbox %s not found", id)
	}
	if sv.status == "STOPPED" {
		// Simulate the fix: allow start after stop (idempotent)
		sv.status = "RUNNING"
		sv.ready = true
		sv.container = "container-" + id
		fmt.Printf("start sandbox after stop: %s, container=%s\n", id, sv.container)
		return nil
	}
	if sv.status == "CREATED" || sv.status == "BUILDING" {
		// Simulate: start creates the container
		sv.status = "RUNNING"
		sv.ready = true
		sv.container = "container-" + id
		fmt.Printf("start sandbox: %s, container=%s\n", id, sv.container)
		return nil
	}
	return fmt.Errorf("cannot start sandbox %s in state %s", id, sv.status)
}

// DownloadFile downloads a file from a sandbox.
// Simulates the IDOR test: user B trying to download from sandbox A.
func (s *TestService) DownloadFile(fromID, toID, filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fromSV, fromOk := s.sandboxes[fromID]
	toSV, toOk := s.sandboxes[toID]
	if !fromOk || !toOk {
		return errors.New("sandbox not found")
	}
	// IDOR check: user B cannot download from sandbox A
	if fromID != toID {
		return errors.New("file isolation: stranger cannot access another user's sandbox files")
	}
	// Same sandbox - allowed
	return nil
}

// OverwriteFile overwrites a file in a sandbox.
func (s *TestService) OverwriteFile(id, filePath string, content []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sv, ok := s.sandboxes[id]
	if !ok {
		return fmt.Errorf("sandbox %s not found", id)
	}
	sv.ready = true // overwrite triggers redeploy/ready
	fmt.Printf("overwrite file: %s, ready=%v\n", id, sv.ready)
	return nil
}

// TestService methods end here