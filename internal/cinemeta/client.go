package cinemeta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const DefaultBaseURL = "https://v3-cinemeta.strem.io"

type Video struct {
	ID      string `json:"id"`
	Season  int    `json:"season"`
	Episode int    `json:"episode"`
}

type Series struct {
	ID     string  `json:"id"`
	Videos []Video `json:"videos"`
}

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func (c *Client) Series(ctx context.Context, imdbID string) (Series, error) {
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	endpoint := fmt.Sprintf("%s/meta/series/%s.json", baseURL, url.PathEscape(imdbID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Series{}, fmt.Errorf("create Cinemeta request: %w", err)
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return Series{}, fmt.Errorf("request Cinemeta metadata for %s: %w", imdbID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return Series{}, fmt.Errorf("Cinemeta metadata for %s returned %s", imdbID, resp.Status)
	}

	var envelope struct {
		Meta *Series `json:"meta"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&envelope); err != nil {
		return Series{}, fmt.Errorf("decode Cinemeta metadata for %s: %w", imdbID, err)
	}
	if envelope.Meta == nil || len(envelope.Meta.Videos) == 0 {
		return Series{}, fmt.Errorf("Cinemeta metadata for %s contains no episodes", imdbID)
	}
	return *envelope.Meta, nil
}
