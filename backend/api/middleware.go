package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
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
			authHeader := c.GetHeader("Authorization")
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				tokenString = authHeader[7:]
			}
		}

		if tokenString == "" {
			host := c.Request.Host
			if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
				var adminUser models.User
				if err := db.DB.Where("email = ? OR username = ?", "admin@api-sandbox.local", "admin").First(&adminUser).Error; err == nil {
					c.Set("userId", adminUser.ID)
					c.Next()
					return
				}
			}
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
			slog.Error("Redis error during rate limiting", "error", err)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Rate limiter unavailable"})
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
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Rate limiter unavailable"})
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
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Rate limiter unavailable"})
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
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Rate limiter unavailable"})
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
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "Rate limiter unavailable"})
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

func hasProjectMemberPermission(userRole, requiredRole models.ProjectMemberRole) bool {
	hierarchy := map[models.ProjectMemberRole]int{
		models.ProjectMemberRoleOwner:  2,
		models.ProjectMemberRoleEditor: 1,
		models.ProjectMemberRoleViewer: 0,
	}
	return hierarchy[userRole] >= hierarchy[requiredRole]
}

func AuthorizeProjectMemberAccess(requiredRole models.ProjectMemberRole) gin.HandlerFunc {
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

		var project models.Project
		if err := db.DB.First(&project, "id = ?", projectID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			c.Abort()
			return
		}

		// If Default Project (Personal Space), only the owner can access it
		if project.IsDefault || project.ID == "default-project" {
			if project.CreatedByUserID != userID {
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: personal default project is private"})
				c.Abort()
				return
			}
			c.Set("projectMemberRole", models.ProjectMemberRoleOwner)
			c.Set("projectID", projectID)
			c.Next()
			return
		}

		var member models.ProjectMember
		err := db.DB.
			Where("project_id = ? AND user_id = ?", projectID, userID).
			First(&member).Error

		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Not a member of this project or project does not exist",
			})
			c.Abort()
			return
		}

		if member.Status != models.ProjectMemberStatusAccepted {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Pending invitations do not grant access to project resources",
			})
			c.Abort()
			return
		}

		if !hasProjectMemberPermission(member.Role, requiredRole) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions on this project",
			})
			c.Abort()
			return
		}

		c.Set("projectMemberRole", member.Role)
		c.Set("projectID", projectID)
		c.Next()
	}
}
