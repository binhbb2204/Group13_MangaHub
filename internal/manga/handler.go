package manga

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/models"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	externalSource ExternalSource
}

func NewHandler() *Handler {
	source, err := NewExternalSourceFromEnv()
	if err != nil {
		return &Handler{}
	}
	return &Handler{
		externalSource: source,
	}
}

// SearchManga searches for manga based on filters
func (h *Handler) SearchManga(c *gin.Context) {
	var req models.SearchMangaRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Limit == 0 {
		req.Limit = 20
	}

	query := `SELECT id, title, author, genres, status, total_chapters, description, cover_url FROM manga WHERE 1=1`
	args := []interface{}{}

	if req.Title != "" {
		query += ` AND title LIKE ?`
		args = append(args, "%"+req.Title+"%")
	}

	if req.Author != "" {
		query += ` AND author LIKE ?`
		args = append(args, "%"+req.Author+"%")
	}

	if req.Status != "" {
		query += ` AND status = ?`
		args = append(args, req.Status)
	}

	if req.Genre != "" {
		query += ` AND genres LIKE ?`
		args = append(args, "%"+req.Genre+"%")
	}

	query += ` LIMIT ? OFFSET ?`
	args = append(args, req.Limit, req.Offset)

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var mangas []models.Manga
	for rows.Next() {
		var manga models.Manga
		var genresJSON string

		err := rows.Scan(
			&manga.ID,
			&manga.Title,
			&manga.Author,
			&genresJSON,
			&manga.Status,
			&manga.TotalChapters,
			&manga.Description,
			&manga.CoverURL,
		)
		if err != nil {
			continue
		}

		if genresJSON != "" {
			json.Unmarshal([]byte(genresJSON), &manga.Genres)
		}

		mangas = append(mangas, manga)
	}

	c.JSON(http.StatusOK, gin.H{
		"mangas": mangas,
		"count":  len(mangas),
	})
}

// SearchExternal searches manga from external API (MyAnimeList)
func (h *Handler) SearchExternal(c *gin.Context) {
	if h.externalSource == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "External manga source not configured"})
		return
	}

	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	if len(strings.TrimSpace(query)) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query must be at least 3 characters"})
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}

	offsetStr := c.DefaultQuery("offset", "0")
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	ctx := context.Background()
	mangas, err := h.externalSource.Search(ctx, query, limit, offset)
	if err != nil {
		if strings.Contains(err.Error(), "400") {
			c.JSON(http.StatusOK, gin.H{
				"mangas": []models.Manga{},
				"count":  0,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"mangas": mangas,
		"count":  len(mangas),
	})
}

// GetMangaInfo gets manga info from external API by ID
func (h *Handler) GetMangaInfo(c *gin.Context) {
	if h.externalSource == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "External manga source not configured"})
		return
	}

	mangaID := c.Param("id")
	if mangaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Manga ID is required"})
		return
	}

	ctx := context.Background()
	manga, err := h.externalSource.GetMangaByID(ctx, mangaID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, manga)
}

func (h *Handler) GetMangaByID(c *gin.Context) {
	mangaID := c.Param("id")

	var manga models.Manga
	var genresJSON string

	query := `SELECT id, title, author, genres, status, total_chapters, description, cover_url FROM manga WHERE id = ?`
	err := database.DB.QueryRow(query, mangaID).Scan(
		&manga.ID,
		&manga.Title,
		&manga.Author,
		&genresJSON,
		&manga.Status,
		&manga.TotalChapters,
		&manga.Description,
		&manga.CoverURL,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Manga not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Parse genres JSON
	if genresJSON != "" {
		json.Unmarshal([]byte(genresJSON), &manga.Genres)
	}

	c.JSON(http.StatusOK, manga)
}

// CreateManga creates a new manga entry (for testing purposes)
func (h *Handler) CreateManga(c *gin.Context) {
	var manga models.Manga
	if err := c.ShouldBindJSON(&manga); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if manga.ID == "" || manga.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID and title are required"})
		return
	}

	genresJSON, err := json.Marshal(manga.Genres)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize genres"})
		return
	}

	query := `INSERT INTO manga (id, title, author, genres, status, total_chapters, description, cover_url) 
              VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = database.DB.Exec(
		query,
		manga.ID,
		manga.Title,
		manga.Author,
		string(genresJSON),
		manga.Status,
		manga.TotalChapters,
		manga.Description,
		manga.CoverURL,
	)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			c.JSON(http.StatusConflict, gin.H{"error": "Manga with this ID already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create manga"})
		return
	}

	c.JSON(http.StatusCreated, manga)
}

// GetAllManga retrieves all manga (for testing purposes)
func (h *Handler) GetAllManga(c *gin.Context) {
	query := `SELECT id, title, author, genres, status, total_chapters, description, cover_url FROM manga`
	rows, err := database.DB.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var mangas []models.Manga
	for rows.Next() {
		var manga models.Manga
		var genresJSON string

		err := rows.Scan(
			&manga.ID,
			&manga.Title,
			&manga.Author,
			&genresJSON,
			&manga.Status,
			&manga.TotalChapters,
			&manga.Description,
			&manga.CoverURL,
		)
		if err != nil {
			continue
		}

		if genresJSON != "" {
			json.Unmarshal([]byte(genresJSON), &manga.Genres)
		}

		mangas = append(mangas, manga)
	}

	c.JSON(http.StatusOK, gin.H{
		"mangas": mangas,
		"count":  len(mangas),
	})
}
