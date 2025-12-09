package auth

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/models"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	JWTSecret string
	// In-memory blacklist for demo (production: use Redis or DB)
	tokenBlacklist map[string]struct{}
}

func NewHandler(jwtSecret string) *Handler {
	return &Handler{
		JWTSecret:      jwtSecret,
		tokenBlacklist: make(map[string]struct{}),
	}
}

// Logout handler: Blacklist JWT token (demo only)
func (h *Handler) Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
		return
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
		return
	}
	token := parts[1]
	h.tokenBlacklist[token] = struct{}{}
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func (h *Handler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	if req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Missing required field",
			"details": "Username is required",
		})
		return
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid email format",
			"details": "Please provide a valid email address",
		})
		return
	}

	if err := validatePasswordStrength(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Password is too weak",
			"details": err.Error(),
		})
		return
	}

	userID, err := utils.GenerateID(16)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Server error",
			"details": "Failed to generate user ID",
		})
		return
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Server error",
			"details": "Failed to process password",
		})
		return
	}

	query := `INSERT INTO users (id, username, email, password_hash) VALUES (?, ?, ?, ?)`
	_, err = database.DB.Exec(query, userID, req.Username, req.Email, hashedPassword)
	if err != nil {
		// Log full DB error to help debugging unique constraint or schema issues
		log.Printf("Insert user error: %v", err)
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.username") {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "Username already exists",
				"details": "This username is already taken. Please choose another one",
			})
			return
		}
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.email") {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "Email already registered",
				"details": "This email is already associated with an account",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Registration failed",
			"details": "Could not create user account",
		})
		return
	}

	token, err := utils.GenerateJWT(userID, req.Username, h.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Server error",
			"details": "Failed to generate authentication token",
		})
		return
	}

	var createdAt time.Time
	_ = database.DB.QueryRow(`SELECT created_at FROM users WHERE id = ?`, userID).Scan(&createdAt)

	c.JSON(http.StatusCreated, models.AuthResponse{
		Token:     token,
		UserID:    userID,
		Username:  req.Username,
		Email:     req.Email,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: createdAt,
	})
}

func (h *Handler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	if req.Username == "" && req.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Missing credentials",
			"details": "Username or email is required",
		})
		return
	}

	if req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Missing credentials",
			"details": "Password is required",
		})
		return
	}

	//Query user from database
	var user models.User
	var err error
	if req.Username != "" {
		err = database.DB.QueryRow(`SELECT id, username, email, password_hash, created_at FROM users WHERE username = ?`, req.Username).
			Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt)
	} else {
		err = database.DB.QueryRow(`SELECT id, username, email, password_hash, created_at FROM users WHERE email = ?`, req.Email).
			Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Authentication failed",
				"details": "Username or email not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Server error",
			"details": "Database error occurred",
		})
		return
	}

	//Verify password
	if err := utils.CheckPassword(user.PasswordHash, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Authentication failed",
			"details": "Password is incorrect",
		})
		return
	}

	//Generate JWT token
	token, err := utils.GenerateJWT(user.ID, user.Username, h.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, models.AuthResponse{
		Token:     token,
		UserID:    user.ID,
		Username:  user.Username,
		Email:     user.Email,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: user.CreatedAt,
	})
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

func (h *Handler) ChangePassword(c *gin.Context) {
	uid, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"details": "Authentication required",
		})
		return
	}
	userID := uid.(string)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	if req.CurrentPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Missing required field",
			"details": "Current password is required",
		})
		return
	}

	if req.NewPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Missing required field",
			"details": "New password is required",
		})
		return
	}

	if err := validatePasswordStrength(req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Password is too weak",
			"details": err.Error(),
		})
		return
	}

	var hash string
	if err := database.DB.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&hash); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Account not found",
				"details": "User account does not exist",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Server error",
			"details": "Database error occurred",
		})
		return
	}

	if err := utils.CheckPassword(hash, req.CurrentPassword); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Change password failed",
			"details": "Current password is incorrect",
		})
		return
	}

	newHash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Server error",
			"details": "Failed to process new password",
		})
		return
	}

	if _, err := database.DB.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, newHash, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Server error",
			"details": "Failed to update password in database",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

func (h *Handler) UpdateEmail(c *gin.Context) {
	uid, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"details": "Authentication required",
		})
		return
	}
	userID := uid.(string)

	var req models.UpdateEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	if req.NewEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Missing required field",
			"details": "New email is required",
		})
		return
	}

	// Verify email format
	if _, err := mail.ParseAddress(req.NewEmail); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid email format",
			"details": "Please provide a valid email address",
		})
		return
	}

	// Get current user info
	var currentEmail string
	if err := database.DB.QueryRow(`SELECT email FROM users WHERE id = ?`, userID).Scan(&currentEmail); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Account not found",
				"details": "User account does not exist",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Server error",
			"details": "Database error occurred",
		})
		return
	}

	// Check if new email is same as current
	if req.NewEmail == currentEmail {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Email unchanged",
			"details": "New email is the same as current email",
		})
		return
	}

	// Check if new email already exists
	var existingEmail string
	err := database.DB.QueryRow(`SELECT email FROM users WHERE email = ? AND id != ?`, req.NewEmail, userID).Scan(&existingEmail)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Email already registered",
			"details": "This email is already associated with another account",
		})
		return
	}
	if err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Server error",
			"details": "Database error occurred",
		})
		return
	}

	// Update email
	if _, err := database.DB.Exec(`UPDATE users SET email = ? WHERE id = ?`, req.NewEmail, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Server error",
			"details": "Failed to update email in database",
		})
		return
	}

	c.JSON(http.StatusOK, models.UpdateProfileResponse{
		Message:   "Email updated successfully",
		UserID:    userID,
		Email:     req.NewEmail,
		UpdatedAt: time.Now(),
	})
}

func (h *Handler) UpdateUsername(c *gin.Context) {
	uid, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"details": "Authentication required",
		})
		return
	}
	userID := uid.(string)

	var req models.UpdateUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	if req.NewUsername == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Missing required field",
			"details": "New username is required",
		})
		return
	}

	if len(req.NewUsername) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid username",
			"details": "Username must be at least 3 characters",
		})
		return
	}

	// Get current user info
	var currentUsername string
	if err := database.DB.QueryRow(`SELECT username FROM users WHERE id = ?`, userID).Scan(&currentUsername); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Account not found",
				"details": "User account does not exist",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Server error",
			"details": "Database error occurred",
		})
		return
	}

	// Check if new username is same as current
	if req.NewUsername == currentUsername {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Username unchanged",
			"details": "New username is the same as current username",
		})
		return
	}

	// Check if new username already exists
	var existingUsername string
	err := database.DB.QueryRow(`SELECT username FROM users WHERE username = ? AND id != ?`, req.NewUsername, userID).Scan(&existingUsername)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Username already exists",
			"details": "This username is already taken. Please choose another one",
		})
		return
	}
	if err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Server error",
			"details": "Database error occurred",
		})
		return
	}

	// Update username
	if _, err := database.DB.Exec(`UPDATE users SET username = ? WHERE id = ?`, req.NewUsername, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Server error",
			"details": "Failed to update username in database",
		})
		return
	}

	c.JSON(http.StatusOK, models.UpdateProfileResponse{
		Message:   "Username updated successfully",
		UserID:    userID,
		Username:  req.NewUsername,
		UpdatedAt: time.Now(),
	})
}

func validatePasswordStrength(pw string) error {
	if len(pw) < 8 {
		return fmt.Errorf("password too weak: must be at least 8 characters with mixed case and numbers")
	}
	var lower, upper, digit bool
	for _, r := range pw {
		switch {
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= '0' && r <= '9':
			digit = true
		}
	}
	if !(lower && upper && digit) {
		return fmt.Errorf("password too weak: must be at least 8 characters with mixed case and numbers")
	}
	return nil
}
