package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/api-sandbox/backend/db"
	"context"
	"time"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token format"})
			return
		}

		tokenString := parts[1]

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
			fmt.Printf("Redis error during rate limiting: %v\n", err)
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
			fmt.Printf("Redis error during rate limiting: %v\n", err)
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
			fmt.Printf("Redis error during API rate limiting: %v\n", err)
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
			fmt.Printf("Redis error during password reset rate limiting: %v\n", err)
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
