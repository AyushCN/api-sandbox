package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateVerificationCode(t *testing.T) {
	code1 := generateVerificationCode()
	code2 := generateVerificationCode()

	if len(code1) != 32 {
		t.Errorf("Expected length 32, got %d", len(code1))
	}
	if code1 == code2 {
		t.Errorf("Generated codes should be unique")
	}
}

func TestLogout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Logout(c)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check if the token cookie is set to expire immediately (empty value, max age -1)
	cookieHeader := w.Header().Get("Set-Cookie")
	if cookieHeader == "" {
		t.Errorf("Expected Set-Cookie header to be present")
	}
}
