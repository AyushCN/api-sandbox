package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupEnvironmentTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	// Mock AuthMiddleware
	r.Use(func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		if userID != "" {
			c.Set("userId", userID)
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	})

	r.POST("/api/environments", CreateEnvironment)
	r.GET("/api/environments/:id", GetEnvironment)
	r.POST("/api/environments/:id/restart", RestartEnvironment)
	r.DELETE("/api/environments/:id", DeleteEnvironment)

	return r
}

func TestEnvironmentAuthz(t *testing.T) {
	// Initialize in-memory SQLite for testing
	var err error
	db.DB, err = gorm.Open(sqlite.Open("test.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	err = db.DB.AutoMigrate(
		&models.User{},
		&models.Project{},
		&models.ProjectCollaborator{},
		&models.Environment{},
		&models.Log{},
		&models.Metric{},
	)
	if err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
	r := setupEnvironmentTestRouter()

	// 1. Setup Users
	user1 := models.User{Email: "user1@example.com"}
	user2 := models.User{Email: "user2@example.com"}
	db.DB.Create(&user1)
	db.DB.Create(&user2)

	// 2. Setup Project A owned by User 1
	projectA := models.Project{Name: "Project A", CreatedByUserID: user1.ID}
	db.DB.Create(&projectA)
	db.DB.Create(&models.ProjectCollaborator{
		ProjectID: projectA.ID,
		UserID:    user1.ID,
		Role:      models.ProjectRoleOwner,
	})

	// 3. Create Environment in Project A
	env := models.Environment{
		Name:         "Project A Env",
		ProjectID:    projectA.ID,
		UserID:       user1.ID,
		GitURL:       "https://github.com/test/repo",
		GithubBranch: "main",
	}
	db.DB.Create(&env)

	// -- Tests for User 2 (Unauthorized) --

	// Test GetEnvironment
	req, _ := http.NewRequest("GET", "/api/environments/"+env.ID, nil)
	req.Header.Set("X-User-ID", user2.ID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("User2 should get 404 (Not Found / Unauthorized) for User1's environment. Got %d, body: %s", w.Code, w.Body.String())
	}

	// Test RestartEnvironment
	reqRestart, _ := http.NewRequest("POST", "/api/environments/"+env.ID+"/restart", nil)
	reqRestart.Header.Set("X-User-ID", user2.ID)
	wRestart := httptest.NewRecorder()
	r.ServeHTTP(wRestart, reqRestart)
	if wRestart.Code != http.StatusNotFound {
		t.Errorf("User2 should get 404 for User1's environment restart. Got %d", wRestart.Code)
	}

	// Test DeleteEnvironment
	reqDelete, _ := http.NewRequest("DELETE", "/api/environments/"+env.ID, nil)
	reqDelete.Header.Set("X-User-ID", user2.ID)
	wDelete := httptest.NewRecorder()
	r.ServeHTTP(wDelete, reqDelete)
	if wDelete.Code != http.StatusNotFound {
		t.Errorf("User2 should get 404 for User1's environment delete. Got %d", wDelete.Code)
	}

	// Test CreateEnvironment authorization (User 2 trying to create in User 1's project)
	createReq := CreateEnvironmentRequest{
		Name:         "Hacked Env",
		GitURL:       "https://github.com/test/repo",
		ProjectID:    projectA.ID,
	}
	body, _ := json.Marshal(createReq)
	reqCreate, _ := http.NewRequest("POST", "/api/environments", bytes.NewBuffer(body))
	reqCreate.Header.Set("X-User-ID", user2.ID)
	reqCreate.Header.Set("Content-Type", "application/json")
	wCreate := httptest.NewRecorder()
	r.ServeHTTP(wCreate, reqCreate)

	// Depending on implementation, it might be 403 or 404. Let's check for non-2xx
	if wCreate.Code == http.StatusCreated {
		t.Errorf("User2 should not be able to create environment in User1's project.")
	}
	
	// Cleanup test DB
	os.Remove("test.db")
}
