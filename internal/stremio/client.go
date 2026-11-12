package stremio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const DefaultAPIURL = "https://api.strem.io/api"

// LibraryItem is the subset of Stremio's internal libraryItem model needed by
// the exporter. The account datastore is not part of the stable addon API.
type LibraryItem struct {
	ID      string  `json:"_id"`
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Removed bool    `json:"removed"`
	Temp    bool    `json:"temp"`
	CTime   *string `json:"_ctime"`
	State   State   `json:"state"`
}

type State struct {
	LastWatched        *string `json:"lastWatched"`
	TimesWatched       int     `json:"timesWatched"`
	FlaggedWatched     int     `json:"flaggedWatched"`
	TimeWatched        int64   `json:"timeWatched"`
	OverallTimeWatched int64   `json:"overallTimeWatched"`
	TimeOffset         int64   `json:"timeOffset"`
	Watched            *string `json:"watched"`
}

type Client struct {
	APIURL     string
	HTTPClient *http.Client
}

// Login exchanges Stremio account credentials for an authKey. The caller is
// responsible for keeping both the password and returned key out of logs.
func (c *Client) Login(ctx context.Context, email, password string) (string, error) {
	baseURL := strings.TrimRight(c.APIURL, "/")
	if baseURL == "" {
		baseURL = DefaultAPIURL
	}
	requestBody, err := json.Marshal(map[string]string{"email": email, "password": password})
	if err != nil {
		return "", fmt.Errorf("encode Stremio login request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/login", bytes.NewReader(requestBody))
	if err != nil {
		return "", fmt.Errorf("create Stremio login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request Stremio login: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read Stremio login response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Stremio login returned %s: %s", resp.Status, apiError(body))
	}
	var envelope struct {
		Result struct {
			AuthKey string `json:"authKey"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", fmt.Errorf("decode Stremio login response: %w", err)
	}
	if len(envelope.Error) > 0 && string(envelope.Error) != "null" {
		return "", fmt.Errorf("Stremio login failed: %s", apiError(envelope.Error))
	}
	if envelope.Result.AuthKey == "" {
		return "", fmt.Errorf("Stremio login response contained no authKey")
	}
	return envelope.Result.AuthKey, nil
}

func (c *Client) Library(ctx context.Context, authKey string) ([]LibraryItem, error) {
	baseURL := strings.TrimRight(c.APIURL, "/")
	if baseURL == "" {
		baseURL = DefaultAPIURL
	}

	requestBody, err := json.Marshal(map[string]any{
		"authKey":    authKey,
		"collection": "libraryItem",
		"all":        true,
	})
	if err != nil {
		return nil, fmt.Errorf("encode Stremio request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/datastoreGet", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("create Stremio request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request Stremio library: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("read Stremio response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Stremio datastoreGet returned %s: %s", resp.Status, apiError(body))
	}

	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode Stremio response: %w", err)
	}
	if len(envelope.Error) > 0 && string(envelope.Error) != "null" {
		return nil, fmt.Errorf("Stremio datastoreGet failed: %s", apiError(envelope.Error))
	}

	var items []LibraryItem
	if err := json.Unmarshal(envelope.Result, &items); err == nil {
		return items, nil
	}
	var wrapped struct {
		Library []LibraryItem `json:"library"`
	}
	if err := json.Unmarshal(envelope.Result, &wrapped); err != nil {
		return nil, fmt.Errorf("decode Stremio library: %w", err)
	}
	if wrapped.Library == nil {
		return nil, fmt.Errorf("Stremio response did not contain a library array")
	}
	return wrapped.Library, nil
}

func apiError(body []byte) string {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		if envelope.Error.Message != "" {
			return envelope.Error.Message
		}
		if envelope.Message != "" {
			return envelope.Message
		}
	}
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
