package user

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/manga"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/models"
	"github.com/gin-gonic/gin"
)

// Handler handles user-related operations
type Handler struct {
	bridge         *bridge.Bridge
	externalSource manga.ExternalSource
}

// NewHandler creates a new user handler
func NewHandler(br *bridge.Bridge) *Handler {
	return &Handler{
		bridge:         br,
		externalSource: manga.NewMALSource(),
	}
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

	// DEBUG: Log that we're entering this function
	log.Printf("[DEBUG] AddToLibrary called for manga ID: %s, user ID: %s", req.MangaID, userID)

	// Fetch complete manga metadata from MAL API
	ctx := context.Background()
	mangaData, err := h.externalSource.GetMangaByID(ctx, req.MangaID)
	if err != nil {
		log.Printf("[ERROR] Failed to fetch manga from MAL for ID %s: %v", req.MangaID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Manga not found in external API"})
		return
	}
	log.Printf("[DEBUG] Successfully fetched manga from MAL: %s (Total Chapters: %d)", mangaData.Title, mangaData.TotalChapters)

	// Fetch MangaDex ID from mapping API (with timeout + fallback)
	log.Printf("[DEBUG] Calling FetchMangaDexID for MAL ID: %s", req.MangaID)
	mangadexID := manga.FetchMangaDexID(req.MangaID)
	mangaData.MangaDexID = mangadexID
	log.Printf("[DEBUG] MangaDex ID result: %s", mangadexID)

	// If we got a MangaDex ID, fetch the actual chapter count from MangaDex
	if mangadexID != "" {
		chapterCount := manga.FetchMangaDexChapterCount(mangadexID)
		if chapterCount > 0 {
			log.Printf("[DEBUG] Updated total_chapters from MAL (%d) to MangaDex (%d)", mangaData.TotalChapters, chapterCount)
			mangaData.TotalChapters = chapterCount
		}
	}

	// Save or update manga in database with complete metadata (UPSERT)
	if err := h.saveMangaToDB(mangaData); err != nil {
		log.Printf("[ERROR] Failed to save manga %s to database: %v", req.MangaID, err)
		// Continue anyway - user progress is more important
	}

	// Auto-set status: new manga always starts as "plan_to_read" (current_chapter = 0)
	status := "plan_to_read"

	// Insert or update user progress
	query := `INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at)
              VALUES (?, ?, 0, ?, ?)
              ON CONFLICT(user_id, manga_id) DO UPDATE SET status = ?, updated_at = ?`

	now := time.Now()
	_, err = database.DB.Exec(query, userID, req.MangaID, status, now, status, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add manga to library"})
		return
	}

	h.bridge.NotifyLibraryUpdate(bridge.LibraryUpdateEvent{
		UserID:  userID,
		MangaID: req.MangaID,
		Action:  "added",
	})

	c.JSON(http.StatusOK, gin.H{"message": "Manga added to library successfully"})
}

// saveMangaToDB saves or updates manga in database with UPSERT
func (h *Handler) saveMangaToDB(m *models.Manga) error {
	genresJSON, err := json.Marshal(m.Genres)
	if err != nil {
		return err
	}

	query := `INSERT INTO manga (id, title, author, genres, status, total_chapters, description, cover_url, mangadex_id, media_type)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	          ON CONFLICT(id) DO UPDATE SET 
	              title = excluded.title,
	              author = excluded.author,
	              genres = excluded.genres,
	              status = excluded.status,
	              total_chapters = excluded.total_chapters,
	              description = excluded.description,
	              cover_url = excluded.cover_url,
	              mangadex_id = excluded.mangadex_id,
	              media_type = excluded.media_type`

	_, err = database.DB.Exec(
		query,
		m.ID,
		m.Title,
		m.Author,
		string(genresJSON),
		m.Status,
		m.TotalChapters,
		m.Description,
		m.CoverURL,
		m.MangaDexID,
		m.MediaType,
	)

	return err
}

// GetLibrary gets user's manga library
func (h *Handler) GetLibrary(c *gin.Context) {
	userID := c.GetString("user_id")

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	query := `
        SELECT m.id, m.title, m.author, m.genres, m.status, m.total_chapters, m.description, m.cover_url, m.mangadex_id,
               up.current_chapter, up.status, up.user_rating, up.updated_at
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
		var mangadexID sql.NullString // Handle NULL values

		var userRating sql.NullFloat64 // Handle NULL values

		err := rows.Scan(
			&mp.Manga.ID,
			&mp.Manga.Title,
			&mp.Manga.Author,
			&genresJSON,
			&mp.Manga.Status,
			&mp.Manga.TotalChapters,
			&mp.Manga.Description,
			&mp.Manga.CoverURL,
			&mangadexID, // Scan into sql.NullString
			&mp.CurrentChapter,
			&mp.Status,
			&userRating, // Scan into sql.NullFloat64
			&mp.UpdatedAt,
		)
		if err != nil {
			// Log error but continue processing other rows
			log.Printf("[ERROR] Failed to scan library row: %v", err)
			continue
		}

		// Convert sql.NullString to regular string
		if mangadexID.Valid {
			mp.Manga.MangaDexID = mangadexID.String
		} else {
			mp.Manga.MangaDexID = "" // Use empty string for NULL
		}

		// Convert user rating (use pointer for explicit null)
		if userRating.Valid {
			rating := userRating.Float64
			mp.UserRating = &rating
		} else {
			mp.UserRating = nil // Explicit null in JSON
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

	// Get manga total_chapters for auto-status calculation
	var totalChapters int
	mangaQuery := `SELECT total_chapters FROM manga WHERE id = ?`
	database.DB.QueryRow(mangaQuery, req.MangaID).Scan(&totalChapters)

	// Build update query - start with updated_at
	query := `UPDATE user_progress SET updated_at = ?`
	args := []interface{}{time.Now()}

	// Auto-calculate status based on chapter progress
	var autoStatus string
	if req.CurrentChapter != nil {
		currentChapter := *req.CurrentChapter

		if currentChapter == 0 {
			autoStatus = "plan_to_read"
		} else if totalChapters > 0 && currentChapter >= totalChapters {
			autoStatus = "completed"
		} else {
			autoStatus = "reading"
		}

		query += `, current_chapter = ?, status = ?`
		args = append(args, currentChapter, autoStatus)
	} else if req.Status != "" {
		// Allow manual status override if no chapter update
		query += `, status = ?`
		args = append(args, req.Status)
	}

	if req.UserRating != nil {
		query += `, user_rating = ?`
		args = append(args, *req.UserRating)
	}

	query += ` WHERE user_id = ? AND manga_id = ?`
	args = append(args, userID, req.MangaID)

	_, err = database.DB.Exec(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update progress"})
		return
	}

	h.bridge.NotifyProgressUpdate(bridge.ProgressUpdateEvent{
		UserID:  userID,
		MangaID: req.MangaID,
		ChapterID: func() int {
			if req.CurrentChapter != nil {
				return *req.CurrentChapter
			}
			return 0
		}(),
		Status:       autoStatus,
		LastReadDate: time.Now(),
	})

	c.JSON(http.StatusOK, gin.H{"message": "Progress updated successfully"})
}

// GetProgress gets user's progress for a specific manga
func (h *Handler) GetProgress(c *gin.Context) {
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

	query := `
		SELECT current_chapter, status, user_rating, updated_at
		FROM user_progress
		WHERE user_id = ? AND manga_id = ?
	`

	var currentChapter int
	var status string
	var userRating sql.NullFloat64
	var updatedAt time.Time

	err := database.DB.QueryRow(query, userID, mangaID).Scan(
		&currentChapter,
		&status,
		&userRating,
		&updatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Manga not in library"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	response := gin.H{
		"manga_id":        mangaID,
		"current_chapter": currentChapter,
		"status":          status,
		"updated_at":      updatedAt,
	}

	// Add user_rating if it exists
	if userRating.Valid {
		response["user_rating"] = userRating.Float64
	} else {
		response["user_rating"] = nil
	}

	c.JSON(http.StatusOK, response)
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

	h.bridge.NotifyLibraryUpdate(bridge.LibraryUpdateEvent{
		UserID:  userID,
		MangaID: mangaID,
		Action:  "removed",
	})

	c.JSON(http.StatusOK, gin.H{"message": "Manga removed from library successfully"})
}
