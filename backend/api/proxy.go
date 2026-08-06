package api

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
)

// ProxyMiddleware inspects host headers or path prefixes to route requests to running sandbox containers
func ProxyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.Request.Host
		
		// Ignore standard domains and IPs
		if host == "" || strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") || strings.HasPrefix(host, "api.") {
			c.Next()
			return
		}

		parts := strings.Split(host, ".")
		if len(parts) < 2 {
			c.Next()
			return
		}

		envID := parts[0]
		var env models.Environment
		if err := db.DB.First(&env, "id = ?", envID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Sandbox environment not found"})
			c.Abort()
			return
		}

		if env.Status != models.StatusRunning || env.Port == nil || *env.Port == 0 {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":  "Sandbox container is not running",
				"status": env.Status,
			})
			c.Abort()
			return
		}

		targetURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", *env.Port))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid proxy target URL"})
			c.Abort()
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(targetURL)
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(fmt.Sprintf(`{"error": "Failed to connect to sandbox container: %v"}`, err)))
		}

		proxy.ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

// DirectProxyHandler allows proxying via path: GET /api/proxy/:id/*path
func DirectProxyHandler(c *gin.Context) {
	envID := c.Param("id")
	var env models.Environment
	if err := db.DB.First(&env, "id = ?", envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sandbox environment not found"})
		return
	}

	if env.Status != models.StatusRunning || env.Port == nil || *env.Port == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":  "Sandbox container is not running",
			"status": env.Status,
		})
		return
	}

	targetURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", *env.Port))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid proxy target URL"})
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	// Strip prefix /api/proxy/:id
	c.Request.URL.Path = strings.TrimPrefix(c.Request.URL.Path, fmt.Sprintf("/api/proxy/%s", envID))
	if c.Request.URL.Path == "" {
		c.Request.URL.Path = "/"
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(fmt.Sprintf(`{"error": "Failed to connect to sandbox container: %v"}`, err)))
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}
