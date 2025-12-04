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
		       JWTSecret: jwtSecret,
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}
	if err := validatePasswordStrength(req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := utils.GenerateID(16)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate user ID"})
		return
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	query := `INSERT INTO users (id, username, email, password_hash) VALUES (?, ?, ?, ?)`
	_, err = database.DB.Exec(query, userID, req.Username, req.Email, hashedPassword)
	if err != nil {
		// Log full DB error to help debugging unique constraint or schema issues
		log.Printf("Insert user error: %v", err)
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.username") {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
			return
		}
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.email") {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	token, err := utils.GenerateJWT(userID, req.Username, h.JWTSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Username == "" && req.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username or email is required"})
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
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Account not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	//Verify password
	if err := utils.CheckPassword(user.PasswordHash, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := uid.(string)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validatePasswordStrength(req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var hash string
	if err := database.DB.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&hash); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Account not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	if err := utils.CheckPassword(hash, req.CurrentPassword); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}
	newHash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}
	if _, err := database.DB.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, newHash, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
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
