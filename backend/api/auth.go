package api

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/api-sandbox/backend/db"
	"github.com/api-sandbox/backend/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"golang.org/x/crypto/bcrypt"
)



func getJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		panic("JWT_SECRET environment variable is required")
	}
	return secret
}

type AuthRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=12,containsany=!@#$%^&*"`
}

func ValidatePassword(password string) error {
	if len(password) < 12 {
		return fmt.Errorf("password must be at least 12 characters")
	}
	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, ch := range password {
		switch {
		case ch >= 'A' && ch <= 'Z':
			hasUpper = true
		case ch >= 'a' && ch <= 'z':
			hasLower = true
		case ch >= '0' && ch <= '9':
			hasDigit = true
		case strings.ContainsRune("!@#$%^&*()", ch):
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return fmt.Errorf("password must contain uppercase, lowercase, digit, and special character")
	}
	return nil
}

func generateVerificationCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n.Int64())
}

func sendVerificationEmail(toEmail, verificationCode string) error {
	apiKey := os.Getenv("SENDGRID_API_KEY")
	verifyURL := fmt.Sprintf("http://localhost:3000/verify?code=%s", verificationCode)

	if apiKey == "" {
		fmt.Printf("\n========== MOCK EMAIL ==========\nTo: %s\nSubject: Verify your API Sandbox account\nLink: %s\n================================\n\n", toEmail, verifyURL)
		return nil
	}

	from := mail.NewEmail("API Sandbox", "noreply@api-sandbox.com")
	to := mail.NewEmail("User", toEmail)
	subject := "Verify your API Sandbox account"
	htmlContent := fmt.Sprintf(`
		<p>Verify your email by clicking <a href="%s">this link</a></p>
		<p>Or paste this code: %s</p>
	`, verifyURL, verificationCode)

	message := mail.NewSingleEmail(from, subject, to, "Verify your account", htmlContent)
	client := sendgrid.NewSendClient(apiKey)
	response, err := client.Send(message)

	if err != nil || response.StatusCode >= 400 {
		return fmt.Errorf("failed to send email: %v", err)
	}
	return nil
}

func sendPasswordResetEmail(toEmail, resetCode string) error {
	apiKey := os.Getenv("SENDGRID_API_KEY")
	resetURL := fmt.Sprintf("http://localhost:3000/reset-password?code=%s", resetCode)

	if apiKey == "" {
		fmt.Printf("\n========== MOCK PASSWORD RESET EMAIL ==========\nTo: %s\nSubject: Reset your API Sandbox password\nLink: %s\n===============================================\n\n", toEmail, resetURL)
		return nil
	}

	from := mail.NewEmail("API Sandbox", "noreply@api-sandbox.com")
	to := mail.NewEmail("User", toEmail)
	subject := "Reset your API Sandbox password"
	htmlContent := fmt.Sprintf(`
		<p>Reset your password by clicking <a href="%s">this link</a></p>
	`, resetURL)

	message := mail.NewSingleEmail(from, subject, to, "Reset your password", htmlContent)
	client := sendgrid.NewSendClient(apiKey)
	response, err := client.Send(message)

	if err != nil || response.StatusCode >= 400 {
		return fmt.Errorf("failed to send email: %v", err)
	}
	return nil
}

func Register(c *gin.Context) {
	var req AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ValidatePassword(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	verificationCode := generateVerificationCode()
	verificationExp := time.Now().Add(24 * time.Hour)

	user := models.User{
		Email:            req.Email,
		Password:         string(hashedPassword),
		IsEmailVerified:  false,
		VerificationCode: verificationCode,
		VerificationExp:  &verificationExp,
	}

	if err := db.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	if err := sendVerificationEmail(req.Email, verificationCode); err != nil {
		fmt.Printf("Failed to send verification email: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Registration successful. Check your email to verify."})
}

func Login(c *gin.Context) {
	var req AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := db.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	if !user.IsEmailVerified {
		c.JSON(http.StatusForbidden, gin.H{"error": "Email not verified. Check your inbox."})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": user.ID,
		"exp":    time.Now().Add(time.Hour * 72).Unix(),
	})

	tokenString, err := token.SignedString([]byte(getJWTSecret()))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

func VerifyEmail(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification code is required"})
		return
	}

	var user models.User
	if err := db.DB.Where("verification_code = ?", code).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid verification code"})
		return
	}

	if user.VerificationExp != nil && user.VerificationExp.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification code expired"})
		return
	}

	db.DB.Model(&user).Updates(map[string]interface{}{
		"is_email_verified": true,
		"verification_code": "",
		"verification_exp":  nil,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Email verified! You can now login."})
}

func ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email address"})
		return
	}

	var user models.User
	if err := db.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Don't leak whether the email exists or not
		c.JSON(http.StatusOK, gin.H{"message": "If that email is registered, we've sent a password reset link."})
		return
	}

	resetCode := generateVerificationCode() + generateVerificationCode()
	resetExp := time.Now().Add(1 * time.Hour)

	db.DB.Model(&user).Updates(map[string]interface{}{
		"reset_password_code": resetCode,
		"reset_password_exp":  resetExp,
	})

	if err := sendPasswordResetEmail(user.Email, resetCode); err != nil {
		fmt.Printf("Failed to send reset email: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "If that email is registered, we've sent a password reset link."})
}

func ResetPassword(c *gin.Context) {
	var req struct {
		Code        string `json:"code" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required,min=12,containsany=!@#$%^&*"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := ValidatePassword(req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := db.DB.Where("reset_password_code = ?", req.Code).First(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset code"})
		return
	}

	if user.ResetPasswordExp != nil && user.ResetPasswordExp.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset code"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	db.DB.Model(&user).Updates(map[string]interface{}{
		"password":            string(hashedPassword),
		"reset_password_code": "",
		"reset_password_exp":  nil,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Password successfully reset. You can now login."})
}
