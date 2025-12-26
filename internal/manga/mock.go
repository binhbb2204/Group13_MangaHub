package manga

import (
	"context"
	"fmt"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/models"
)

// MockExternalSource implements ExternalSource for testing
type MockExternalSource struct {
	// Simulated manga database
	mangaDB map[string]*models.Manga

	// Control flags for testing error scenarios
	ShouldFailSearch  bool
	ShouldFailGetByID bool
}

// NewMockExternalSource creates a new mock source with some test data
func NewMockExternalSource() *MockExternalSource {
	return &MockExternalSource{
		mangaDB: map[string]*models.Manga{
			"mangaX": {
				ID:            "mangaX",
				Title:         "Test Manga X",
				Author:        "Test Author",
				Status:        "ongoing",
				TotalChapters: 150,
				MangaDexID:    "test-mangadex-id-x",
			},
			"manga1": {
				ID:            "manga1",
				Title:         "Test Manga 1",
				Author:        "Test Author 1",
				Status:        "completed",
				TotalChapters: 100,
				MangaDexID:    "test-mangadex-id-1",
			},
		},
	}
}

// Search implements ExternalSource.Search for testing
func (m *MockExternalSource) Search(ctx context.Context, query string, limit, offset int) ([]models.Manga, error) {
	if m.ShouldFailSearch {
		return nil, fmt.Errorf("mock search error")
	}

	// Simple mock: return all manga that match the query in title
	var results []models.Manga
	for _, manga := range m.mangaDB {
		results = append(results, *manga)
		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

// GetMangaByID implements ExternalSource.GetMangaByID for testing
func (m *MockExternalSource) GetMangaByID(ctx context.Context, id string) (*models.Manga, error) {
	if m.ShouldFailGetByID {
		return nil, fmt.Errorf("mock getbyid error")
	}

	manga, ok := m.mangaDB[id]
	if !ok {
		return nil, fmt.Errorf("manga not found: %s", id)
	}

	return manga, nil
}

// AddManga adds a manga to the mock database (useful for test setup)
func (m *MockExternalSource) AddManga(manga *models.Manga) {
	m.mangaDB[manga.ID] = manga
}
