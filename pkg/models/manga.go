package models

type Manga struct {
	ID                string                   `json:"id" db:"id"`
	Title             string                   `json:"title" db:"title"`
	Author            string                   `json:"author" db:"author"`
	Genres            []string                 `json:"genres" db:"genres"`
	Status            string                   `json:"status" db:"status"`
	TotalChapters     int                      `json:"total_chapters" db:"total_chapters"`
	Description       string                   `json:"description" db:"description"`
	CoverURL          string                   `json:"cover_url" db:"cover_url"`
	MangaDexID        string                   `json:"mangadex_id,omitempty" db:"mangadex_id"`
	AlternativeTitles map[string]interface{}   `json:"alternative_titles,omitempty"`
	StartDate         string                   `json:"start_date,omitempty"`
	EndDate           string                   `json:"end_date,omitempty"`
	Mean              float64                  `json:"mean,omitempty"`
	Rank              int                      `json:"rank,omitempty"`
	Popularity        int                      `json:"popularity,omitempty"`
	NumListUsers      int                      `json:"num_list_users,omitempty"`
	NumScoringUsers   int                      `json:"num_scoring_users,omitempty"`
	MediaType         string                   `json:"media_type,omitempty"`
	NumVolumes        int                      `json:"num_volumes,omitempty"`
	Authors           []map[string]interface{} `json:"authors,omitempty"`
	Serialization     []map[string]interface{} `json:"serialization,omitempty"`
	Background        string                   `json:"background,omitempty"`
}

type SearchMangaRequest struct {
	Title  string   `form:"title"`
	Author string   `form:"author"`
	Genre  string   `form:"genre"`  // Single genre for filtering
	Genres []string `form:"genres"` // Multiple genres (for future use)
	Status string   `form:"status"`
	Limit  int      `form:"limit" binding:"min=1"`
	Offset int      `form:"offset" binding:"min=0"`
	Page   int      `form:"page" binding:"min=0"` // Optional: if provided, return only that page
}

type PaginationMeta struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

type PaginatedBooksResponse struct {
	Mangas     []Manga        `json:"mangas"`
	Pagination PaginationMeta `json:"pagination"`
}
