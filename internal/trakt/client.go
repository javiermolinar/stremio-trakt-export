package trakt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/javiermolinar/stremio-trakt-export/internal/exporter"
)

const DefaultBaseURL = "https://api.trakt.tv"

type Client struct {
	BaseURL          string
	AuthorizationURL string
	ClientID         string
	ClientSecret     string
	AccessToken      string
	RefreshToken     string
	TokenExpiry      time.Time
	RedirectURI      string
	HTTPClient       *http.Client
}

type historyEntry struct {
	Type  string `json:"type"`
	Movie *struct {
		IDs struct {
			IMDb string `json:"imdb"`
		} `json:"ids"`
	} `json:"movie"`
	Show *struct {
		IDs struct {
			IMDb string `json:"imdb"`
		} `json:"ids"`
	} `json:"show"`
	Episode *struct {
		Season int `json:"season"`
		Number int `json:"number"`
	} `json:"episode"`
}

func (c *Client) ExistingKeys(ctx context.Context) (map[string]struct{}, error) {
	keys := make(map[string]struct{})
	const pageSize = 100
	for page := 1; ; page++ {
		var entries []historyEntry
		headers, err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/sync/history?page=%d&limit=%d", page, pageSize), nil, &entries, true)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			switch entry.Type {
			case "movie":
				if entry.Movie != nil && entry.Movie.IDs.IMDb != "" {
					keys[exporter.MovieKey(entry.Movie.IDs.IMDb)] = struct{}{}
				}
			case "episode":
				if entry.Show != nil && entry.Show.IDs.IMDb != "" && entry.Episode != nil {
					keys[exporter.EpisodeKey(entry.Show.IDs.IMDb, entry.Episode.Season, entry.Episode.Number)] = struct{}{}
				}
			}
		}

		pageCount, _ := strconv.Atoi(headers.Get("X-Pagination-Page-Count"))
		if pageCount > 0 {
			if page >= pageCount {
				break
			}
		} else if len(entries) < pageSize {
			break
		}
	}
	return keys, nil
}

type SyncSummary struct {
	AddedMovies      int `json:"added_movies"`
	AddedEpisodes    int `json:"added_episodes"`
	ExistingMovies   int `json:"existing_movies"`
	ExistingEpisodes int `json:"existing_episodes"`
	NotFound         int `json:"not_found"`
}

type syncResponse struct {
	Added struct {
		Movies   int `json:"movies"`
		Episodes int `json:"episodes"`
	} `json:"added"`
	Existing struct {
		Movies   int `json:"movies"`
		Episodes int `json:"episodes"`
	} `json:"existing"`
	NotFound struct {
		Movies   []json.RawMessage `json:"movies"`
		Shows    []json.RawMessage `json:"shows"`
		Seasons  []json.RawMessage `json:"seasons"`
		Episodes []json.RawMessage `json:"episodes"`
	} `json:"not_found"`
}

func (c *Client) SyncHistory(ctx context.Context, history exporter.History) (SyncSummary, error) {
	var total SyncSummary
	const movieBatchSize = 100
	for start := 0; start < len(history.Movies); start += movieBatchSize {
		end := start + movieBatchSize
		if end > len(history.Movies) {
			end = len(history.Movies)
		}
		summary, err := c.syncBatch(ctx, exporter.History{Movies: history.Movies[start:end]})
		if err != nil {
			return total, err
		}
		total.add(summary)
	}
	// Keeping each show in its own request preserves the show/season/episode
	// hierarchy and avoids constructing a request with every episode at once.
	for _, show := range history.Shows {
		summary, err := c.syncBatch(ctx, exporter.History{Shows: []exporter.Show{show}})
		if err != nil {
			return total, err
		}
		total.add(summary)
	}
	return total, nil
}

func (s *SyncSummary) add(other SyncSummary) {
	s.AddedMovies += other.AddedMovies
	s.AddedEpisodes += other.AddedEpisodes
	s.ExistingMovies += other.ExistingMovies
	s.ExistingEpisodes += other.ExistingEpisodes
	s.NotFound += other.NotFound
}

func (c *Client) syncBatch(ctx context.Context, history exporter.History) (SyncSummary, error) {
	var response syncResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/sync/history", history, &response, true); err != nil {
		return SyncSummary{}, err
	}
	return SyncSummary{
		AddedMovies:      response.Added.Movies,
		AddedEpisodes:    response.Added.Episodes,
		ExistingMovies:   response.Existing.Movies,
		ExistingEpisodes: response.Existing.Episodes,
		NotFound: len(response.NotFound.Movies) + len(response.NotFound.Shows) +
			len(response.NotFound.Seasons) + len(response.NotFound.Episodes),
	}, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, requestBody, responseBody any, authenticated bool) (http.Header, error) {
	var encoded []byte
	var err error
	if requestBody != nil {
		encoded, err = json.Marshal(requestBody)
		if err != nil {
			return nil, fmt.Errorf("encode Trakt request: %w", err)
		}
	}
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return nil, fmt.Errorf("create Trakt request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("trakt-api-key", c.ClientID)
		req.Header.Set("trakt-api-version", "2")
		if authenticated {
			if c.AccessToken == "" {
				return nil, fmt.Errorf("Trakt access token is required")
			}
			req.Header.Set("Authorization", "Bearer "+c.AccessToken)
		}

		httpClient := c.HTTPClient
		if httpClient == nil {
			httpClient = http.DefaultClient
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Trakt %s %s request: %w", method, path, err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
		closeErr := resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read Trakt response: %w", readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close Trakt response: %w", closeErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			if err := waitForRetry(ctx, resp.Header.Get("Retry-After")); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("Trakt %s %s returned %s: %s", method, path, resp.Status, responseError(body))
		}
		if responseBody != nil && len(body) > 0 {
			if err := json.Unmarshal(body, responseBody); err != nil {
				return nil, fmt.Errorf("decode Trakt response: %w", err)
			}
		}
		return resp.Header, nil
	}
	return nil, fmt.Errorf("Trakt request exceeded retry limit")
}

func waitForRetry(ctx context.Context, value string) error {
	delay := 2 * time.Second
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		delay = time.Duration(seconds) * time.Second
	} else if retryAt, err := http.ParseTime(value); err == nil {
		delay = time.Until(retryAt)
		if delay < 0 {
			delay = 0
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func responseError(body []byte) string {
	const maxErrorLength = 512
	message := strings.TrimSpace(string(body))
	if len(message) > maxErrorLength {
		message = message[:maxErrorLength] + "..."
	}
	if message == "" {
		return "empty response"
	}
	return message
}
