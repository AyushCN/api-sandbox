package api

import (
	"crypto/rand"
	"fmt"
	"log/slog"
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
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		panic("crypto/rand failed")
	}
	return fmt.Sprintf("%x", b)
}

func sendViaSendGrid(apiKey, fromEmail, toEmail, subject, body string) error {
	from := mail.NewEmail("API Sandbox", fromEmail)
	to := mail.NewEmail("", toEmail)
	message := mail.NewSingleEmail(from, subject, to, body, body)
	client := sendgrid.NewSendClient(apiKey)
	response, err := client.Send(message)
	if err != nil {
		return err
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("SendGrid returned status code %d: %s", response.StatusCode, response.Body)
	}
	return nil
}

func sendViaSMTP(host, port, user, pass, fromEmail, toEmail, subject, body string) error {
	auth := smtp.PlainAuth("", user, pass, host)
	msg := []byte("To: " + toEmail + "\r\n" +
		"From: API Sandbox <" + fromEmail + ">\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n\r\n" +
		body + "\r\n")

	err := smtp.SendMail(host+":"+port, auth, fromEmail, []string{toEmail}, msg)
	if err != nil {
		return fmt.Errorf("failed to send email via SMTP: %v", err)
	}
	return nil
}

func sendEmail(toEmail, subject, body string) error {
	fromEmail := os.Getenv("SMTP_FROM")
	if fromEmail == "" {
		fromEmail = "noreply@api-sandbox.com"
	}

	sendgridKey := os.Getenv("SENDGRID_API_KEY")
	if sendgridKey != "" {
		return sendViaSendGrid(sendgridKey, fromEmail, toEmail, subject, body)
	}

	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	if smtpHost != "" && smtpPort != "" {
		smtpUser := os.Getenv("SMTP_USER")
		smtpPass := os.Getenv("SMTP_PASS")
		return sendViaSMTP(smtpHost, smtpPort, smtpUser, smtpPass, fromEmail, toEmail, subject, body)
	}

	if os.Getenv("GIN_MODE") == "release" {
		return fmt.Errorf("email configuration (SendGrid or SMTP) is missing in production")
	}

	slog.Info("MOCK EMAIL", "to", toEmail, "subject", subject, "body", body)
	return nil
}

func sendVerificationEmail(toEmail, verificationCode string) error {
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "http://localhost:3000"
	}
	verifyURL := fmt.Sprintf("%s/verify?code=%s", appURL, verificationCode)

	subject := "Verify your API Sandbox account"
	htmlContent := fmt.Sprintf(`
		<p>Verify your email by clicking <a href="%s">this link</a></p>
		<p>Or paste this code: %s</p>
	`, verifyURL, verificationCode)

	return sendEmail(toEmail, subject, htmlContent)
}

func sendPasswordResetEmail(toEmail, resetCode string) error {
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "http://localhost:3000"
	}
	resetURL := fmt.Sprintf("%s/reset-password?code=%s", appURL, resetCode)

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
		slog.Error("Failed to create user", "email", req.Email, "error", err)
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	// Create Personal Workspace Organization
	orgName := fmt.Sprintf("%s's Workspace", user.Email)
	org := models.Organization{
		Name: orgName,
	}
	if err := db.DB.Create(&org).Error; err == nil {
		// Add user as admin
		db.DB.Create(&models.OrganizationMember{
			OrganizationID: org.ID,
			UserID:         user.ID,
			Role:           models.RoleAdmin,
		})
	}

	if err := sendVerificationEmail(req.Email, verificationCode); err != nil {
		slog.Error("Failed to send verification email", "email", req.Email, "error", err)
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

	isProd := os.Getenv("GIN_MODE") == "release"
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("token", tokenString, int(72*time.Hour/time.Second), "/", "", isProd, true)

	c.JSON(http.StatusOK, gin.H{"message": "Login successful"})
}

func Logout(c *gin.Context) {
	isProd := os.Getenv("GIN_MODE") == "release"
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("token", "", -1, "/", "", isProd, true)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
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
		slog.Error("Failed to send reset email", "email", user.Email, "error", err)
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
