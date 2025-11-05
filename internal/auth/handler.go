package auth

import(
	"database/sql"
	"net/http"
	"strings"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
    "github.com/binhbb2204/Manga-Hub-Group13/pkg/models"
    "github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
    "github.com/gin-gonic/gin"
)

type Handler struct{
	JWTSecret string
}

func NewHandler(jwtSecret string) *Handler{
	return &Handler{
		JWTSecret: jwtSecret,
	}
}

func (h *Handler) Register(c *gin.Context){
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil{
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := utils.GenerateID(16)
	if err != nil{
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate user ID"})
		return
	}

	hashedPassword, err := utils.HashPassword(req.Password)
    if err != nil{
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
        return
    }

	query := `INSERT INTO users (id, username, password_hash) VALUES (?, ?, ?)`
    _, err = database.DB.Exec(query, userID, req.Username, hashedPassword)
    if err != nil {
        if strings.Contains(err.Error(), "UNIQUE constraint failed"){
            c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
        return
    }

    token, err := utils.GenerateJWT(userID, req.Username, h.JWTSecret)
    if err != nil{
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
        return
    }

    c.JSON(http.StatusCreated, models.AuthResponse{
        Token:    token,
        UserID:   userID,
        Username: req.Username,
    })
}

func (h *Handler) Login(c *gin.Context) {
    var req models.LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    //Query user from database
    var user models.User
    query := `SELECT id, username, password_hash FROM users WHERE username = ?`
    err := database.DB.QueryRow(query, req.Username).Scan(&user.ID, &user.Username, &user.PasswordHash)
    if err != nil {
        if err == sql.ErrNoRows {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
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
        Token:    token,
        UserID:   user.ID,
        Username: user.Username,
    })
}