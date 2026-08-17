package api

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func getJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		panic("JWT_SECRET environment variable is required")
	}
	return secret
}

func Logout(c *gin.Context) {
	isProd := os.Getenv("GIN_MODE") == "release"
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("token", "", -1, "/", "", isProd, true)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}
