package manga

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// MangaDexMappingResponse represents the response from MangaDex API
type MangaDexMappingResponse struct {
	Result string `json:"result"`
	Data   []struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Title map[string]string `json:"title"`
		} `json:"attributes"`
		Relationships []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"relationships"`
	} `json:"data"`
}

// FetchMangaDexID fetches the MangaDex UUID for a given MAL ID
// Returns empty string if not found or on error
func FetchMangaDexID(malID string) string {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Use MangaDex Legacy Mapping API to convert MAL ID to MangaDex UUID
	// https://api.mangadex.org/docs/redoc.html#tag/Legacy/operation/post-legacy-mapping
	url := "https://api.mangadex.org/legacy/mapping"

	// Create POST request body with MAL IDs
	payload := fmt.Sprintf(`{"type":"manga","ids":[%s]}`, malID)

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(payload))
	if err != nil {
		log.Printf("[WARN] Failed to create MangaDex request for MAL ID %s: %v", malID, err)
		return ""
	}

	// Set content type header
	req.Header.Set("Content-Type", "application/json")

	// Execute request with custom DNS resolver to bypass localhost DNS hijacking
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// Use custom DNS resolver (Google DNS) to bypass hosts file
				dialer := &net.Dialer{
					Resolver: &net.Resolver{
						PreferGo: true,
						Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
							d := net.Dialer{}
							return d.DialContext(ctx, "udp", "8.8.8.8:53")
						},
					},
				}
				return dialer.DialContext(ctx, "tcp4", addr)
			},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		// Timeout or network error - log but don't fail
		log.Printf("[WARN] MangaDex API error for MAL ID %s: %v", malID, err)
		return ""
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		log.Printf("[WARN] MangaDex API returned status %d for MAL ID %s", resp.StatusCode, malID)
		return ""
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[WARN] Failed to read MangaDex response for MAL ID %s: %v", malID, err)
		return ""
	}

	// Parse JSON response - Legacy Mapping API returns an object with data array
	// Format: {"result":"ok","response":"collection","data":[{"attributes":{"newId":"mangadex_uuid"}}]}
	var response struct {
		Result   string `json:"result"`
		Response string `json:"response"`
		Data     []struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Attributes struct {
				Type     string `json:"type"`
				LegacyID int    `json:"legacyId"`
				NewID    string `json:"newId"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		log.Printf("[WARN] Failed to parse MangaDex response for MAL ID %s: %v", malID, err)
		log.Printf("[DEBUG] Response body was: %s", string(body))
		return ""
	}

	// Extract MangaDex ID from response
	if response.Result == "ok" && len(response.Data) > 0 {
		mangadexID := response.Data[0].Attributes.NewID
		return mangadexID
	}

	// No mapping found
	return ""
}

// FetchMangaDexChapterCount fetches the total number of available chapters from MangaDex
func FetchMangaDexChapterCount(mangadexID string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Use MangaDex aggregate endpoint to get chapter statistics
	url := fmt.Sprintf("https://api.mangadex.org/manga/%s/aggregate?translatedLanguage[]=en", mangadexID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("[WARN] Failed to create MangaDex aggregate request: %v", err)
		return 0
	}

	// Use custom DNS resolver to bypass localhost DNS hijacking
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				dialer := &net.Dialer{
					Resolver: &net.Resolver{
						PreferGo: true,
						Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
							d := net.Dialer{}
							return d.DialContext(ctx, "udp", "8.8.8.8:53")
						},
					},
				}
				return dialer.DialContext(ctx, "tcp4", addr)
			},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[WARN] MangaDex aggregate API error: %v", err)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[WARN] MangaDex aggregate API returned status %d", resp.StatusCode)
		return 0
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[WARN] Failed to read MangaDex aggregate response: %v", err)
		return 0
	}

	// Parse aggregate response to count chapters
	var aggregateResp struct {
		Result  string `json:"result"`
		Volumes map[string]struct {
			Chapters map[string]interface{} `json:"chapters"`
		} `json:"volumes"`
	}

	if err := json.Unmarshal(body, &aggregateResp); err != nil {
		log.Printf("[WARN] Failed to parse MangaDex aggregate response: %v", err)
		return 0
	}

	// Count total chapters across all volumes
	totalChapters := 0
	for _, volume := range aggregateResp.Volumes {
		totalChapters += len(volume.Chapters)
	}

	log.Printf("[INFO] MangaDex ID %s has %d chapters available", mangadexID, totalChapters)
	return totalChapters
}
