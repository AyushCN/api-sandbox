package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func generateStateString() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func GithubLogin(c *gin.Context) {
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	if clientID == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "GITHUB_CLIENT_ID not configured"})
		return
	}

	state := generateStateString()

	// Store state in a secure http-only cookie for CSRF protection
	isProd := os.Getenv("GIN_MODE") == "release"
	c.SetSameSite(http.SameSiteLaxMode) // Lax is usually better for OAuth redirects
	c.SetCookie("oauth_state", state, int(10*time.Minute/time.Second), "/", "", isProd, true)

	// Scope: user:email for identity, repo for clone/commit/push
	redirectURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&scope=user:email,repo&state=%s",
		clientID, state,
	)
	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

func GithubCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	cookieState, err := c.Cookie("oauth_state")
	if err != nil || cookieState != state {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid OAuth state"})
		return
	}

	clientID := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "GitHub credentials not configured"})
		return
	}

	// 1. Exchange code for token
	tokenURL := "https://github.com/login/oauth/access_token"
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)

	req, _ := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token"})
		return
	}
	defer resp.Body.Close()

	var tokenRes struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenRes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode token response"})
		return
	}

	if tokenRes.Error != "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": tokenRes.Error})
		return
	}

	accessToken := tokenRes.AccessToken

	// 2. Fetch User Profile
	userReq, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	userReq.Header.Set("Authorization", "Bearer "+accessToken)
	userReq.Header.Set("Accept", "application/json")

	userResp, err := client.Do(userReq)
	if err != nil || userResp.StatusCode != 200 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch GitHub user"})
		return
	}
	defer userResp.Body.Close()

	var ghUser struct {
		ID      int    `json:"id"`
		Login   string `json:"login"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Avatar  string `json:"avatar_url"`
		HtmlUrl string `json:"html_url"`
	}
	if err := json.NewDecoder(userResp.Body).Decode(&ghUser); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode GitHub user"})
		return
	}

	// Fetch primary email if null (users with private emails)
	if ghUser.Email == "" {
		emailReq, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
		emailReq.Header.Set("Authorization", "Bearer "+accessToken)
		emailReq.Header.Set("Accept", "application/json")
		emailResp, err := client.Do(emailReq)
		if err == nil && emailResp.StatusCode == 200 {
			defer emailResp.Body.Close()
			var emails []struct {
				Email   string `json:"email"`
				Primary bool   `json:"primary"`
			}
			json.NewDecoder(emailResp.Body).Decode(&emails)
			for _, e := range emails {
				if e.Primary {
					ghUser.Email = e.Email
					break
				}
			}
			if ghUser.Email == "" && len(emails) > 0 {
				ghUser.Email = emails[0].Email
			}
		}
	}

	if ghUser.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No email associated with GitHub account"})
		return
	}

	ghIDStr := fmt.Sprintf("%d", ghUser.ID)
	encryptedToken, err := Encrypt(accessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt token"})
		return
	}

	var user models.User
	err = db.DB.Where("github_id = ? OR email = ?", ghIDStr, ghUser.Email).First(&user).Error
	if err != nil {
		// Create new user
		username := ghUser.Login
		if username == "" {
			username = strings.Split(ghUser.Email, "@")[0]
		}

		user = models.User{
			Email:          ghUser.Email,
			Username:       username,
			GithubID:       ghIDStr,
			GithubUsername: ghUser.Login,
			GithubToken:    encryptedToken,
			Github:         ghUser.HtmlUrl,
			AvatarURL:      ghUser.Avatar,
		}

		if err := db.DB.Create(&user).Error; err != nil {
			slog.Error("Failed to create OAuth user", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		// Create Personal Workspace Organization
		orgName := fmt.Sprintf("%s's Workspace", user.Username)
		org := models.Organization{
			Name: orgName,
		}
		if err := db.DB.Create(&org).Error; err == nil {
			db.DB.Create(&models.OrganizationMember{
				OrganizationID: org.ID,
				UserID:         user.ID,
				Role:           models.RoleAdmin,
			})

			// Create Default Workspace Project
			defaultProject := models.Project{
				Name:                "Default Workspace",
				OwnerOrganizationID: org.ID,
				CreatedByUserID:     user.ID,
			}
			if err := db.DB.Create(&defaultProject).Error; err == nil {
				db.DB.Create(&models.ProjectCollaborator{
					ProjectID: defaultProject.ID,
					UserID:    user.ID,
					Role:      models.ProjectRoleOwner,
				})
			}
		}
	} else {
		// Update existing user with latest GitHub info
		db.DB.Model(&user).Updates(map[string]interface{}{
			"github_id":       ghIDStr,
			"github_username": ghUser.Login,
			"github_token":    encryptedToken,
			"github":          ghUser.HtmlUrl,
			"avatar_url":      ghUser.Avatar,
		})
	}

	// 3. Issue JWT
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": user.ID,
		"exp":    time.Now().Add(time.Hour * 72).Unix(),
	})

	tokenString, err := jwtToken.SignedString([]byte(getJWTSecret()))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	isProd := os.Getenv("GIN_MODE") == "release"
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("token", tokenString, int(72*time.Hour/time.Second), "/", "", isProd, true)

	// Redirect to frontend dashboard
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "http://localhost:3000"
	}
	c.Redirect(http.StatusTemporaryRedirect, appURL+"/dashboard")
}
