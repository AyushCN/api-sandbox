package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := c.Cookie("token")
		if err != nil || tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: missing token"})
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(getJWTSecret()), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			c.Set("userId", claims["userId"])
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		c.Next()
	}
}

func RateLimitRegister() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		key := fmt.Sprintf("register:%s", clientIP)

		ctx := context.Background()
		count, err := db.RedisClient.Incr(ctx, key).Result()
		if err != nil {
			// If Redis fails, log it but don't block registration
			slog.Error("Redis error during rate limiting", "error", err)
			c.Next()
			return
		}

		if count == 1 {
			// Set expiry on first request
			db.RedisClient.Expire(ctx, key, 1*time.Hour)
		}

		if count > 5 {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many registration attempts. Try again in 1 hour."})
			c.Abort()
			return
		}

		c.Next()
	}
}

func RateLimitLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		key := fmt.Sprintf("login:%s", clientIP)

		ctx := context.Background()
		count, err := db.RedisClient.Incr(ctx, key).Result()
		if err != nil {
			slog.Error("Redis error during rate limiting", "error", err)
			c.Next()
			return
		}

		if count == 1 {
			db.RedisClient.Expire(ctx, key, 1*time.Hour)
		}

		if count > 20 {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many login attempts. Try again in 1 hour."})
			c.Abort()
			return
		}

		c.Next()
	}
}

func RateLimitAPI() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("userId")
		if !exists {
			userID = "anonymous"
		}
		clientIP := c.ClientIP()

		key := fmt.Sprintf("api_limit:%v:%s", userID, clientIP)

		ctx := context.Background()
		count, err := db.RedisClient.Incr(ctx, key).Result()
		if err != nil {
			slog.Error("Redis error during API rate limiting", "error", err)
			c.Next()
			return
		}

		if count == 1 {
			db.RedisClient.Expire(ctx, key, 1*time.Minute)
		}

		if count > 200 {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "API rate limit exceeded. Try again in 1 minute."})
			c.Abort()
			return
		}

		c.Next()
	}
}

func RateLimitPasswordReset() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		key := fmt.Sprintf("password_reset_limit:%s", clientIP)

		ctx := context.Background()
		count, err := db.RedisClient.Incr(ctx, key).Result()
		if err != nil {
			slog.Error("Redis error during password reset rate limiting", "error", err)
			c.Next()
			return
		}

		if count == 1 {
			db.RedisClient.Expire(ctx, key, 1*time.Hour)
		}

		if count > 5 {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many password reset attempts. Try again later."})
			c.Abort()
			return
		}

		c.Next()
	}
}

func RateLimitVerifyEmail() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		key := fmt.Sprintf("verify_email_limit:%s", clientIP)

		ctx := context.Background()
		count, err := db.RedisClient.Incr(ctx, key).Result()
		if err != nil {
			slog.Error("Redis error during verify email rate limiting", "error", err)
			c.Next()
			return
		}

		if count == 1 {
			db.RedisClient.Expire(ctx, key, 10*time.Minute)
		}

		if count > 10 {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many email verification attempts. Try again in 10 minutes."})
			c.Abort()
			return
		}
		c.Next()
	}
}

func hasProjectPermission(userRole, requiredRole models.ProjectRole) bool {
	hierarchy := map[models.ProjectRole]int{
		models.ProjectRoleOwner:        3,
		models.ProjectRoleAdmin:        2,
		models.ProjectRoleCollaborator: 1,
		models.ProjectRoleViewer:       0,
	}
	return hierarchy[userRole] >= hierarchy[requiredRole]
}

func AuthorizeProjectAccess(requiredRole models.ProjectRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		if projectID == "" {
			projectID = c.Param("id")
		}

		userIDVal, exists := c.Get("userId")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
		userID := userIDVal.(string)

		var collab models.ProjectCollaborator
		err := db.DB.
			Where("project_id = ? AND user_id = ?", projectID, userID).
			First(&collab).Error

		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Not a collaborator on this project or project does not exist",
			})
			c.Abort()
			return
		}

		if collab.AcceptedAt == nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Invite pending acceptance",
			})
			c.Abort()
			return
		}

		if !hasProjectPermission(collab.Role, requiredRole) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions on this project",
			})
			c.Abort()
			return
		}

		c.Set("projectRole", collab.Role)
		c.Set("projectID", projectID)
		c.Next()
	}
}
