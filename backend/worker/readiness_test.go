package worker

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWaitForRuntimeReadinessHTTPStatuses(t *testing.T) {
	tests := []struct {
		status int
		ready  bool
	}{
		{status: 200, ready: true},
		{status: 404},
		{status: 403},
		{status: 500},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			_, err := waitForRuntimeReadiness(ctx, "http",
				func(context.Context) (bool, string, error) { return true, "", nil },
				func(context.Context) (int, error) { return tt.status, nil }, nil, time.Millisecond)
			if tt.ready && err != nil {
				t.Fatalf("status %d should be ready: %v", tt.status, err)
			}
			if !tt.ready && err == nil {
				t.Fatalf("status %d must not be ready", tt.status)
			}
		})
	}
}

func TestWaitForRuntimeReadinessConnectionFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := waitForRuntimeReadiness(ctx, "http",
		func(context.Context) (bool, string, error) { return true, "", nil },
		func(context.Context) (int, error) { return 0, errors.New("connection refused") }, nil, time.Millisecond)
	if err == nil {
		t.Fatal("connection failure must not be ready")
	}
}

func TestWaitForRuntimeReadinessDetectsProcessExit(t *testing.T) {
	checks := 0
	logs, err := waitForRuntimeReadiness(context.Background(), "http",
		func(context.Context) (bool, string, error) {
			checks++
			if checks == 1 {
				return true, "", nil
			}
			return false, "application exited with status 7", nil
		}, func(context.Context) (int, error) { return 502, nil }, nil, time.Millisecond)
	if err == nil || !strings.Contains(logs, "status 7") {
		t.Fatalf("process exit should fail readiness with logs, got logs=%q err=%v", logs, err)
	}
}

func TestWaitForRuntimeReadinessNoneRequiresLiveProcessOnly(t *testing.T) {
	probeCalled := false
	_, err := waitForRuntimeReadiness(context.Background(), "none",
		func(context.Context) (bool, string, error) { return true, "", nil },
		func(context.Context) (int, error) { probeCalled = true; return 500, nil },
		func(context.Context) error { probeCalled = true; return errors.New("should not probe") }, time.Millisecond)
	if err != nil || probeCalled {
		t.Fatalf("none policy should only require a live process; probeCalled=%v err=%v", probeCalled, err)
	}
}

func TestWaitForRuntimeReadinessNoneStillDetectsExit(t *testing.T) {
	_, err := waitForRuntimeReadiness(context.Background(), "none",
		func(context.Context) (bool, string, error) { return false, "worker exited", nil },
		func(context.Context) (int, error) { t.Fatal("HTTP probe called for none policy"); return 0, nil },
		func(context.Context) error { t.Fatal("TCP probe called for none policy"); return nil }, time.Millisecond)
	if err == nil {
		t.Fatal("none policy must still require the container process to be alive")
	}
}

func TestWaitForRuntimeReadinessTCPPolicy(t *testing.T) {
	t.Run("open port is ready", func(t *testing.T) {
		httpCalled := false
		_, err := waitForRuntimeReadiness(context.Background(), "tcp",
			func(context.Context) (bool, string, error) { return true, "", nil },
			func(context.Context) (int, error) { httpCalled = true; return 500, nil },
			func(context.Context) error { return nil }, time.Millisecond)
		if err != nil || httpCalled {
			t.Fatalf("TCP policy err=%v, HTTP probe called=%v", err, httpCalled)
		}
	})
	t.Run("closed port is not ready", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err := waitForRuntimeReadiness(ctx, "tcp",
			func(context.Context) (bool, string, error) { return true, "", nil }, nil,
			func(context.Context) error { return errors.New("connection refused") }, time.Millisecond)
		if err == nil {
			t.Fatal("closed TCP port must not be ready")
		}
	})
}

func TestProbeRuntimeHTTPUsesConfiguredHostAndReturnsStatus(t *testing.T) {
	client := newRuntimeHTTPClient()
	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Host != "env-1.example.test" {
			t.Errorf("Host = %q, want env-1.example.test", req.Host)
		}
		if req.URL.Path != "/" {
			t.Errorf("path = %q, want /", req.URL.Path)
		}
		return testHTTPResponse(req, http.StatusNotFound, ""), nil
	})
	status, err := probeRuntimeHTTPWithClient(context.Background(), "env-1", "example.test", "http://traefik", client)
	if err != nil || status != http.StatusNotFound {
		t.Fatalf("probe = %d, %v; want 404 and no transport error", status, err)
	}
}

func TestProbeRuntimeHTTPDoesNotFollowRedirects(t *testing.T) {
	client := newRuntimeHTTPClient()
	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return testHTTPResponse(req, http.StatusFound, "/target"), nil
	})
	status, err := probeRuntimeHTTPWithClient(context.Background(), "env-1", "example.test", "http://traefik", client)
	if err != nil || status != http.StatusFound {
		t.Fatalf("probe followed or lost redirect: status=%d err=%v", status, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func testHTTPResponse(req *http.Request, status int, location string) *http.Response {
	header := make(http.Header)
	if location != "" {
		header.Set("Location", location)
	}
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader("")), Request: req}
}
