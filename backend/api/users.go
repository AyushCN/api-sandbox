package api

import (
	"net/http"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
)

// SearchUsers allows searching for users by username or email.
// It returns a limited number of results (e.g., 5) to prevent data scraping.
func SearchUsers(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusOK, []gin.H{})
		return
	}

	// We don't want to search if query is too short to avoid massive results
	if len(query) < 2 {
		c.JSON(http.StatusOK, []gin.H{})
		return
	}

	searchPattern := "%" + query + "%"
	var users []models.User

	// Find top 5 users matching either email or username
	if err := db.DB.Where("email LIKE ? OR username LIKE ?", searchPattern, searchPattern).Limit(5).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search users"})
		return
	}

	// Format response to only include safe fields
	var results []gin.H
	for _, u := range users {
		results = append(results, gin.H{
			"id":       u.ID,
			"username": u.Username,
			"email":    u.Email,
		})
	}

	c.JSON(http.StatusOK, results)
}
