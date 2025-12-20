package grpc

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/models"
	"github.com/google/uuid"
)

type MemoryRepository struct {
	mu       sync.RWMutex
	mangas   map[string]*models.Manga
	progress map[string]map[string]int32
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		mangas:   make(map[string]*models.Manga),
		progress: make(map[string]map[string]int32),
	}
}

func (r *MemoryRepository) AddManga(ctx context.Context, manga *models.Manga) (*models.Manga, error) {
	if manga == nil {
		return nil, fmt.Errorf("manga is nil")
	}
	if manga.Title == "" {
		return nil, fmt.Errorf("manga title is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

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

	r.mangas[manga.ID] = manga
	return manga, nil
}

func (r *MemoryRepository) GetMangaByID(ctx context.Context, id string) (*models.Manga, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	manga, ok := r.mangas[id]
	if !ok {
		return nil, fmt.Errorf("manga not found: %s", id)
	}

	return manga, nil
}

func (r *MemoryRepository) SearchManga(ctx context.Context, filter *SearchFilter) ([]*models.Manga, int32, error) {
	if filter == nil {
		return nil, 0, fmt.Errorf("filter is nil")
	}

	query := strings.ToLower(filter.Query)
	author := strings.ToLower(filter.Author)
	status := strings.ToLower(filter.Status)

	genreSet := make(map[string]struct{}, len(filter.Genres))
	for _, g := range filter.Genres {
		genreSet[strings.ToLower(g)] = struct{}{}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]*models.Manga, 0)

	for _, manga := range r.mangas {
		if query != "" {
			titleOK := strings.Contains(strings.ToLower(manga.Title), query)
			authorOK := strings.Contains(strings.ToLower(manga.Author), query)
			if !titleOK && !authorOK {
				continue
			}
		}

		if author != "" && !strings.Contains(strings.ToLower(manga.Author), author) {
			continue
		}

		if status != "" && strings.ToLower(manga.Status) != status {
			continue
		}

		if len(genreSet) > 0 {
			match := false
			for _, g := range manga.Genres {
				if _, ok := genreSet[strings.ToLower(g)]; ok {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}

		results = append(results, manga)
	}

	total := int32(len(results))

	if filter.Offset > 0 {
		if int(filter.Offset) >= len(results) {
			return []*models.Manga{}, total, nil
		}
		results = results[filter.Offset:]
	}

	if filter.Limit > 0 && int(filter.Limit) < len(results) {
		results = results[:filter.Limit]
	}

	return results, total, nil
}

func (r *MemoryRepository) UpdateMangaProgress(ctx context.Context, userID, mangaID string, chapter int32) error {
	if userID == "" || mangaID == "" {
		return fmt.Errorf("userID and mangaID are required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.mangas[mangaID]; !ok {
		return fmt.Errorf("manga not found: %s", mangaID)
	}

	if r.progress[userID] == nil {
		r.progress[userID] = make(map[string]int32)
	}

	r.progress[userID][mangaID] = chapter
	return nil
}
