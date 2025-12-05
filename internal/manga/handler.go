package manga

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/models"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	externalSource ExternalSource
}

// Helper function to parse query parameters as integers
func parseIntQuery(c *gin.Context, param string, defaultValue int) int {
	valueStr := c.Query(param)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil || value < 0 {
		return defaultValue
	}
	return value
}

// Helper function to calculate pagination metadata
func calculatePagination(page, limit, total int) models.PaginationMeta {
	if limit > 20 {
		limit = 20
	}
	if limit <= 0 {
		limit = 20
	}

	totalPages := (total + limit - 1) / limit // Ceiling division
	if totalPages == 0 {
		totalPages = 1
	}

	hasNext := page < totalPages
	hasPrev := page > 1

	return models.PaginationMeta{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    hasNext,
		HasPrev:    hasPrev,
	}
}

// This is for get manga based on ranking
type Author struct {
	Node struct {
		Name      string `json:"name"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	} `json:"node"`
}

type RankingManga struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	MainPicture *struct {
		Medium string `json:"medium"`
		Large  string `json:"large"`
	} `json:"main_picture,omitempty"`
	Status      string   `json:"status"`
	NumChapters int      `json:"num_chapters"`
	Authors     []Author `json:"authors"`
	Synopsis    string   `json:"description,omitempty"`
	CoverURL    string   `json:"cover_url,omitempty"`
}

type RankingList struct {
	Data []struct {
		Node RankingManga `json:"node"`
	} `json:"data"`
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

	// Set default limit to 20, max 20
	limit := req.Limit
	if limit <= 0 || limit > 20 {
		limit = 20
	}

	// Default page to 1 if not provided
	page := req.Page
	if page <= 0 {
		page = 1
	}

	// Get total count first
	countQuery := `SELECT COUNT(*) FROM manga WHERE 1=1`
	countArgs := []interface{}{}

	if req.Title != "" {
		countQuery += ` AND title LIKE ?`
		countArgs = append(countArgs, "%"+req.Title+"%")
	}

	if req.Author != "" {
		countQuery += ` AND author LIKE ?`
		countArgs = append(countArgs, "%"+req.Author+"%")
	}

	if req.Status != "" {
		countQuery += ` AND status = ?`
		countArgs = append(countArgs, req.Status)
	}

	if req.Genre != "" {
		countQuery += ` AND genres LIKE ?`
		countArgs = append(countArgs, "%"+req.Genre+"%")
	}

	var total int
	err := database.DB.QueryRow(countQuery, countArgs...).Scan(&total)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Calculate offset for the requested page
	offset := (page - 1) * limit

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
	args = append(args, limit, offset)

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

	pagination := calculatePagination(req.Page, limit, total)

	response := models.PaginatedBooksResponse{
		Mangas:     mangas,
		Pagination: pagination,
	}

	c.JSON(http.StatusOK, response)
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

	// Parse pagination parameters
	page := parseIntQuery(c, "page", 1)          // Default to page 1
	requestLimit := parseIntQuery(c, "limit", 0) // 0 means no limit (fetch all)
	pageSize := 20                               // Fixed page size

	// Determine max results to fetch
	maxResults := 500 // Default: fetch all available (MAL max)
	if requestLimit > 0 {
		maxResults = requestLimit // User specified limit
		if maxResults > 500 {
			maxResults = 500 // Cap at MAL maximum
		}
	}

	ctx := context.Background()

	// Fetch all results to get accurate total and return requested page
	allMangas := []models.Manga{}
	fetchPage := 1
	for {
		fetchOffset := (fetchPage - 1) * 100
		fetchLimit := 100

		mangas, err := h.externalSource.Search(ctx, query, fetchLimit, fetchOffset)
		if err != nil {
			if strings.Contains(err.Error(), "400") && fetchPage == 1 {
				pagination := calculatePagination(page, pageSize, 0)
				response := models.PaginatedBooksResponse{
					Mangas:     []models.Manga{},
					Pagination: pagination,
				}
				c.JSON(http.StatusOK, response)
				return
			}
			break
		}

		if len(mangas) == 0 {
			break
		}

		allMangas = append(allMangas, mangas...)

		if len(mangas) < fetchLimit {
			break
		}

		fetchPage++
		if fetchPage > 50 {
			break
		}
	}

	// Now get the requested page from all results
	total := len(allMangas)
	offset := (page - 1) * pageSize
	end := offset + pageSize
	if end > total {
		end = total
	}

	var pageMangas []models.Manga
	if offset < total {
		pageMangas = allMangas[offset:end]
	} else {
		pageMangas = []models.Manga{}
	}

	pagination := calculatePagination(page, pageSize, total)

	response := models.PaginatedBooksResponse{
		Mangas:     pageMangas,
		Pagination: pagination,
	}

	c.JSON(http.StatusOK, response)
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

	isNumeric := true
	for _, ch := range mangaID {
		if ch < '0' || ch > '9' {
			isNumeric = false
			break
		}
	}
	if !isNumeric {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Manga ID must be numeric"})
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

func (h *Handler) fetchRanking(clientID, rankingType string, limit int) ([]RankingManga, error) {
	apiURL := "https://api.myanimelist.net/v2/manga/ranking"
	params := url.Values{}
	params.Add("ranking_type", rankingType)
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("fields", "id,title,main_picture,authors{name,first_name,last_name},status,num_chapters,synopsis")

	req, err := http.NewRequest("GET", apiURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MAL-Client-ID", clientID)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MAL API returned status: %v", res.Status)
	}

	var result RankingList
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}

	var mangas []RankingManga
	for _, item := range result.Data {
		manga := item.Node
		// Populate cover_url from main_picture
		if manga.MainPicture != nil {
			if manga.MainPicture.Large != "" {
				manga.CoverURL = manga.MainPicture.Large
			} else {
				manga.CoverURL = manga.MainPicture.Medium
			}
		}
		mangas = append(mangas, manga)
	}
	return mangas, nil
}

func (h *Handler) GetFeaturedManga(c *gin.Context) {
	clientID := os.Getenv("MAL_CLIENT_ID")
	if clientID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "MAL API not configured"})
		return
	}

	sections := []struct {
		Label string `json:"label"`
		Key   string `json:"key"`
	}{
		{"Top Ranked Manga", "all"},
		{"Most Popular Manga", "bypopularity"},
		{"Most Favorited Manga", "favorite"},
	}

	type SectionResult struct {
		Label  string         `json:"label"`
		Mangas []RankingManga `json:"mangas"`
	}

	var wg sync.WaitGroup
	results := make([]SectionResult, len(sections))
	errors := make([]error, len(sections))

	for i, s := range sections {
		wg.Add(1)
		go func(i int, s struct {
			Label string `json:"label"`
			Key   string `json:"key"`
		}) {
			defer wg.Done()
			mangas, err := h.fetchRanking(clientID, s.Key, 10)
			if err != nil {
				errors[i] = err
				return
			}
			results[i] = SectionResult{
				Label:  s.Label,
				Mangas: mangas,
			}
		}(i, s)
	}

	wg.Wait()

	allFailed := true
	for _, err := range errors {
		if err == nil {
			allFailed = false
			break
		}
	}

	if allFailed {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch manga rankings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sections": results,
	})
}

func (h *Handler) GetRanking(c *gin.Context) {
	clientID := os.Getenv("MAL_CLIENT_ID")
	if clientID == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "MAL API not configured"})
		return
	}

	rankingType := c.DefaultQuery("type", "all")

	// Parse pagination parameters
	page := parseIntQuery(c, "page", 1)          // Default to page 1
	requestLimit := parseIntQuery(c, "limit", 0) // 0 means no limit (fetch all)
	pageSize := 20                               // Fixed page size

	// Determine max results to fetch
	maxResults := 500 // Default: fetch all available (MAL max)
	if requestLimit > 0 {
		maxResults = requestLimit // User specified limit
		if maxResults > 500 {
			maxResults = 500 // Cap at MAL maximum
		}
	}

	// Fetch all results for the ranking type
	allMangas, err := h.fetchRanking(clientID, rankingType, maxResults)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get the requested page
	offset := (page - 1) * pageSize
	var mangas []RankingManga
	if offset < len(allMangas) {
		end := offset + pageSize
		if end > len(allMangas) {
			end = len(allMangas)
		}
		mangas = allMangas[offset:end]
	}

	total := len(allMangas)

	// Convert RankingManga to regular Manga for pagination response
	regularMangas := make([]models.Manga, len(mangas))
	for i, rm := range mangas {
		regularMangas[i] = models.Manga{
			ID:            fmt.Sprintf("%d", rm.ID),
			Title:         rm.Title,
			Status:        rm.Status,
			TotalChapters: rm.NumChapters,
			Description:   rm.Synopsis,

			CoverURL: rm.CoverURL,
		}
		if len(rm.Authors) > 0 {
			author := strings.TrimSpace(rm.Authors[0].Node.FirstName + " " + rm.Authors[0].Node.LastName)
			if author != "" {
				regularMangas[i].Author = author
			}
		}
	}

	pagination := calculatePagination(page, pageSize, total)

	response := models.PaginatedBooksResponse{
		Mangas:     regularMangas,
		Pagination: pagination,
	}

	c.JSON(http.StatusOK, response)
}

// GetChapters fetches chapters for a manga from MangaDex
// GET /api/manga/chapters/:mangadexId?language=en&limit=100
func (h *Handler) GetChapters(c *gin.Context) {
	mangaDexID := c.Param("mangadexId")
	if mangaDexID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MangaDex ID is required"})
		return
	}

	language := c.DefaultQuery("language", "en")
	limit := parseIntQuery(c, "limit", 100)
	if limit > 500 {
		limit = 500
	}

	mangadex := NewMangaDexSource()
	ctx := context.Background()

	chapters, err := mangadex.GetChapters(ctx, mangaDexID, language, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"mangadex_id": mangaDexID,
		"language":    language,
		"total":       len(chapters),
		"chapters":    chapters,
	})
}

// GetChapterPages fetches page URLs for a specific chapter from MangaDex
// GET /api/manga/chapter/:chapterId/pages
func (h *Handler) GetChapterPages(c *gin.Context) {
	chapterID := c.Param("chapterId")
	if chapterID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chapter ID is required"})
		return
	}

	mangadex := NewMangaDexSource()
	ctx := context.Background()

	pages, err := mangadex.GetChapterPages(ctx, chapterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Build full page URLs
	pageURLs := make([]string, len(pages.Data))
	for i := range pages.Data {
		pageURLs[i] = pages.GetPageURL(i)
	}

	c.JSON(http.StatusOK, gin.H{
		"chapter_id":  chapterID,
		"total_pages": len(pageURLs),
		"base_url":    pages.BaseURL,
		"hash":        pages.Hash,
		"page_urls":   pageURLs,
	})
}
