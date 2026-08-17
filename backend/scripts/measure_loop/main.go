package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
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
			repo: "https://github.com/expressjs/express",
			files: map[string]string{
				"package.json": `{"scripts":{"dev":"node index.js"},"dependencies":{"express":"^4.18.2"}}`,
				"index.js": `const express = require('express');
const app = express();
app.get('/', (req, res) => res.send('Hello V%d'));
app.listen(3000, '0.0.0.0', () => console.log('Ready'));`,
			},
		},
		{
			name: "Python FastAPI",
			repo: "https://github.com/pallets/flask", // just for python detection
			files: map[string]string{
				"requirements.txt": `fastapi
uvicorn[standard]`,
				"main.py": `from fastapi import FastAPI
app = FastAPI()
@app.get("/")
def read_root():
    return {"Hello": "V%d"}`,
			},
		},
		{
			name: "Go Basic",
			repo: "https://github.com/gin-gonic/gin", // just for go detection
			files: map[string]string{
				"go.mod": `module example.com/m
go 1.22`,
				"main.go": `package main
import (
	"fmt"
	"net/http"
)
func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello V%%d")
	})
	http.ListenAndServe("0.0.0.0:8080", nil)
}`,
			},
		},
	}

	for _, app := range apps {
		fmt.Printf("\n=== Benchmarking %s ===\n", app.name)
		runBenchmark(app.name, app.repo, app.files)
	}
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

	// 2. Write Files
	for path, content := range files {
		// Just replace %d with 0
		content = strings.ReplaceAll(content, "%d", "0")
		writeBody := map[string]interface{}{
			"path":    path,
			"content": content,
		}
		wb, _ := json.Marshal(writeBody)
		req, _ = http.NewRequest("POST", appURL+"/api/environments/"+envID+"/files/content", bytes.NewReader(wb))
		req.Header.Set("Cookie", "token="+sessionCookie)
		req.Header.Set("Content-Type", "application/json")
		resp, _ = http.DefaultClient.Do(req)
		resp.Body.Close()
	}

	// 2.5 Restart Env so it picks up the new files and port
	fmt.Printf("Restarting env to pick up changes...\n")
	req, _ = http.NewRequest("POST", appURL+"/api/environments/"+envID+"/restart", nil)
	req.Header.Set("Cookie", "token="+sessionCookie)
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()

	// Force port 3000 in DB for node and 8000 for Python/Go
	port := 3000
	if strings.Contains(name, "Python") || strings.Contains(name, "Go") {
		port = 8000
	}
	exec.Command("docker", "exec", "api-sandbox-postgres-1", "psql", "-U", "postgres", "-d", "api_sandbox", "-c", fmt.Sprintf("UPDATE environments SET port = %d WHERE id = '%s'", port, envID)).Run()

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
				return
			}
			var msg map[string]interface{}
			json.Unmarshal(message, &msg)
			if t, ok := msg["type"].(string); ok {
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
