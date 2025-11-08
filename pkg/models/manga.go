package models

type Manga struct {
	ID            string   `json:"id" db:"id"`
	Title         string   `json:"title" db:"title"`
	Author        string   `json:"author" db:"author"`
	Genres        []string `json:"genres" db:"genres"`
	Status        string   `json:"status" db:"status"`
	TotalChapters int      `json:"total_chapters" db:"total_chapters"`
	Description   string   `json:"description" db:"description"`
	CoverURL      string   `json:"cover_url" db:"cover_url"`
}

type SearchMangaRequest struct {
	Title  string   `form:"title"`
	Author string   `form:"author"`
	Genre  string   `form:"genre"`  // Single genre for filtering
	Genres []string `form:"genres"` // Multiple genres (for future use)
	Status string   `form:"status"`
	Limit  int      `form:"limit" binding:"min=1,max=100"`
	Offset int      `form:"offset" binding:"min=0"`
}
