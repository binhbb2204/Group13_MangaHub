package manga

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/models"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	externalSource ExternalSource
	broker         *NotificationBroker
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

// RankingManga represents MAL ranking API response format
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
	Synopsis    string   `json:"synopsis,omitempty"`
	CoverURL    string   `json:"cover_url,omitempty"`
	Genres      []string `json:"genres,omitempty"`
}

// UnmarshalJSON custom unmarshaler to convert MAL genres format to string array
func (r *RankingManga) UnmarshalJSON(data []byte) error {
	// Temporary struct matching MAL API format exactly
	aux := &struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		MainPicture *struct {
			Medium string `json:"medium"`
			Large  string `json:"large"`
		} `json:"main_picture,omitempty"`
		Status      string   `json:"status"`
		NumChapters int      `json:"num_chapters"`
		Authors     []Author `json:"authors"`
		Synopsis    string   `json:"synopsis,omitempty"`
		GenresRaw   []struct {
			Name string `json:"name"`
		} `json:"genres"`
	}{}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Copy fields
	r.ID = aux.ID
	r.Title = aux.Title
	r.MainPicture = aux.MainPicture
	r.Status = aux.Status
	r.NumChapters = aux.NumChapters
	r.Authors = aux.Authors
	r.Synopsis = aux.Synopsis

	// Convert genres to string array
	if len(aux.GenresRaw) > 0 {
		r.Genres = make([]string, len(aux.GenresRaw))
		for i, g := range aux.GenresRaw {
			r.Genres[i] = g.Name
		}
	}

	return nil
}

type RankingList struct {
	Data []struct {
		Node RankingManga `json:"node"`
	} `json:"data"`
}

func NewHandler() *Handler {
	source, err := NewExternalSourceFromEnv()
	broker := NewBroker()
	if err != nil {
		return &Handler{
			broker: broker,
		}
	}
	return &Handler{
		externalSource: source,
		broker:         broker,
	}
}

// GetBroker returns the notification broker
func (h *Handler) GetBroker() *NotificationBroker {
	return h.broker
}

// SearchManga searches for manga based on filters
func (h *Handler) SearchManga(c *gin.Context) {
	var req models.SearchMangaRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Support 'q' parameter as alias for title search
	if q := c.Query("q"); q != "" && req.Title == "" {
		req.Title = q
	}

	// Set default limit to 20, cap at 100 to avoid huge payloads
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	// Parse genres (comma-separated or repeated)
	var genreFilters []string
	if req.Genre != "" {
		parts := strings.Split(req.Genre, ",")
		for _, p := range parts {
			if g := strings.TrimSpace(p); g != "" {
				genreFilters = append(genreFilters, g)
			}
		}
	}
	for _, g := range req.Genres {
		if gg := strings.TrimSpace(g); gg != "" {
			genreFilters = append(genreFilters, gg)
		}
	}

	// Canonicalize type (only media types apply as filters)
	canonicalType := func(t string) string {
		t = strings.TrimSpace(strings.ToLower(t))
		switch t {
		case "", "all":
			return ""
		case "manga":
			return "manga"
		case "manhwa":
			return "manhwa"
		case "manhua":
			return "manhua"
		case "novel", "novels", "lightnovel", "light_novel", "lightnovels":
			return "novel"
		case "one_shot", "oneshot", "oneshots":
			return "one_shot"
		case "doujin", "doujinshi":
			return "doujinshi"
		default:
			return ""
		}
	}
	normalizedType := canonicalType(req.Type)

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

	for _, g := range genreFilters {
		countQuery += ` AND genres LIKE ?`
		countArgs = append(countArgs, "%"+g+"%")
	}

	if normalizedType != "" {
		countQuery += ` AND media_type = ?`
		countArgs = append(countArgs, normalizedType)
	}

	var total int
	err := database.DB.QueryRow(countQuery, countArgs...).Scan(&total)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Calculate offset for the requested page
	offset := (page - 1) * limit

	query := `SELECT id, title, author, genres, status, total_chapters, description, cover_url, media_type FROM manga WHERE 1=1`
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

	for _, g := range genreFilters {
		query += ` AND genres LIKE ?`
		args = append(args, "%"+g+"%")
	}

	if normalizedType != "" {
		query += ` AND media_type = ?`
		args = append(args, normalizedType)
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
			&manga.MediaType,
		)
		if err != nil {
			continue
		}

		if genresJSON != "" {
			json.Unmarshal([]byte(genresJSON), &manga.Genres)
		}

		mangas = append(mangas, manga)
	}

	pagination := calculatePagination(page, limit, total)

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

	query := strings.TrimSpace(c.Query("q"))
	typeParam := strings.TrimSpace(strings.ToLower(c.Query("type")))
	genreParam := strings.TrimSpace(c.Query("genre"))

	// Parse comma-separated genres
	var genreFilters []string
	if genreParam != "" {
		parts := strings.Split(genreParam, ",")
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				genreFilters = append(genreFilters, trimmed)
			}
		}
	}

	// Canonicalize media type strings for both ranking fallback and media-type filtering
	mapToCanonicalMediaType := func(mt string) string {
		mt = strings.TrimSpace(strings.ToLower(mt))
		mt = strings.ReplaceAll(mt, "-", "_")
		switch mt {
		case "manga":
			return "manga"
		case "novel", "novels", "lightnovel", "light_novel", "lightnovels":
			return "novel"
		case "one_shot", "oneshot", "oneshots":
			return "one_shot"
		case "doujin", "doujinshi":
			return "doujinshi"
		case "manhwa":
			return "manhwa"
		case "manhua":
			return "manhua"
		default:
			return ""
		}
	}

	// If no query provided but type is provided, fall back to ranking-by-type search
	if query == "" {
		clientID := os.Getenv("MAL_CLIENT_ID")
		if clientID == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "MAL API not configured"})
			return
		}

		// If no type and no query and no genre, return error
		if typeParam == "" && genreParam == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "At least one of 'q', 'type', or 'genre' is required"})
			return
		}

		// Pagination params already parsed below
		page := parseIntQuery(c, "page", 1)
		requestLimit := parseIntQuery(c, "limit", 0)
		pageSize := 20

		normalizeRankingType := func(mt string) string {
			mt = strings.TrimSpace(strings.ToLower(mt))
			mt = strings.ReplaceAll(mt, "-", "_")
			switch mt {
			case "", "all":
				return "all"
			case "manga":
				return "manga"
			case "oneshot", "one_shot", "oneshots":
				return "oneshots"
			case "doujin", "doujinshi":
				return "doujin"
			case "manhwa":
				return "manhwa"
			case "manhua":
				return "manhua"
			case "lightnovel", "light_novel", "lightnovels", "novel", "novels":
				return "lightnovels"
			case "bypopularity", "popularity", "popular":
				return "bypopularity"
			case "favorite", "favorites", "favourite", "favourites":
				return "favorite"
			default:
				return "all"
			}
		}

		rankingType := normalizeRankingType(typeParam)

		maxResults := requestLimit
		if maxResults <= 0 {
			maxResults = 50
		}
		if maxResults > 500 {
			maxResults = 500
		}

		mangas, err := h.fetchRanking(clientID, rankingType, maxResults)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Convert RankingManga to regular Manga first
		allRegularMangas := make([]models.Manga, len(mangas))
		for i, rm := range mangas {
			allRegularMangas[i] = models.Manga{
				ID:            fmt.Sprintf("%d", rm.ID),
				Title:         rm.Title,
				Status:        rm.Status,
				TotalChapters: rm.NumChapters,
				Description:   rm.Synopsis,
				Genres:        rm.Genres,
				CoverURL:      rm.CoverURL,
				MediaType:     mapToCanonicalMediaType(typeParam),
			}
			if len(rm.Authors) > 0 {
				author := strings.TrimSpace(rm.Authors[0].Node.FirstName + " " + rm.Authors[0].Node.LastName)
				if author != "" {
					allRegularMangas[i].Author = author
				}
			}
		}

		// Apply genre filter to ranking results if provided (before pagination)
		if len(genreFilters) > 0 {
			filtered := make([]models.Manga, 0, len(allRegularMangas))
			for _, m := range allRegularMangas {
				for _, g := range m.Genres {
					for _, gf := range genreFilters {
						if strings.EqualFold(strings.TrimSpace(g), gf) {
							filtered = append(filtered, m)
							goto nextManga
						}
					}
				}
			nextManga:
			}
			allRegularMangas = filtered
		}

		// Now paginate after filtering
		offset := (page - 1) * pageSize
		var regularMangas []models.Manga
		if offset < len(allRegularMangas) {
			end := offset + pageSize
			if end > len(allRegularMangas) {
				end = len(allRegularMangas)
			}
			regularMangas = allRegularMangas[offset:end]
		}

		total := len(allRegularMangas)
		pagination := calculatePagination(page, pageSize, total)
		response := models.PaginatedBooksResponse{
			Mangas:     regularMangas,
			Pagination: pagination,
		}

		c.JSON(http.StatusOK, response)
		return
	}

	// Query provided: enforce minimum length
	if len(query) < 3 {
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

	// Optional client-side media type filter (e.g., type=novels, type=manga)
	normalizedType := mapToCanonicalMediaType(typeParam)
	if normalizedType != "" {
		filtered := make([]models.Manga, 0, len(allMangas))
		for _, m := range allMangas {
			mt := mapToCanonicalMediaType(m.MediaType)
			if mt == normalizedType {
				filtered = append(filtered, m)
			}
		}
		allMangas = filtered
	}

	// Optional client-side genre filter (case-insensitive substring match)
	if len(genreFilters) > 0 {
		filtered := make([]models.Manga, 0, len(allMangas))
		for _, m := range allMangas {
			for _, g := range m.Genres {
				for _, gf := range genreFilters {
					if strings.EqualFold(strings.TrimSpace(g), gf) {
						filtered = append(filtered, m)
						goto nextQueryManga
					}
				}
			}
		nextQueryManga:
		}
		allMangas = filtered
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

	// First check if manga exists in local database
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM manga WHERE id = ?)`
	err := database.DB.QueryRow(checkQuery, mangaID).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check manga existence"})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Manga does not exist in database"})
		return
	}

	// Decide source: UUID-style goes to MangaDex, numeric goes to MAL
	isUUID := func(id string) bool {
		return len(id) == 36 && strings.Count(id, "-") == 4
	}

	ctx := context.Background()

	var manga *models.Manga

	if isUUID(mangaID) {
		mangadex := NewMangaDexSource()
		manga, err = mangadex.GetMangaByID(ctx, mangaID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
	} else {
		// Fallback to MAL (numeric IDs)
		for _, ch := range mangaID {
			if ch < '0' || ch > '9' {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Manga ID must be numeric or a MangaDex UUID"})
				return
			}
		}

		manga, err = h.externalSource.GetMangaByID(ctx, mangaID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
	}

	// Add local rating statistics to external manga data
	ratingQuery := `
		SELECT 
			ROUND(AVG(user_rating), 1) as average_rating,
			COUNT(*) as total_ratings,
			COUNT(CASE WHEN ROUND(user_rating) = 5 THEN 1 END) as rating_5,
			COUNT(CASE WHEN ROUND(user_rating) = 4 THEN 1 END) as rating_4,
			COUNT(CASE WHEN ROUND(user_rating) = 3 THEN 1 END) as rating_3,
			COUNT(CASE WHEN ROUND(user_rating) = 2 THEN 1 END) as rating_2,
			COUNT(CASE WHEN ROUND(user_rating) = 1 THEN 1 END) as rating_1
		FROM user_progress
		WHERE manga_id = ? AND user_rating IS NOT NULL
	`

	var avgRating sql.NullFloat64
	var totalCount int
	var r5, r4, r3, r2, r1 int

	err = database.DB.QueryRow(ratingQuery, mangaID).Scan(
		&avgRating, &totalCount, &r5, &r4, &r3, &r2, &r1,
	)

	// Build response with external manga data + local rating stats
	response := make(map[string]interface{})

	// Convert manga struct to map (preserve all external API fields)
	mangaJSON, _ := json.Marshal(manga)
	json.Unmarshal(mangaJSON, &response)

	// Add rating_stats if we have ratings
	if err == nil && totalCount > 0 {
		response["rating_stats"] = gin.H{
			"average":     avgRating.Float64,
			"total_count": totalCount,
			"distribution": gin.H{
				"5": r5,
				"4": r4,
				"3": r3,
				"2": r2,
				"1": r1,
			},
		}
	}

	c.JSON(http.StatusOK, response)
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

	// Get rating statistics
	ratingQuery := `
		SELECT 
			ROUND(AVG(user_rating), 1) as average_rating,
			COUNT(*) as total_ratings,
			COUNT(CASE WHEN ROUND(user_rating) = 5 THEN 1 END) as rating_5,
			COUNT(CASE WHEN ROUND(user_rating) = 4 THEN 1 END) as rating_4,
			COUNT(CASE WHEN ROUND(user_rating) = 3 THEN 1 END) as rating_3,
			COUNT(CASE WHEN ROUND(user_rating) = 2 THEN 1 END) as rating_2,
			COUNT(CASE WHEN ROUND(user_rating) = 1 THEN 1 END) as rating_1
		FROM user_progress
		WHERE manga_id = ? AND user_rating IS NOT NULL
	`

	var avgRating sql.NullFloat64
	var totalCount int
	var r5, r4, r3, r2, r1 int

	err = database.DB.QueryRow(ratingQuery, mangaID).Scan(
		&avgRating, &totalCount, &r5, &r4, &r3, &r2, &r1,
	)

	// Build response with rating stats
	response := gin.H{
		"id":             manga.ID,
		"title":          manga.Title,
		"author":         manga.Author,
		"genres":         manga.Genres,
		"status":         manga.Status,
		"total_chapters": manga.TotalChapters,
		"description":    manga.Description,
		"cover_url":      manga.CoverURL,
	}

	// Only include rating_stats if there are ratings
	if err == nil && totalCount > 0 {
		response["rating_stats"] = gin.H{
			"average":     avgRating.Float64,
			"total_count": totalCount,
			"distribution": gin.H{
				"5": r5,
				"4": r4,
				"3": r3,
				"2": r2,
				"1": r1,
			},
		}
	}

	c.JSON(http.StatusOK, response)
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

	// Broadcast SSE notification
	if h.broker != nil {
		h.broker.Broadcast("manga_created", fmt.Sprintf("New manga added: %s", manga.Title), gin.H{
			"id":     manga.ID,
			"title":  manga.Title,
			"author": manga.Author,
			"cover":  manga.CoverURL,
		})
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

// RefreshManga updates a manga's total_chapters from external sources (MangaDex via MAL mapping)
// and broadcasts a UDP chapter_release notification if chapters increased.
func (h *Handler) RefreshManga(c *gin.Context) {
	mangaID := c.Param("id") // MAL ID stored in DB
	if mangaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "manga id required"})
		return
	}

	// Load current manga from DB
	var title string
	var oldTotal int
	err := database.DB.QueryRow(`SELECT title, total_chapters FROM manga WHERE id = ?`, mangaID).Scan(&title, &oldTotal)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "manga not found"})
		return
	}

	// Map MAL ID -> MangaDex ID and fetch aggregate chapter count
	mangadexID := FetchMangaDexID(mangaID)
	if mangadexID == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to map to MangaDex ID"})
		return
	}
	newTotal := FetchMangaDexChapterCount(mangadexID)
	if newTotal <= 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch chapter count"})
		return
	}

	// Update DB if increased
	delta := 0
	if newTotal > oldTotal {
		_, updErr := database.DB.Exec(`UPDATE manga SET total_chapters = ? WHERE id = ?`, newTotal, mangaID)
		if updErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update database"})
			return
		}
		delta = newTotal - oldTotal

		// Broadcast SSE notification about new chapters
		if h.broker != nil {
			h.broker.Broadcast("chapter_release", fmt.Sprintf("%d new chapter(s) for %s", delta, title), gin.H{
				"manga_id":     mangaID,
				"title":        title,
				"old_total":    oldTotal,
				"new_total":    newTotal,
				"new_chapters": delta,
			})
		}

		// Forward a UDP chapter_release notification to UDP server (global broadcast)
		// Build payload
		data := map[string]interface{}{
			"manga_id":  mangaID,
			"title":     title,
			"old_total": oldTotal,
			"new_total": newTotal,
			"delta":     delta,
		}
		// Resolve UDP addr and send
		udpAddr := resolveUDPAddr()
		_ = forwardChapterReleaseToUDP(udpAddr, data)
	}

	c.JSON(http.StatusOK, gin.H{
		"manga_id": mangaID,
		"title":    title,
		"old":      oldTotal,
		"new":      newTotal,
		"delta":    delta,
		"updated":  newTotal > oldTotal,
	})
}

// forwardChapterReleaseToUDP sends a global UDP notification (no user_id) for chapter releases
func forwardChapterReleaseToUDP(udpAddr string, data map[string]interface{}) error {
	addr, err := net.ResolveUDPAddr("udp", udpAddr)
	if err != nil {
		return err
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	msg := map[string]interface{}{
		"type":       "notification",
		"event_type": "chapter_release",
		"user_id":    "",
		"data":       data,
		"timestamp":  time.Now().Format(time.RFC3339),
	}
	bytes, _ := json.Marshal(msg)
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Write(bytes)
	return err
}

// RefreshAllManga updates total_chapters for ALL manga in the database
func (h *Handler) RefreshAllManga(c *gin.Context) {
	// Get all manga from DB
	rows, err := database.DB.Query(`SELECT id, title, total_chapters FROM manga`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query manga"})
		return
	}
	defer rows.Close()

	type MangaRow struct {
		ID       string
		Title    string
		OldTotal int
	}

	var allManga []MangaRow
	for rows.Next() {
		var m MangaRow
		if err := rows.Scan(&m.ID, &m.Title, &m.OldTotal); err != nil {
			continue
		}
		allManga = append(allManga, m)
	}

	totalProcessed := 0
	totalUpdated := 0
	totalFailed := 0
	var updates []map[string]interface{}

	udpAddr := resolveUDPAddr()

	for _, manga := range allManga {
		totalProcessed++

		// Map MAL ID -> MangaDex ID
		mangadexID := FetchMangaDexID(manga.ID)
		if mangadexID == "" {
			totalFailed++
			continue
		}

		// Fetch chapter count
		newTotal := FetchMangaDexChapterCount(mangadexID)
		if newTotal <= 0 {
			totalFailed++
			continue
		}

		// Update if increased
		if newTotal > manga.OldTotal {
			_, err := database.DB.Exec(`UPDATE manga SET total_chapters = ? WHERE id = ?`, newTotal, manga.ID)
			if err != nil {
				totalFailed++
				continue
			}

			delta := newTotal - manga.OldTotal
			totalUpdated++

			// Broadcast UDP notification
			data := map[string]interface{}{
				"manga_id":  manga.ID,
				"title":     manga.Title,
				"old_total": manga.OldTotal,
				"new_total": newTotal,
				"delta":     delta,
			}
			forwardChapterReleaseToUDP(udpAddr, data)

			updates = append(updates, map[string]interface{}{
				"manga_id": manga.ID,
				"title":    manga.Title,
				"old":      manga.OldTotal,
				"new":      newTotal,
				"delta":    delta,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_processed": totalProcessed,
		"total_updated":   totalUpdated,
		"total_failed":    totalFailed,
		"updates":         updates,
	})
}

// resolveUDPAddr determines UDP server address from env or local IP
func resolveUDPAddr() string {
	udpPort := os.Getenv("UDP_PORT")
	if udpPort == "" {
		udpPort = "9091"
	}
	udpHost := os.Getenv("UDP_HOST")
	if udpHost == "" {
		udpHost = utils.GetLocalIP()
	}
	return net.JoinHostPort(udpHost, udpPort)
}

func (h *Handler) fetchRanking(clientID, rankingType string, limit int) ([]RankingManga, error) {
	apiURL := "https://api.myanimelist.net/v2/manga/ranking"
	params := url.Values{}
	params.Add("ranking_type", rankingType)
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("fields", "id,title,main_picture,authors{name,first_name,last_name},status,num_chapters,synopsis,genres")

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
	// Deprecated: routing is now handled by /manga/search with type-only queries
	c.JSON(http.StatusGone, gin.H{"error": "Deprecated. Use /manga/search?type=<ranking_type> instead."})
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
	limit := parseIntQuery(c, "limit", 20) // default to smaller page to avoid heavy payloads
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
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
