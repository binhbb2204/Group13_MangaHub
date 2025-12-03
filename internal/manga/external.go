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
	BaseURL string
	Token   string
	Client  *http.Client
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

	return models.Manga{
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

	return models.Manga{
		ID:            fmt.Sprintf("%d", id),
		Title:         title,
		Author:        authorName,
		Genres:        genreList,
		Status:        statusLower,
		TotalChapters: numChapters,
		Description:   synopsis,
		CoverURL:      coverURL,
	}
}

func NewExternalSourceFromEnv() (ExternalSource, error) {
	clientID := strings.TrimSpace(os.Getenv("MAL_CLIENT_ID"))
	if clientID == "" {
		return nil, fmt.Errorf("MAL_CLIENT_ID is required in environment")
	}
	return NewMALSource(), nil
}
