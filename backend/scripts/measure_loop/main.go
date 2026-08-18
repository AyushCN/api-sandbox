package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

var (
	appURL        = "http://localhost"
	wsURL         = "ws://localhost"
	sessionCookie = ""
	jwtSecret     = "testsecret"
)

type EnvResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func main() {
	if os.Getenv("JWT_SECRET") != "" {
		jwtSecret = os.Getenv("JWT_SECRET")
	}

	sessionCookie = os.Getenv("SESSION_COOKIE")
	if sessionCookie == "" {
		// Generate a fake token for a dummy user
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"userId": "u1",
			"exp":    time.Now().Add(time.Hour * 24).Unix(),
		})
		sessionCookie, _ = token.SignedString([]byte(jwtSecret))
		fmt.Println("Generated fake SESSION_COOKIE")
	}

	appURLStr := os.Getenv("APP_URL")
	if appURLStr != "" {
		appURL = appURLStr
	}
	wsURLStr := os.Getenv("WS_URL")
	if wsURLStr != "" {
		wsURL = wsURLStr
	}

	apps := []struct {
		name  string
		repo  string
		files map[string]string
	}{
		{
			name: "Node Express",
			repo: "https://github.com/render-examples/express-hello-world.git",
			files: map[string]string{
				"app.js": `const express = require("express");
const app = express();
const port = process.env.PORT || 3001;

app.get("/", (req, res) => res.type('html').send('<html><body><h1>Hello World V%d</h1></body></html>'));

const server = app.listen(port, () => console.log('Ready'));
`,
			},
		},
		{
			name: "Python FastAPI",
			repo: "https://github.com/render-examples/fastapi.git",
			files: map[string]string{
				"main.py": `from typing import Union
from fastapi import FastAPI
app = FastAPI()

@app.get("/")
def read_root():
    return {"Hello": "World V%d"}
`,
			},
		},
		{
			name: "Go Basic",
			repo: "https://github.com/render-examples/go-gin-web-server.git",
			files: map[string]string{
				"main.go": `package main

import (
	"net/http"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong V%d",
		})
	})
	r.Run()
}
`,
			},
		},
	}

	isCold := false
	if len(os.Args) > 1 && os.Args[1] == "--cold" {
		isCold = true
	}

	for _, app := range apps {
		if isCold {
			runColdBenchmark(app.name, app.repo)
		} else {
			fmt.Printf("\n=== Benchmarking %s ===\n", app.name)
			runBenchmark(app.name, app.repo, app.files)
		}
	}
}

func runColdBenchmark(name string, repo string) {
	fmt.Printf("\n=== Cold-Start Benchmarking %s ===\n", name)
	cycles := 10
	var latencies []time.Duration
	failures := 0

	for i := 1; i <= cycles; i++ {
		t0 := time.Now()

		// 1. Create Env
		body := map[string]interface{}{
			"name":        "Cold Benchmark " + name,
			"gitUrl":      repo,
			"description": "Measurement harness",
		}
		bodyBytes, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", appURL+"/api/environments", bytes.NewReader(bodyBytes))
		req.Header.Set("Cookie", "token="+sessionCookie)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("  Cycle %d: Create env failed: %v\n", i, err)
			failures++
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 && resp.StatusCode != 201 {
			fmt.Printf("  Cycle %d: Failed to create env: %d %s\n", i, resp.StatusCode, string(respBody))
			failures++
			continue
		}

		var envResp EnvResponse
		json.Unmarshal(respBody, &envResp)
		envID := envResp.ID
		if envID == "" {
			fmt.Printf("  Cycle %d: Invalid env ID\n", i)
			failures++
			continue
		}

		cleanup := func() {
			req, _ := http.NewRequest("DELETE", appURL+"/api/environments/"+envID, nil)
			req.Header.Set("Cookie", "token="+sessionCookie)
			http.DefaultClient.Do(req)
		}

		// 2. Wait for RUNNING
		running := false
		for j := 0; j < 120; j++ { // up to 4 mins
			req, _ := http.NewRequest("GET", appURL+"/api/environments/"+envID, nil)
			req.Header.Set("Cookie", "token="+sessionCookie)
			r, err := http.DefaultClient.Do(req)
			if err == nil {
				var statusResp EnvResponse
				json.NewDecoder(r.Body).Decode(&statusResp)
				r.Body.Close()
				if strings.EqualFold(statusResp.Status, "running") {
					running = true
					break
				} else if strings.EqualFold(statusResp.Status, "failed") {
					break
				}
			}
			time.Sleep(2 * time.Second)
		}

		if !running {
			fmt.Printf("  Cycle %d: Failed to reach RUNNING\n", i)
			cleanup()
			failures++
			continue
		}

		// 3. Wait for preview URL
		previewReady := false
		for j := 0; j < 60; j++ { // up to 60 secs
			req, _ := http.NewRequest("GET", appURL+"/", nil)
			req.Host = envID + ".localhost"
			r, err := http.DefaultClient.Do(req)
			if err == nil {
				io.Copy(io.Discard, r.Body)
				r.Body.Close()
				if r.StatusCode != http.StatusBadGateway && r.StatusCode != http.StatusServiceUnavailable {
					previewReady = true
					break
				}
			}
			time.Sleep(1 * time.Second)
		}

		if !previewReady {
			fmt.Printf("  Cycle %d: Failed to reach preview URL\n", i)
			cleanup()
			failures++
			continue
		}

		lat := time.Since(t0)
		latencies = append(latencies, lat)
		fmt.Printf("  Cycle %d: %v\n", i, lat)

		cleanup()
		// Wait a bit before next cycle to avoid overwhelming Docker
		time.Sleep(3 * time.Second)
	}

	if len(latencies) == 0 {
		fmt.Printf("All failed!\n")
		return
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	max := latencies[len(latencies)-1]

	fmt.Printf("\n--- Cold-Start Results for %s ---\n", name)
	fmt.Printf("p50: %v\n", p50)
	fmt.Printf("p95: %v\n", p95)
	fmt.Printf("Max: %v\n", max)
	fmt.Printf("Failures: %d / %d\n", failures, cycles)
}

func runBenchmark(name string, repo string, files map[string]string) {
	// 1. Create Env
	body := map[string]interface{}{
		"name":        "Benchmark " + name,
		"gitUrl":      repo,
		"description": "Measurement harness",
	}
	bodyBytes, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", appURL+"/api/environments", bytes.NewReader(bodyBytes))
	req.Header.Set("Cookie", "token="+sessionCookie)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Create env failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		log.Fatalf("Failed to create env: %d %s", resp.StatusCode, string(respBody))
	}

	var envResp EnvResponse
	json.Unmarshal(respBody, &envResp)
	envID := envResp.ID
	if envID == "" {
		log.Fatalf("Invalid env ID: %s", string(respBody))
	}
	fmt.Printf("Created Env %s\n", envID)

	// Clean up at the end
	defer func() {
		req, _ := http.NewRequest("DELETE", appURL+"/api/environments/"+envID, nil)
		req.Header.Set("Cookie", "token="+sessionCookie)
		http.DefaultClient.Do(req)
		fmt.Printf("Deleted Env %s\n", envID)
	}()

	// 3. Wait for RUNNING
	fmt.Printf("Waiting for RUNNING...")
	for i := 0; i < 60; i++ {
		req, _ := http.NewRequest("GET", appURL+"/api/environments/"+envID, nil)
		req.Header.Set("Cookie", "token="+sessionCookie)
		r, _ := http.DefaultClient.Do(req)
		var statusResp EnvResponse
		json.NewDecoder(r.Body).Decode(&statusResp)
		r.Body.Close()
		if strings.EqualFold(statusResp.Status, "running") {
			fmt.Println(" RUNNING!")
			break
		} else if strings.EqualFold(statusResp.Status, "failed") {
			log.Fatalf("Env failed to build")
		}
		time.Sleep(2 * time.Second)
	}

	// 4. Connect WS
	wsDialURL := wsURL + "/api/ws/environments/" + envID
	header := http.Header{}
	header.Set("Cookie", "token="+sessionCookie)
	header.Set("Origin", appURL)
	c, _, err := websocket.DefaultDialer.Dial(wsDialURL, header)
	if err != nil {
		log.Fatalf("WS dial: %v", err)
	}
	defer c.Close()

	// Read WS messages in background
	wsEvents := make(chan string, 100)
	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				fmt.Printf("WS Read error: %v\n", err)
				return
			}
			// fmt.Printf("WS RECV: %s\n", string(message))
			var msg map[string]interface{}
			json.Unmarshal(message, &msg)
			if t, ok := msg["Type"].(string); ok {
				if t == "reload_ready" || t == "reload_failed" {
					wsEvents <- t
				}
			}
		}
	}()

	// 5. Measure Loop
	cycles := 15
	var latencies []time.Duration
	failures := 0

	mainFile := ""
	for f := range files {
		if strings.HasSuffix(f, ".js") || strings.HasSuffix(f, ".py") || strings.HasSuffix(f, ".go") {
			mainFile = f
			break
		}
	}

	for i := 1; i <= cycles; i++ {
		content := strings.ReplaceAll(files[mainFile], "%d", fmt.Sprintf("%d", i))
		writeBody := map[string]interface{}{
			"path":    mainFile,
			"content": content,
		}
		wb, _ := json.Marshal(writeBody)
		req, _ := http.NewRequest("POST", appURL+"/api/environments/"+envID+"/files/content", bytes.NewReader(wb))
		req.Header.Set("Cookie", "token="+sessionCookie)
		req.Header.Set("Content-Type", "application/json")

		start := time.Now()
		resp, _ := http.DefaultClient.Do(req)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		// Wait for WS
		select {
		case ev := <-wsEvents:
			if ev == "reload_ready" {
				lat := time.Since(start)
				latencies = append(latencies, lat)
				fmt.Printf("  Cycle %d: %v\n", i, lat)
			} else {
				fmt.Printf("  Cycle %d: FAILED\n", i)
				failures++
			}
		case <-time.After(15 * time.Second):
			fmt.Printf("  Cycle %d: TIMEOUT\n", i)
			failures++
		}

		// Optional: wait a sec before next
		time.Sleep(1 * time.Second)
	}

	if len(latencies) == 0 {
		fmt.Printf("All failed!\n")
		return
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	max := latencies[len(latencies)-1]

	fmt.Printf("\n--- Results for %s ---\n", name)
	fmt.Printf("p50: %v\n", p50)
	fmt.Printf("p95: %v\n", p95)
	fmt.Printf("Max: %v\n", max)
	fmt.Printf("Failures: %d / %d\n", failures, cycles)
}
