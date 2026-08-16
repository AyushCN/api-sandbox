package api

import (
	"bytes"
	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestDBForFiles(t *testing.T) {
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

func createTestEnvironmentForFiles(t *testing.T) (*models.Environment, *models.User) {
	user := models.User{Email: "userfiles@example.com"}
	db.DB.Create(&user)

	project := models.Project{Name: "Project Files", CreatedByUserID: user.ID}
	db.DB.Create(&project)
	db.DB.Create(&models.ProjectCollaborator{
		ProjectID: project.ID,
		UserID:    user.ID,
		Role:      models.ProjectRoleOwner,
	})

	env := models.Environment{
		Name:         "Files Env",
		ProjectID:    project.ID,
		UserID:       user.ID,
		GitURL:       "https://github.com/test/repo",
		GithubBranch: "main",
	}
	db.DB.Create(&env)
	return &env, &user
}

func setupTestRouterForFiles() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	r.Use(AuthMiddleware())
	r.GET("/api/environments/:id/files", GetWorkspaceFiles)
	r.GET("/api/environments/:id/files/content", GetWorkspaceFileContent)
	r.POST("/api/environments/:id/files/content", UpdateWorkspaceFileContent)
	r.POST("/api/environments/:id/files/create", CreateWorkspaceFileOrFolder)
	r.POST("/api/environments/:id/files/delete", DeleteWorkspaceFileOrFolder)

	return r
}

func TestPathTraversal(t *testing.T) {
	// Set up environment with user and workspace
	setupTestDBForFiles(t)
	env, user := createTestEnvironmentForFiles(t)

	router := setupTestRouterForFiles()

	tests := []struct {
		name       string
		method     string
		url        string
		body       []byte
		wantStatus int
	}{
		// GetWorkspaceFileContent (GET)
		{"GET relative parent", "GET", "/api/environments/" + env.ID + "/files/content?path=../secret.txt", nil, http.StatusBadRequest},
		{"GET absolute path", "GET", "/api/environments/" + env.ID + "/files/content?path=/etc/passwd", nil, http.StatusBadRequest},
		{"GET complex traversal", "GET", "/api/environments/" + env.ID + "/files/content?path=src/../../etc/passwd", nil, http.StatusBadRequest},
		{"GET empty path", "GET", "/api/environments/" + env.ID + "/files/content?path=", nil, http.StatusBadRequest},
		{"GET dot path", "GET", "/api/environments/" + env.ID + "/files/content?path=.", nil, http.StatusBadRequest},
		{"GET slash path", "GET", "/api/environments/" + env.ID + "/files/content?path=/", nil, http.StatusBadRequest},
		// UpdateWorkspaceFileContent (POST)
		{"POST relative parent", "POST", "/api/environments/" + env.ID + "/files/content", []byte(`{"path":"../secret.txt","content":"hack"}`), http.StatusBadRequest},
		{"POST absolute path", "POST", "/api/environments/" + env.ID + "/files/content", []byte(`{"path":"/etc/passwd","content":"hack"}`), http.StatusBadRequest},
		{"POST complex traversal", "POST", "/api/environments/" + env.ID + "/files/content", []byte(`{"path":"src/../../etc/passwd","content":"hack"}`), http.StatusBadRequest},
		{"POST empty path", "POST", "/api/environments/" + env.ID + "/files/content", []byte(`{"path":"","content":"hack"}`), http.StatusBadRequest},
		{"POST dot path", "POST", "/api/environments/" + env.ID + "/files/content", []byte(`{"path":".","content":"hack"}`), http.StatusBadRequest},
		// CreateWorkspaceFileOrFolder (POST)
		{"CREATE relative parent", "POST", "/api/environments/" + env.ID + "/files/create", []byte(`{"path":"../new.txt"}`), http.StatusBadRequest},
		{"CREATE absolute path", "POST", "/api/environments/" + env.ID + "/files/create", []byte(`{"path":"/etc/passwd"}`), http.StatusBadRequest},
		{"CREATE empty path", "POST", "/api/environments/" + env.ID + "/files/create", []byte(`{"path":""}`), http.StatusBadRequest},
		// DeleteWorkspaceFileOrFolder (POST)
		{"DELETE relative parent", "POST", "/api/environments/" + env.ID + "/files/delete", []byte(`{"path":"../old.txt"}`), http.StatusBadRequest},
		{"DELETE absolute path", "POST", "/api/environments/" + env.ID + "/files/delete", []byte(`{"path":"/etc/passwd"}`), http.StatusBadRequest},
		{"DELETE empty path", "POST", "/api/environments/" + env.ID + "/files/delete", []byte(`{"path":""}`), http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != nil {
				req, _ = http.NewRequest(tt.method, tt.url, bytes.NewBuffer(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, _ = http.NewRequest(tt.method, tt.url, nil)
			}
			req.AddCookie(&http.Cookie{Name: "token", Value: generateTestToken(user.ID)})

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d for %s, got %d", tt.wantStatus, tt.name, w.Code)
			}
		})
	}

	// Test one happy path (we don't need actual file to exist for bad request check, it returns 404/500 if it passes traversal check but file doesn't exist)
	t.Run("GET valid path", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/environments/"+env.ID+"/files/content?path=src/main.go", nil)
		req.AddCookie(&http.Cookie{Name: "token", Value: generateTestToken(user.ID)})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		// Should not be 400 Bad Request. Will probably be 404 File Not Found, which means traversal check passed.
		if w.Code == http.StatusBadRequest {
			t.Errorf("Expected status != 400 for valid path, got %d", w.Code)
		}
	})
}
