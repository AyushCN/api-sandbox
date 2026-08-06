package api

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"net/smtp"
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

func getAppURL(c *gin.Context) string {
	appURL := os.Getenv("APP_URL")
	if appURL != "" {
		return appURL
	}
	
	scheme := "https"
	if os.Getenv("GIN_MODE") != "release" {
		scheme = "http"
	}
	
	host := c.Request.Host
	if host == "" {
		host = "localhost:3000"
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}



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

func sendEmail(toEmail, subject, htmlContent string) error {
	// 1. Try SendGrid
	sendgridKey := os.Getenv("SENDGRID_API_KEY")
	if sendgridKey != "" {
		from := mail.NewEmail("API Sandbox", "noreply@api-sandbox.com")
		to := mail.NewEmail("User", toEmail)
		message := mail.NewSingleEmail(from, subject, to, subject, htmlContent)
		client := sendgrid.NewSendClient(sendgridKey)
		response, err := client.Send(message)
		if err != nil || response.StatusCode >= 400 {
			return fmt.Errorf("sendgrid failed: %v", err)
		}
		return nil
	}

	// 2. Try SMTP
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	fromEmail := os.Getenv("SMTP_FROM")
	if fromEmail == "" {
		fromEmail = "noreply@api-sandbox.com"
	}

	if smtpHost != "" && smtpPort != "" {
		auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
		msg := []byte("To: " + toEmail + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\r\n\r\n" +
			htmlContent + "\r\n")
		
		err := smtp.SendMail(smtpHost+":"+smtpPort, auth, fromEmail, []string{toEmail}, msg)
		if err != nil {
			return fmt.Errorf("smtp failed: %v", err)
		}
		return nil
	}

	// 3. Fallback to Mock
	if os.Getenv("GIN_MODE") != "release" {
		fmt.Printf("\n========== MOCK EMAIL ==========\nTo: %s\nSubject: %s\nContent: %s\n================================\n\n", toEmail, subject, htmlContent)
		return nil
	}

	return fmt.Errorf("SMTP not configured. Set SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS")
}

func sendVerificationEmail(toEmail, verifyURL, verificationCode string) error {
	subject := "Verify your API Sandbox account"
	htmlContent := fmt.Sprintf(`
		<p>Verify your email by clicking <a href="%s">this link</a></p>
		<p>Or paste this code: %s</p>
	`, verifyURL, verificationCode)

	return sendEmail(toEmail, subject, htmlContent)
}

func sendPasswordResetEmail(toEmail, resetURL string) error {
	subject := "Reset your API Sandbox password"
	htmlContent := fmt.Sprintf(`
		<p>Reset your password by clicking <a href="%s">this link</a></p>
	`, resetURL)

	return sendEmail(toEmail, subject, htmlContent)
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

	verifyURL := fmt.Sprintf("%s/verify?code=%s", getAppURL(c), verificationCode)
	if err := sendVerificationEmail(req.Email, verifyURL, verificationCode); err != nil {
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

	resetURL := fmt.Sprintf("%s/reset-password?code=%s", getAppURL(c), resetCode)
	if err := sendPasswordResetEmail(user.Email, resetURL); err != nil {
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
