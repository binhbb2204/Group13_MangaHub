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

// This is for get manga based on ranking
type Author struct {
	Node struct {
		Name      string `json:"name"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	} `json:"node"`
}

type RankingManga struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Status      string   `json:"status"`
	NumChapters int      `json:"num_chapters"`
	Authors     []Author `json:"authors"`
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

	   if req.Limit <= 0 || req.Limit > 100 {
		   req.Limit = 100
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

	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 100 {
		limit = 100
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

	// If the provided id is not purely numeric, treat it as a slug/name and resolve via search
	isNumeric := true
	for _, ch := range mangaID {
		if ch < '0' || ch > '9' {
			isNumeric = false
			break
		}
	}

	resolvedID := mangaID
	if !isNumeric {
		q := strings.ReplaceAll(mangaID, "-", " ")
		results, err := h.externalSource.Search(ctx, q, 1, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if len(results) == 0 || results[0].ID == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Manga not found"})
			return
		}
		resolvedID = results[0].ID
	}

	manga, err := h.externalSource.GetMangaByID(ctx, resolvedID)
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
	params.Add("fields", "id,title,authors{name,first_name,last_name},status,num_chapters")

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
		mangas = append(mangas, item.Node)
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
	   limitStr := c.DefaultQuery("limit", "100")
	   limit, err := strconv.Atoi(limitStr)
	   if err != nil || limit <= 0 || limit > 100 {
		   limit = 100
	   }

	mangas, err := h.fetchRanking(clientID, rankingType, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"mangas": mangas,
		"count":  len(mangas),
		"type":   rankingType,
	})
}
