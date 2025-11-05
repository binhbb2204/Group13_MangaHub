package manga

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/models"
)

// ExternalSource abstracts a manga provider (e.g., Jikan, MangaDex, RapidAPI wrappers)
type ExternalSource interface {
	Search(ctx context.Context, query string, limit, offset int) ([]models.Manga, error)
}

// ========== MangaDex (official, no key needed for public search) ==========

type MangaDexSource struct {
	BaseURL string
	Client  *http.Client
}

func NewMangaDexSource() *MangaDexSource {
	return &MangaDexSource{
		BaseURL: "https://api.mangadex.org",
		Client:  &http.Client{Timeout: 10 * time.Second},
	}
}

type mangadexSearchResp struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Title       map[string]string `json:"title"`
			Description map[string]string `json:"description"`
			Status      string            `json:"status"`
			LastChapter string            `json:"lastChapter"`
			Tags        []struct {
				Attributes struct {
					Name map[string]string `json:"name"`
				} `json:"attributes"`
			} `json:"tags"`
		} `json:"attributes"`
	} `json:"data"`
}

func firstLocalized(m map[string]string) string {
	if m == nil {
		return ""
	}
	if v, ok := m["en"]; ok && v != "" {
		return v
	}
	for _, v := range m {
		if v != "" {
			return v
		}
	}
	return ""
}

func (mgs *MangaDexSource) Search(ctx context.Context, q string, limit, offset int) ([]models.Manga, error) {
	if limit <= 0 {
		limit = 20
	}
	u, _ := url.Parse(mgs.BaseURL + "/manga")
	qs := u.Query()
	if q != "" {
		qs.Set("title", q)
	}
	qs.Set("limit", fmt.Sprintf("%d", limit))
	if offset > 0 {
		qs.Set("offset", fmt.Sprintf("%d", offset))
	}
	qs.Add("contentRating[]", "safe")
	u.RawQuery = qs.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("User-Agent", "MangaHub/1.0 (+github.com/binhbb2204/Manga-Hub-Group13)")

	resp, err := mgs.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("mangadex search failed: %s", resp.Status)
	}

	var r mangadexSearchResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	out := make([]models.Manga, 0, len(r.Data))
	for _, d := range r.Data {
		genres := make([]string, 0, len(d.Attributes.Tags))
		for _, t := range d.Attributes.Tags {
			name := firstLocalized(t.Attributes.Name)
			if name != "" {
				genres = append(genres, name)
			}
		}
		total := 0
		if lc := strings.TrimSpace(d.Attributes.LastChapter); lc != "" {
			if n, err := strconv.Atoi(lc); err == nil {
				total = n
			}
		}
		out = append(out, models.Manga{
			ID:            d.ID,
			Title:         firstLocalized(d.Attributes.Title),
			Author:        "",
			Genres:        genres,
			Status:        strings.ToLower(d.Attributes.Status),
			TotalChapters: total,
			Description:   firstLocalized(d.Attributes.Description),
			CoverURL:      "",
		})
	}
	return out, nil
}

// ========== Jikan (no key) ==========

type JikanSource struct {
	BaseURL string
	Client  *http.Client
}

func NewJikanSource() *JikanSource {
	return &JikanSource{
		BaseURL: "https://api.jikan.moe/v4",
		Client:  &http.Client{Timeout: 10 * time.Second},
	}
}

type jikanSearchResp struct {
	Data []struct {
		MalID    int    `json:"mal_id"`
		Title    string `json:"title"`
		Synopsis string `json:"synopsis"`
		Chapters int    `json:"chapters"`
		Status   string `json:"status"`
		Genres   []struct {
			Name string `json:"name"`
		} `json:"genres"`
		Images map[string]any `json:"images"`
	} `json:"data"`
}

func (j *JikanSource) Search(ctx context.Context, q string, limit, offset int) ([]models.Manga, error) {
	if limit <= 0 {
		limit = 20
	}
	u, _ := url.Parse(j.BaseURL + "/manga")
	qs := u.Query()
	if q != "" {
		qs.Set("q", q)
	}
	qs.Set("limit", fmt.Sprintf("%d", limit))
	if offset > 0 {
		qs.Set("offset", fmt.Sprintf("%d", offset))
	}
	u.RawQuery = qs.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("User-Agent", "MangaHub/1.0 (+github.com/binhbb2204/Manga-Hub-Group13)")

	resp, err := j.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jikan search failed: %s", resp.Status)
	}

	var r jikanSearchResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}

	out := make([]models.Manga, 0, len(r.Data))
	for _, d := range r.Data {
		cover := ""
		if im, ok := d.Images["jpg"].(map[string]any); ok {
			if v, ok := im["image_url"].(string); ok {
				cover = v
			}
		}
		genres := make([]string, 0, len(d.Genres))
		for _, g := range d.Genres {
			genres = append(genres, g.Name)
		}
		out = append(out, models.Manga{
			ID:            fmt.Sprintf("mal-%d", d.MalID),
			Title:         d.Title,
			Author:        "",
			Genres:        genres,
			Status:        strings.ToLower(d.Status),
			TotalChapters: d.Chapters,
			Description:   d.Synopsis,
			CoverURL:      cover,
		})
	}
	return out, nil
}

// ========== RapidAPI (example) ==========

type RapidAPISource struct {
	BaseURL string
	Host    string
	APIKey  string
	Client  *http.Client
}

func NewRapidAPISource(baseURL, host, key string) *RapidAPISource {
	return &RapidAPISource{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Host:    host,
		APIKey:  key,
		Client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (r *RapidAPISource) Search(ctx context.Context, q string, limit, offset int) ([]models.Manga, error) {
	if r.APIKey == "" || r.Host == "" || r.BaseURL == "" {
		return nil, errors.New("rapidapi not configured: set RAPIDAPI_URL, RAPIDAPI_HOST, RAPIDAPI_KEY")
	}
	// NOTE: Replace path and query params based on the specific RapidAPI provider you choose.
	u, _ := url.Parse(r.BaseURL + "/manga")
	qs := u.Query()
	if q != "" {
		qs.Set("q", q)
	}
	if limit > 0 {
		qs.Set("limit", fmt.Sprintf("%d", limit))
	}
	if offset > 0 {
		qs.Set("offset", fmt.Sprintf("%d", offset))
	}
	u.RawQuery = qs.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("X-RapidAPI-Key", r.APIKey)
	req.Header.Set("X-RapidAPI-Host", r.Host)
	req.Header.Set("User-Agent", "MangaHub/1.0 (+github.com/binhbb2204/Manga-Hub-Group13)")

	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("rapidapi search failed: %s", resp.Status)
	}

	var raw any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	// TODO: map raw -> []models.Manga based on selected provider schema
	return []models.Manga{}, nil
}

// NewExternalSourceFromEnv chooses a source based on env vars.
// MANGA_SOURCE=jikan|mangadex|rapidapi
// For rapidapi, also set RAPIDAPI_URL, RAPIDAPI_HOST, RAPIDAPI_KEY.
func NewExternalSourceFromEnv() (ExternalSource, error) {
	src := strings.ToLower(strings.TrimSpace(os.Getenv("MANGA_SOURCE")))
	switch src {
	case "", "jikan":
		return NewJikanSource(), nil
	case "mangadex":
		return NewMangaDexSource(), nil
	case "rapidapi":
		url := os.Getenv("RAPIDAPI_URL")
		host := os.Getenv("RAPIDAPI_HOST")
		key := os.Getenv("RAPIDAPI_KEY")
		if url == "" || host == "" || key == "" {
			return nil, errors.New("RAPIDAPI_URL, RAPIDAPI_HOST, RAPIDAPI_KEY must be set for rapidapi source")
		}
		return NewRapidAPISource(url, host, key), nil
	default:
		return nil, fmt.Errorf("unknown MANGA_SOURCE: %s", src)
	}
}
