package worker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/api-sandbox/backend/provider"
)

const (
	runtimeStartupTimeout = 10 * time.Minute
	runtimeReadinessPoll  = time.Second
	runtimeProbeTimeout   = 2 * time.Second
)

type containerHealthCheck func(context.Context) (bool, string, error)
type runtimeHTTPProbe func(context.Context) (int, error)
type runtimeTCPProbe func(context.Context) error

func waitForRuntimeReadiness(ctx context.Context, healthType string, checkContainer containerHealthCheck, probeHTTP runtimeHTTPProbe, probeTCP runtimeTCPProbe, pollInterval time.Duration) (string, error) {
	if pollInterval <= 0 {
		pollInterval = runtimeReadinessPoll
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		running, logs, err := checkContainer(ctx)
		if err != nil {
			return logs, fmt.Errorf("inspect runtime process: %w", err)
		}
		if !running {
			if strings.TrimSpace(logs) == "" {
				logs = "runtime container exited before becoming ready"
			}
			return logs, fmt.Errorf("runtime container exited before becoming ready")
		}
		if healthType == "none" {
			return "", nil
		}

		switch healthType {
		case "tcp":
			if probeTCP != nil && probeTCP(ctx) == nil {
				return "", nil
			}
		default: // "http" and legacy/unknown values require a successful preview response.
			statusCode, probeErr := probeHTTP(ctx)
			if probeErr == nil && provider.IsHTTPReadyStatus(statusCode) {
				return "", nil
			}
		}

		select {
		case <-ctx.Done():
			return logs, fmt.Errorf("runtime did not become ready within %s: %w", runtimeStartupTimeout, ctx.Err())
		case <-ticker.C:
		}
	}
}

func probeRuntimeHTTP(ctx context.Context, envID, domain string) (int, error) {
	traefikURL := os.Getenv("TRAEFIK_URL")
	if traefikURL == "" {
		traefikURL = "http://api-sandbox-traefik"
	}
	return probeRuntimeHTTPWithClient(ctx, envID, domain, traefikURL, newRuntimeHTTPClient())
}

func newRuntimeHTTPClient() *http.Client {
	return &http.Client{
		Timeout: runtimeProbeTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func probeRuntimeHTTPWithClient(ctx context.Context, envID, domain, traefikURL string, client *http.Client) (int, error) {
	reqCtx, cancel := context.WithTimeout(ctx, runtimeProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, strings.TrimRight(traefikURL, "/")+"/", nil)
	if err != nil {
		return 0, err
	}
	req.Host = envID + "." + domain
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	_, _ = io.CopyN(io.Discard, resp.Body, 4096)
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}
