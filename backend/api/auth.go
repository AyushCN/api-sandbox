package api

import (
	"net/http"
	"os"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func getJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "api-sandbox-dev-secret-key-32-chars-ok"
	}
	return secret
}

func Logout(c *gin.Context) {
	isProd := os.Getenv("GIN_MODE") == "release"
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("token", "", -1, "/", "", isProd, true)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// DevLogin generates a valid session for the local admin user
func DevLogin(c *gin.Context) {
	var user models.User
	err := db.DB.Where("email = ? OR username = ?", "admin@api-sandbox.local", "admin").First(&user).Error
	if err != nil {
		user = models.User{
			Email:          "admin@api-sandbox.local",
			Username:       "admin",
			GithubID:       "1",
			GithubUsername: "admin",
		}
		if err := db.DB.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create admin user: " + err.Error()})
			return
		}
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": user.ID,
		"exp":    time.Now().Add(time.Hour * 72).Unix(),
	})

	tokenString, err := jwtToken.SignedString([]byte(getJWTSecret()))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate dev token"})
		return
	}

	isProd := os.Getenv("GIN_MODE") == "release"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", tokenString, int(72*time.Hour/time.Second), "/", "", isProd, true)

	c.JSON(http.StatusOK, gin.H{
		"message": "Dev login successful",
		"token":   tokenString,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}
