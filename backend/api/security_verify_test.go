package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/api-sandbox/backend/db"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func TestMetricsProtection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	// Trust all proxies for testing X-Forwarded-For, or we can just set RemoteAddr
	r.GET("/metrics", PrometheusMetrics)

	os.Setenv("METRICS_TOKEN", "test-metrics-token-123")
	defer os.Unsetenv("METRICS_TOKEN")

	// 1. Test without token (should be 403)
	req0, _ := http.NewRequest("GET", "/metrics", nil)
	w0 := httptest.NewRecorder()
	r.ServeHTTP(w0, req0)
	if w0.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden without token, got %d", w0.Code)
	}

	// 2. Test public IP with token (should be 403)
	req, _ := http.NewRequest("GET", "/metrics", nil)
	req.Header.Set("X-Metrics-Token", "test-metrics-token-123")
	req.RemoteAddr = "203.0.113.1:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for public IP, got %d", w.Code)
	}

	// 3. Test private IP with token
	req2, _ := http.NewRequest("GET", "/metrics", nil)
	req2.Header.Set("X-Metrics-Token", "test-metrics-token-123")
	req2.RemoteAddr = "10.0.0.5:1234"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for private IP with token, got %d", w2.Code)
	}

	// 4. Test localhost IP with token
	req3, _ := http.NewRequest("GET", "/metrics", nil)
	req3.Header.Set("X-Metrics-Token", "test-metrics-token-123")
	req3.RemoteAddr = "127.0.0.1:1234"
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for localhost IP with token, got %d", w3.Code)
	}
}

func TestRateLimitFailClosed(t *testing.T) {
	// Setup bad redis client to simulate redis being down
	opt, _ := redis.ParseURL("redis://localhost:9999") // Bad port
	db.RedisClient = redis.NewClient(opt)

	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.Use(RateLimitRegister())
	r.POST("/register", func(c *gin.Context) {
		c.Status(200)
	})

	req, _ := http.NewRequest("POST", "/register", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected 503 Service Unavailable, got %d", w.Code)
	}
}
