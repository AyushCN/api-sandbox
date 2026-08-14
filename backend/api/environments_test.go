package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func generateTestToken(userId string) string {
	os.Setenv("JWT_SECRET", "test-secret-key-12345")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": userId,
		"exp":    time.Now().Add(time.Hour * 24).Unix(),
	})
	tokenString, _ := token.SignedString([]byte("test-secret-key-12345"))
	return tokenString
}



func setupEnvironmentTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	r.Use(AuthMiddleware())

	r.POST("/api/environments", CreateEnvironment)
	r.GET("/api/environments/:id", GetEnvironment)
	r.POST("/api/environments/:id/restart", RestartEnvironment)
	r.DELETE("/api/environments/:id", DeleteEnvironment)
	r.GET("/api/ws/environments/:id", ServeWS)

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
	req, _ := http.NewRequest(http.MethodGet, "/api/environments/"+env.ID, nil)
	req.AddCookie(&http.Cookie{
		Name:  "token",
		Value: generateTestToken(user2.ID),
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("User2 should get 404 (Not Found / Unauthorized) for User1's environment. Got %d, body: %s", w.Code, w.Body.String())
	}

	// Test RestartEnvironment
	req, _ = http.NewRequest(http.MethodPost, "/api/environments/"+env.ID+"/restart", nil)
	req.AddCookie(&http.Cookie{
		Name:  "token",
		Value: generateTestToken(user2.ID),
	})
	wRestart := httptest.NewRecorder()
	r.ServeHTTP(wRestart, req)
	if wRestart.Code != http.StatusNotFound {
		t.Errorf("User2 should get 404 for User1's environment restart. Got %d", wRestart.Code)
	}

	// Test DeleteEnvironment
	req, _ = http.NewRequest(http.MethodDelete, "/api/environments/"+env.ID, nil)
	req.AddCookie(&http.Cookie{
		Name:  "token",
		Value: generateTestToken(user2.ID),
	})
	wDelete := httptest.NewRecorder()
	r.ServeHTTP(wDelete, req)
	if wDelete.Code != http.StatusNotFound {
		t.Errorf("User2 should get 404 for User1's environment delete. Got %d", wDelete.Code)
	}

	// Test WebSocket access
	req, _ = http.NewRequest(http.MethodGet, "/api/ws/environments/"+env.ID, nil)
	req.AddCookie(&http.Cookie{
		Name:  "token",
		Value: generateTestToken(user2.ID),
	})
	// We must mock the websocket upgrade, but a simple 404/403/400 check works since Upgrader fails or Auth fails first.
	wWs := httptest.NewRecorder()
	r.ServeHTTP(wWs, req)
	if wWs.Code != http.StatusNotFound && wWs.Code != http.StatusForbidden {
		t.Errorf("User2 should get 403 or 404 for User1's environment WS. Got %d", wWs.Code)
	}

	// Test CreateEnvironment authorization (User 2 trying to create in User 1's project)
	createReq := CreateEnvironmentRequest{
		Name:         "Hacked Env",
		GitURL:       "https://github.com/test/repo",
		ProjectID:    projectA.ID,
	}
	payload, _ := json.Marshal(createReq)
	req, _ = http.NewRequest(http.MethodPost, "/api/environments", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{
		Name:  "token",
		Value: generateTestToken(user2.ID),
	})
	wCreate := httptest.NewRecorder()
	r.ServeHTTP(wCreate, req)

	// Depending on implementation, it might be 403 or 404. Let's check for non-2xx
	if wCreate.Code == http.StatusCreated {
		t.Errorf("User2 should not be able to create environment in User1's project.")
	}
	
	// Cleanup test DB
	os.Remove("test.db")
}
