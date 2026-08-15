package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupIntegrationDB(t *testing.T) {
	var err error
	db.DB, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
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
}

func setupIntegrationRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	// Use actual JWT middleware if testing protected routes
	auth := r.Group("/api", AuthMiddleware())
	
	// Auth routes
	r.POST("/api/auth/register", Register)
	r.POST("/api/auth/login", Login)
	r.POST("/api/auth/verify", VerifyEmail)
	r.POST("/api/auth/reset-password", ForgotPassword)
	
	auth.POST("/environments", CreateEnvironment)

	return r
}


func TestQuotas(t *testing.T) {
	setupIntegrationDB(t)
	r := setupIntegrationRouter()
	os.Setenv("JWT_SECRET", "testsecret")

	user := models.User{Email: "quota@example.com", Username: "quota", IsEmailVerified: true, GithubToken: "fake_token"}
	db.DB.Create(&user)
	token := generateTestToken(user.ID)

	// Mock Enqueue for CreateEnvironment so it doesn't fail
	// Wait, we don't need to mock Enqueue, it will just fail to enqueue (redis offline) and return 500, but quota check happens BEFORE enqueue.
	
	// Max environments is 5. Create 5 environments manually in DB.
	for i := 0; i < 5; i++ {
		db.DB.Create(&models.Environment{
			UserID: user.ID,
			Name:   "Env",
			Status: models.StatusRunning,
		})
	}

	req, _ := http.NewRequest("POST", "/api/environments", bytes.NewBuffer([]byte(`{"name":"Env 6","gitUrl":"https://github.com/foo/bar"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden { // 403
		t.Errorf("Expected 403 Forbidden for concurrent limit, got %d", w.Code)
	}

	// Reset DB, test builds per hour
	db.DB.Exec("DELETE FROM environments")
	
	// Max builds per hour is 10.
	for i := 0; i < 10; i++ {
		db.DB.Create(&models.Environment{
			UserID:    user.ID,
			Name:      "Env",
			Status:    models.StatusStopped,
			CreatedAt: time.Now(),
		})
	}

	req2, _ := http.NewRequest("POST", "/api/environments", bytes.NewBuffer([]byte(`{"name":"Env 11","gitUrl":"https://github.com/foo/bar"}`)))
	req2.Header.Set("Content-Type", "application/json")
	req2.AddCookie(&http.Cookie{Name: "token", Value: token})
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests { // 429
		t.Errorf("Expected 429 Too Many Requests for builds per hour limit, got %d", w2.Code)
	}
}
