package api

import (
	"net/http"
	"net/http/httptest"
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

	// 1. Test public IP
	req, _ := http.NewRequest("GET", "/metrics", nil)
	req.RemoteAddr = "203.0.113.1:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for public IP, got %d", w.Code)
	}

	// 2. Test private IP
	req2, _ := http.NewRequest("GET", "/metrics", nil)
	req2.RemoteAddr = "10.0.0.5:1234"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	
	if w2.Code == http.StatusForbidden {
		t.Errorf("Expected allowed for private IP, got 403")
	}

	// 3. Test localhost IP
	req3, _ := http.NewRequest("GET", "/metrics", nil)
	req3.RemoteAddr = "127.0.0.1:1234"
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	
	if w3.Code == http.StatusForbidden {
		t.Errorf("Expected allowed for localhost IP, got 403")
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
