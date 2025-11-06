package user

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/models"
	"github.com/gin-gonic/gin"
)

// Handler handles user-related operations
type Handler struct{}

// NewHandler creates a new user handler
func NewHandler() *Handler {
	return &Handler{}
}

// GetProfile gets the current user's profile
func (h *Handler) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var user models.User
	query := `SELECT id, username, email, created_at FROM users WHERE id = ?`
	err := database.DB.QueryRow(query, userID).Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// AddToLibrary adds a manga to user's library
func (h *Handler) AddToLibrary(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req models.AddToLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if manga exists
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM manga WHERE id = ?)`
	err := database.DB.QueryRow(checkQuery, req.MangaID).Scan(&exists)
	if err != nil || !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Manga not found"})
		return
	}

	// Insert or update user progress
	query := `INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at)
              VALUES (?, ?, 0, ?, ?)
              ON CONFLICT(user_id, manga_id) DO UPDATE SET status = ?, updated_at = ?`

	now := time.Now()
	_, err = database.DB.Exec(query, userID, req.MangaID, req.Status, now, req.Status, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add manga to library"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Manga added to library successfully"})
}

// GetLibrary gets user's manga library
func (h *Handler) GetLibrary(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	query := `
        SELECT m.id, m.title, m.author, m.genres, m.status, m.total_chapters, m.description, m.cover_url,
               up.current_chapter, up.status, up.updated_at
        FROM user_progress up
        JOIN manga m ON up.manga_id = m.id
        WHERE up.user_id = ?
        ORDER BY up.updated_at DESC
    `

	rows, err := database.DB.Query(query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	library := models.UserLibrary{
		Reading:    []models.MangaProgress{},
		Completed:  []models.MangaProgress{},
		PlanToRead: []models.MangaProgress{},
	}

	for rows.Next() {
		var mp models.MangaProgress
		var genresJSON string

		err := rows.Scan(
			&mp.Manga.ID,
			&mp.Manga.Title,
			&mp.Manga.Author,
			&genresJSON,
			&mp.Manga.Status,
			&mp.Manga.TotalChapters,
			&mp.Manga.Description,
			&mp.Manga.CoverURL,
			&mp.CurrentChapter,
			&mp.Status,
			&mp.UpdatedAt,
		)
		if err != nil {
			continue
		}

		// Parse genres JSON
		if genresJSON != "" {
			json.Unmarshal([]byte(genresJSON), &mp.Manga.Genres)
		}

		// Categorize by status
		switch mp.Status {
		case "reading":
			library.Reading = append(library.Reading, mp)
		case "completed":
			library.Completed = append(library.Completed, mp)
		case "plan_to_read":
			library.PlanToRead = append(library.PlanToRead, mp)
		}
	}

	c.JSON(http.StatusOK, library)
}

// UpdateProgress updates user's reading progress
func (h *Handler) UpdateProgress(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req models.UpdateProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if manga exists in user's library
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM user_progress WHERE user_id = ? AND manga_id = ?)`
	err := database.DB.QueryRow(checkQuery, userID, req.MangaID).Scan(&exists)
	if err != nil || !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Manga not in library"})
		return
	}

	// Build update query
	query := `UPDATE user_progress SET current_chapter = ?, updated_at = ?`
	args := []interface{}{req.CurrentChapter, time.Now()}

	if req.Status != "" {
		query += `, status = ?`
		args = append(args, req.Status)
	}

	query += ` WHERE user_id = ? AND manga_id = ?`
	args = append(args, userID, req.MangaID)

	_, err = database.DB.Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update progress"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Progress updated successfully"})
}

// RemoveFromLibrary removes manga from user's library
func (h *Handler) RemoveFromLibrary(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	mangaID := c.Param("manga_id")
	if mangaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Manga ID is required"})
		return
	}

	query := `DELETE FROM user_progress WHERE user_id = ? AND manga_id = ?`
	result, err := database.DB.Exec(query, userID, mangaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove manga"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Manga not in library"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Manga removed from library successfully"})
}
