package manga

import (
    "database/sql"
    "encoding/json"
    "net/http"
    "strings"

    "github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
    "github.com/binhbb2204/Manga-Hub-Group13/pkg/models"
    "github.com/gin-gonic/gin"
)

// Handler handles manga-related operations
type Handler struct{}

// NewHandler creates a new manga handler
func NewHandler() *Handler {
    return &Handler{}
}

// SearchManga searches for manga based on filters
func (h *Handler) SearchManga(c *gin.Context) {
    var req models.SearchMangaRequest
    if err := c.ShouldBindQuery(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Set default values
    if req.Limit == 0 {
        req.Limit = 20
    }

    // Build query
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

    // Add pagination
    query += ` LIMIT ? OFFSET ?`
    args = append(args, req.Limit, req.Offset)

    // Execute query
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

        // Parse genres JSON
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

// GetMangaByID gets a specific manga by ID
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

    // Validate required fields
    if manga.ID == "" || manga.Title == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "ID and title are required"})
        return
    }

    // Convert genres to JSON
    genresJSON, err := json.Marshal(manga.Genres)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize genres"})
        return
    }

    // Insert into database
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

        // Parse genres JSON
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