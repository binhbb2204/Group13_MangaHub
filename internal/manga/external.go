package manga

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/models"
)

type ExternalSource interface {
	Search(ctx context.Context, query string, limit, offset int) ([]models.Manga, error)
	GetMangaByID(ctx context.Context, id string) (*models.Manga, error)
}

type MALSource struct {
	BaseURL  string
	ClientID string
	Client   *http.Client
}

type MangaDexSource struct {
	BaseURL  string
	ClientID string
	Client   *http.Client
}

func NewMangaDexSource() *MangaDexSource {
	return &MangaDexSource{
		BaseURL:  "https://api.mangadex.org",
		ClientID: strings.TrimSpace(os.Getenv("MANGADEX_CLIENT_ID")),
		Client:   &http.Client{Timeout: 10 * time.Second},
	}
}

type mangaDexSearchResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Title         map[string]string   `json:"title"`
			AltTitles     []map[string]string `json:"altTitles"`
			Description   map[string]string   `json:"description"`
			Status        string              `json:"status"`
			Year          int                 `json:"year"`
			ContentRating string              `json:"contentRating"`
			Tags          []struct {
				Attributes struct {
					Name map[string]string `json:"name"`
				} `json:"attributes"`
			} `json:"tags"`
		} `json:"attributes"`
		Relationships []struct {
			Type       string                 `json:"type"`
			ID         string                 `json:"id"`
			Attributes map[string]interface{} `json:"attributes"`
		} `json:"relationships"`
	} `json:"data"`
}

func (m *MangaDexSource) Search(ctx context.Context, query string, limit, offset int) ([]models.Manga, error) {
	if limit <= 0 {
		limit = 20
	}

	u, _ := url.Parse(m.BaseURL + "/manga")
	qs := u.Query()
	qs.Set("title", query)
	qs.Set("limit", fmt.Sprintf("%d", limit))
	qs.Set("offset", fmt.Sprintf("%d", offset))
	qs.Set("includes[]", "author")
	qs.Set("includes[]", "artist")
	qs.Set("includes[]", "cover_art")
	u.RawQuery = qs.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("User-Agent", "MangaHub/1.0")

	res, err := m.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MangaDex API request failed: %s", res.Status)
	}

	var r mangaDexSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}

	out := make([]models.Manga, 0, len(r.Data))
	for _, d := range r.Data {
		manga := convertMangaDexToManga(d)
		out = append(out, manga)
	}
	return out, nil
}

func (m *MangaDexSource) GetMangaByID(ctx context.Context, id string) (*models.Manga, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/manga/%s", m.BaseURL, id))
	qs := u.Query()
	qs.Set("includes[]", "author")
	qs.Set("includes[]", "artist")
	qs.Set("includes[]", "cover_art")
	u.RawQuery = qs.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("User-Agent", "MangaHub/1.0")

	res, err := m.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MangaDex API request failed: %s", res.Status)
	}

	var response struct {
		Data struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Attributes struct {
				Title         map[string]string   `json:"title"`
				AltTitles     []map[string]string `json:"altTitles"`
				Description   map[string]string   `json:"description"`
				Status        string              `json:"status"`
				Year          int                 `json:"year"`
				ContentRating string              `json:"contentRating"`
				Tags          []struct {
					Attributes struct {
						Name map[string]string `json:"name"`
					} `json:"attributes"`
				} `json:"tags"`
			} `json:"attributes"`
			Relationships []struct {
				Type       string                 `json:"type"`
				ID         string                 `json:"id"`
				Attributes map[string]interface{} `json:"attributes"`
			} `json:"relationships"`
		} `json:"data"`
	}

	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, err
	}

	manga := convertMangaDexToManga(response.Data)
	return &manga, nil
}

func convertMangaDexToManga(data interface{}) models.Manga {
	var id, title, author, description, status, coverURL, mangaDexID string
	genres := []string{}

	// Type assertion to handle the struct
	type MangaData struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Title         map[string]string   `json:"title"`
			AltTitles     []map[string]string `json:"altTitles"`
			Description   map[string]string   `json:"description"`
			Status        string              `json:"status"`
			Year          int                 `json:"year"`
			ContentRating string              `json:"contentRating"`
			Tags          []struct {
				Attributes struct {
					Name map[string]string `json:"name"`
				} `json:"attributes"`
			} `json:"tags"`
		} `json:"attributes"`
		Relationships []struct {
			Type       string                 `json:"type"`
			ID         string                 `json:"id"`
			Attributes map[string]interface{} `json:"attributes"`
		} `json:"relationships"`
	}

	var d MangaData
	// Convert data to JSON and back to struct
	jsonData, _ := json.Marshal(data)
	json.Unmarshal(jsonData, &d)

	id = d.ID
	mangaDexID = d.ID

	// Get title (prefer English, fallback to first available)
	if enTitle, ok := d.Attributes.Title["en"]; ok && enTitle != "" {
		title = enTitle
	} else {
		for _, t := range d.Attributes.Title {
			if t != "" {
				title = t
				break
			}
		}
	}

	// Get description (prefer English)
	if enDesc, ok := d.Attributes.Description["en"]; ok {
		description = enDesc
	} else {
		for _, desc := range d.Attributes.Description {
			if desc != "" {
				description = desc
				break
			}
		}
	}

	// Get status
	status = strings.ToLower(d.Attributes.Status)
	if status == "completed" {
		status = "completed"
	} else if status == "ongoing" {
		status = "ongoing"
	}

	// Get author
	for _, rel := range d.Relationships {
		if rel.Type == "author" {
			if name, ok := rel.Attributes["name"].(string); ok {
				author = name
				break
			}
		}
	}

	// Get cover
	for _, rel := range d.Relationships {
		if rel.Type == "cover_art" {
			if fileName, ok := rel.Attributes["fileName"].(string); ok {
				coverURL = fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s.256.jpg", id, fileName)
				break
			}
		}
	}

	// Get genres/tags
	for _, tag := range d.Attributes.Tags {
		if enName, ok := tag.Attributes.Name["en"]; ok && enName != "" {
			genres = append(genres, enName)
		}
	}

	return models.Manga{
		ID:          id,
		MangaDexID:  mangaDexID,
		Title:       title,
		Author:      author,
		Genres:      genres,
		Status:      status,
		Description: description,
		CoverURL:    coverURL,
	}
}

// Chapter represents a manga chapter from MangaDex
type Chapter struct {
	ID         string `json:"id"`
	Chapter    string `json:"chapter"`
	Title      string `json:"title"`
	Pages      int    `json:"pages"`
	Volume     string `json:"volume"`
	Language   string `json:"language"`
	ReadableAt string `json:"readableAt"`
}

// GetChapters fetches chapters for a manga from MangaDex
func (m *MangaDexSource) GetChapters(ctx context.Context, mangaDexID string, language string, limit int) ([]Chapter, error) {
	if limit <= 0 {
		limit = 100
	}
	if language == "" {
		language = "en"
	}

	u, _ := url.Parse(fmt.Sprintf("%s/manga/%s/feed", m.BaseURL, mangaDexID))
	qs := u.Query()
	qs.Add("translatedLanguage[]", language)
	qs.Set("limit", fmt.Sprintf("%d", limit))
	qs.Set("order[chapter]", "asc")
	qs.Add("contentRating[]", "safe")
	qs.Add("contentRating[]", "suggestive")
	u.RawQuery = qs.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("User-Agent", "MangaHub/1.0")

	res, err := m.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MangaDex API request failed: %s", res.Status)
	}

	var response struct {
		Data []struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Attributes struct {
				Chapter        string `json:"chapter"`
				Title          string `json:"title"`
				Pages          int    `json:"pages"`
				Volume         string `json:"volume"`
				TranslatedLang string `json:"translatedLanguage"`
				ReadableAt     string `json:"readableAt"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, err
	}

	chapters := make([]Chapter, 0, len(response.Data))
	for _, d := range response.Data {
		// Only include chapters that have actual pages available
		if d.Attributes.Pages > 0 {
			chapters = append(chapters, Chapter{
				ID:         d.ID,
				Chapter:    d.Attributes.Chapter,
				Title:      d.Attributes.Title,
				Pages:      d.Attributes.Pages,
				Volume:     d.Attributes.Volume,
				Language:   d.Attributes.TranslatedLang,
				ReadableAt: d.Attributes.ReadableAt,
			})
		}
	}

	return chapters, nil
}

// ChapterPages represents the page URLs for a chapter
type ChapterPages struct {
	BaseURL string   `json:"baseUrl"`
	Hash    string   `json:"hash"`
	Data    []string `json:"data"` // Page filenames
}

// GetChapterPages fetches the page URLs for a specific chapter
func (m *MangaDexSource) GetChapterPages(ctx context.Context, chapterID string) (*ChapterPages, error) {
	u := fmt.Sprintf("%s/at-home/server/%s", m.BaseURL, chapterID)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("User-Agent", "MangaHub/1.0")

	res, err := m.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MangaDex API request failed: %s", res.Status)
	}

	var response struct {
		BaseURL string `json:"baseUrl"`
		Chapter struct {
			Hash string   `json:"hash"`
			Data []string `json:"data"`
		} `json:"chapter"`
	}

	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &ChapterPages{
		BaseURL: response.BaseURL,
		Hash:    response.Chapter.Hash,
		Data:    response.Chapter.Data,
	}, nil
}

// GetPageURL constructs the full URL for a manga page
func (cp *ChapterPages) GetPageURL(pageIndex int) string {
	if pageIndex < 0 || pageIndex >= len(cp.Data) {
		return ""
	}
	return fmt.Sprintf("%s/data/%s/%s", cp.BaseURL, cp.Hash, cp.Data[pageIndex])
}

func NewMALSource() *MALSource {
	return &MALSource{
		BaseURL:  "https://api.myanimelist.net/v2",
		ClientID: strings.TrimSpace(os.Getenv("MAL_CLIENT_ID")),
		Client:   &http.Client{Timeout: 10 * time.Second},
	}
}

type malSearchRes struct {
	Data []struct {
		Node struct {
			ID          int    `json:"id"`
			Title       string `json:"title"`
			MainPicture *struct {
				Medium string `json:"medium"`
				Large  string `json:"large"`
			} `json:"main_picture"`
			AlternativeTitles *struct {
				Synonyms []string `json:"synonyms"`
				En       string   `json:"en"`
				Ja       string   `json:"ja"`
			} `json:"alternative_titles"`
			Synopsis    string `json:"synopsis"`
			NumChapters int    `json:"num_chapters"`
			Status      string `json:"status"`
			Genres      []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"genres"`
			Authors []struct {
				Node struct {
					ID        int    `json:"id"`
					FirstName string `json:"first_name"`
					LastName  string `json:"last_name"`
				} `json:"node"`
				Role string `json:"role"`
			} `json:"authors"`
		} `json:"node"`
	} `json:"data"`
	Paging *struct {
		Next string `json:"next"`
	} `json:"paging"`
}

type malMangaDetailRes struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	MainPicture *struct {
		Medium string `json:"medium"`
		Large  string `json:"large"`
	} `json:"main_picture"`
	AlternativeTitles *struct {
		Synonyms []string `json:"synonyms"`
		En       string   `json:"en"`
		Ja       string   `json:"ja"`
	} `json:"alternative_titles"`
	StartDate       string  `json:"start_date"`
	EndDate         string  `json:"end_date"`
	Synopsis        string  `json:"synopsis"`
	Mean            float64 `json:"mean"`
	Rank            int     `json:"rank"`
	Popularity      int     `json:"popularity"`
	NumListUsers    int     `json:"num_list_users"`
	NumScoringUsers int     `json:"num_scoring_users"`
	MediaType       string  `json:"media_type"`
	NumChapters     int     `json:"num_chapters"`
	NumVolumes      int     `json:"num_volumes"`
	Status          string  `json:"status"`
	Background      string  `json:"background"`
	Genres          []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	Authors []struct {
		Node struct {
			ID        int    `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"node"`
		Role string `json:"role"`
	} `json:"authors"`
	Serialization []struct {
		Node struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"node"`
	} `json:"serialization"`
}

func (m *MALSource) Search(ctx context.Context, q string, limit, offset int) ([]models.Manga, error) {
	if m.ClientID == "" {
		return nil, fmt.Errorf("MAL_CLIENT_ID not set in environment")
	}

	if limit <= 0 {
		limit = 20
	}

	u, _ := url.Parse(m.BaseURL + "/manga")
	qs := u.Query()
	if q != "" {
		qs.Set("q", q)
	}
	qs.Set("limit", fmt.Sprintf("%d", limit))
	if offset > 0 {
		qs.Set("offset", fmt.Sprintf("%d", offset))
	}
	qs.Set("fields", "id,title,main_picture,alternative_titles,synopsis,num_chapters,status,genres,authors{first_name,last_name}")
	u.RawQuery = qs.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("X-MAL-Client-ID", m.ClientID)
	req.Header.Set("User-Agent", "MangaHub/1.0 (+github.com/binhbb2204/Manga-Hub-Group13)")
	res, err := m.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MAL API request failed: %s", res.Status)
	}

	var r malSearchRes
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}

	out := make([]models.Manga, 0, len(r.Data))
	for _, d := range r.Data {
		manga := convertMALToManga(d.Node.ID, d.Node.Title, d.Node.MainPicture, d.Node.AlternativeTitles,
			d.Node.Synopsis, d.Node.NumChapters, d.Node.Status, d.Node.Genres, d.Node.Authors)
		out = append(out, manga)
	}
	return out, nil
}

func (m *MALSource) GetMangaByID(ctx context.Context, id string) (*models.Manga, error) {
	if m.ClientID == "" {
		return nil, fmt.Errorf("MAL_CLIENT_ID not set in environment")
	}

	u, _ := url.Parse(fmt.Sprintf("%s/manga/%s", m.BaseURL, id))
	qs := u.Query()
	qs.Set("fields", "id,title,main_picture,alternative_titles,start_date,end_date,synopsis,mean,rank,popularity,num_list_users,num_scoring_users,media_type,status,genres,num_volumes,num_chapters,authors{first_name,last_name},background,serialization{name}")
	u.RawQuery = qs.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("X-MAL-Client-ID", m.ClientID)
	req.Header.Set("User-Agent", "MangaHub/1.0 (+github.com/binhbb2204/Manga-Hub-Group13)")

	res, err := m.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MAL API request failed: %s", res.Status)
	}

	var r malMangaDetailRes
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}

	manga := convertMALDetailToManga(r)

	return &manga, nil
}

func convertMALDetailToManga(r malMangaDetailRes) models.Manga {
	coverURL := ""
	if r.MainPicture != nil {
		if r.MainPicture.Large != "" {
			coverURL = r.MainPicture.Large
		} else {
			coverURL = r.MainPicture.Medium
		}
	}

	genreList := []string{}
	for _, g := range r.Genres {
		genreList = append(genreList, g.Name)
	}

	authorName := ""
	authorsList := []map[string]interface{}{}
	for _, a := range r.Authors {
		name := strings.TrimSpace(a.Node.FirstName + " " + a.Node.LastName)
		authorsList = append(authorsList, map[string]interface{}{
			"node": map[string]interface{}{
				"first_name": a.Node.FirstName,
				"last_name":  a.Node.LastName,
			},
			"role": a.Role,
		})
		if authorName == "" && (a.Role == "Story" || a.Role == "Story & Art") {
			authorName = name
		}
	}
	if authorName == "" && len(r.Authors) > 0 {
		authorName = strings.TrimSpace(r.Authors[0].Node.FirstName + " " + r.Authors[0].Node.LastName)
	}

	altTitlesMap := map[string]interface{}{}
	if r.AlternativeTitles != nil {
		altTitlesMap["en"] = r.AlternativeTitles.En
		altTitlesMap["ja"] = r.AlternativeTitles.Ja
		altTitlesMap["synonyms"] = r.AlternativeTitles.Synonyms
	}

	serializationList := []map[string]interface{}{}
	for _, s := range r.Serialization {
		serializationList = append(serializationList, map[string]interface{}{
			"node": map[string]interface{}{
				"name": s.Node.Name,
			},
		})
	}

	statusLower := strings.ToLower(r.Status)
	if statusLower == "finished" {
		statusLower = "completed"
	}

	manga := models.Manga{
		ID:                fmt.Sprintf("%d", r.ID),
		Title:             r.Title,
		Author:            authorName,
		Genres:            genreList,
		Status:            statusLower,
		TotalChapters:     r.NumChapters,
		Description:       r.Synopsis,
		CoverURL:          coverURL,
		AlternativeTitles: altTitlesMap,
		StartDate:         r.StartDate,
		EndDate:           r.EndDate,
		Mean:              r.Mean,
		Rank:              r.Rank,
		Popularity:        r.Popularity,
		NumListUsers:      r.NumListUsers,
		NumScoringUsers:   r.NumScoringUsers,
		MediaType:         r.MediaType,
		NumVolumes:        r.NumVolumes,
		Authors:           authorsList,
		Serialization:     serializationList,
		Background:        r.Background,
	}

	// Fetch MangaDex ID
	manga.MangaDexID = fetchMangaDexID(r.Title)

	return manga
}

func convertMALToManga(id int, title string, mainPicture interface{}, altTitles interface{},
	synopsis string, numChapters int, status string, genres interface{}, authors interface{}) models.Manga {

	coverURL := ""
	if pic, ok := mainPicture.(*struct {
		Medium string `json:"medium"`
		Large  string `json:"large"`
	}); ok && pic != nil {
		if pic.Large != "" {
			coverURL = pic.Large
		} else {
			coverURL = pic.Medium
		}
	}

	// Convert genres
	genreList := []string{}
	if g, ok := genres.([]struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}); ok {
		for _, genre := range g {
			genreList = append(genreList, genre.Name)
		}
	}

	// Convert author
	authorName := ""
	if a, ok := authors.([]struct {
		Node struct {
			ID        int    `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"node"`
		Role string `json:"role"`
	}); ok && len(a) > 0 {
		for _, author := range a {
			if author.Role == "Story" || author.Role == "Story & Art" {
				authorName = strings.TrimSpace(author.Node.FirstName + " " + author.Node.LastName)
				if authorName != "" {
					break
				}
			}
		}
		if authorName == "" && len(a) > 0 {
			authorName = strings.TrimSpace(a[0].Node.FirstName + " " + a[0].Node.LastName)
		}
	}

	statusLower := strings.ToLower(status)
	if statusLower == "finished" {
		statusLower = "completed"
	}

	manga := models.Manga{
		ID:            fmt.Sprintf("%d", id),
		Title:         title,
		Author:        authorName,
		Genres:        genreList,
		Status:        statusLower,
		TotalChapters: numChapters,
		Description:   synopsis,
		CoverURL:      coverURL,
	}

	// Fetch MangaDex ID
	manga.MangaDexID = fetchMangaDexID(title)

	return manga
}

func fetchMangaDexID(title string) string {
	if title == "" {
		return ""
	}

	mangadex := NewMangaDexSource()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := mangadex.Search(ctx, title, 1, 0)
	if err != nil || len(results) == 0 {
		return ""
	}

	return results[0].MangaDexID
}

func NewExternalSourceFromEnv() (ExternalSource, error) {
	clientID := strings.TrimSpace(os.Getenv("MAL_CLIENT_ID"))
	if clientID == "" {
		return nil, fmt.Errorf("MAL_CLIENT_ID is required in environment")
	}
	return NewMALSource(), nil
}
