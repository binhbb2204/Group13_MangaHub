package grpc

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/models"
	"github.com/google/uuid"
)

type DBRepository struct {
	db *sql.DB
}

func NewDBRepository(db *sql.DB) *DBRepository {
	return &DBRepository{db: db}
}

func (r *DBRepository) AddManga(ctx context.Context, manga *models.Manga) (*models.Manga, error) {
	if manga == nil {
		return nil, fmt.Errorf("manga is nil")
	}
	if manga.Title == "" {
		return nil, fmt.Errorf("manga title is required")
	}

	manga.ID = uuid.New().String()

	if manga.Status == "" {
		manga.Status = "ongoing"
	}
	if manga.MediaType == "" {
		manga.MediaType = "manga"
	}
	if manga.Genres == nil {
		manga.Genres = []string{}
	}

	genresJSON, err := json.Marshal(manga.Genres)
	if err != nil {
		return nil, fmt.Errorf("serialize genres: %w", err)
	}

	query := `
		INSERT INTO manga (
			id, title, author, genres, status,
			total_chapters, description, cover_url, media_type
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = r.db.ExecContext(
		ctx,
		query,
		manga.ID,
		manga.Title,
		manga.Author,
		string(genresJSON),
		manga.Status,
		manga.TotalChapters,
		manga.Description,
		manga.CoverURL,
		manga.MediaType,
	)
	if err != nil {
		return nil, fmt.Errorf("insert manga: %w", err)
	}

	return manga, nil
}

func (r *DBRepository) GetMangaByID(ctx context.Context, id string) (*models.Manga, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	query := `
		SELECT
			id, title, author, genres, status,
			total_chapters, description, cover_url, media_type
		FROM manga
		WHERE id = ?
	`

	var manga models.Manga
	var genresJSON string

	err := r.db.QueryRowContext(ctx, query, id).Scan(
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

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("manga not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("query manga: %w", err)
	}

	if genresJSON != "" {
		if err := json.Unmarshal([]byte(genresJSON), &manga.Genres); err != nil {
			manga.Genres = []string{}
		}
	}

	return &manga, nil
}

func (r *DBRepository) SearchManga(ctx context.Context, filter *SearchFilter) ([]*models.Manga, int32, error) {
	if filter == nil {
		return nil, 0, fmt.Errorf("filter is nil")
	}

	var (
		queryBuilder strings.Builder
		countBuilder strings.Builder
		args         []interface{}
	)

	queryBuilder.WriteString(`
		SELECT
			id, title, author, genres, status,
			total_chapters, description, cover_url, media_type
		FROM manga WHERE 1=1
	`)

	countBuilder.WriteString(`SELECT COUNT(*) FROM manga WHERE 1=1`)

	if filter.Query != "" {
		cond := ` AND (title LIKE ? OR author LIKE ?)`
		queryBuilder.WriteString(cond)
		countBuilder.WriteString(cond)

		q := "%" + filter.Query + "%"
		args = append(args, q, q)
	}

	if filter.Author != "" {
		cond := ` AND author LIKE ?`
		queryBuilder.WriteString(cond)
		countBuilder.WriteString(cond)

		args = append(args, "%"+filter.Author+"%")
	}

	if filter.Status != "" {
		cond := ` AND status = ?`
		queryBuilder.WriteString(cond)
		countBuilder.WriteString(cond)

		args = append(args, filter.Status)
	}

	var total int32
	if err := r.db.QueryRowContext(ctx, countBuilder.String(), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count manga: %w", err)
	}

	if filter.Limit > 0 {
		queryBuilder.WriteString(" LIMIT ?")
		args = append(args, filter.Limit)
	} else {
		queryBuilder.WriteString(" LIMIT 100")
		args = append(args, 100)
	}

	if filter.Offset > 0 {
		queryBuilder.WriteString(" OFFSET ?")
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search manga: %w", err)
	}
	defer rows.Close()

	genreSet := make(map[string]struct{}, len(filter.Genres))
	for _, g := range filter.Genres {
		genreSet[strings.ToLower(g)] = struct{}{}
	}

	results := make([]*models.Manga, 0)

	for rows.Next() {
		var manga models.Manga
		var genresJSON string

		if err := rows.Scan(
			&manga.ID,
			&manga.Title,
			&manga.Author,
			&genresJSON,
			&manga.Status,
			&manga.TotalChapters,
			&manga.Description,
			&manga.CoverURL,
			&manga.MediaType,
		); err != nil {
			continue
		}

		if genresJSON != "" {
			if err := json.Unmarshal([]byte(genresJSON), &manga.Genres); err != nil {
				manga.Genres = []string{}
			}
		}

		if len(genreSet) > 0 {
			ok := false
			for _, g := range manga.Genres {
				if _, found := genreSet[strings.ToLower(g)]; found {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}

		results = append(results, &manga)
	}

	return results, total, nil
}

func (r *DBRepository) UpdateMangaProgress(ctx context.Context, userID, mangaID string, chapter int32) error {
	if userID == "" || mangaID == "" {
		return fmt.Errorf("userID and mangaID are required")
	}

	query := `
		INSERT INTO user_progress (
			user_id, manga_id, current_chapter, updated_at
		)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, manga_id) DO UPDATE SET
			current_chapter = excluded.current_chapter,
			updated_at = CURRENT_TIMESTAMP
	`

	if _, err := r.db.ExecContext(ctx, query, userID, mangaID, chapter); err != nil {
		return fmt.Errorf("update progress: %w", err)
	}

	return nil
}
