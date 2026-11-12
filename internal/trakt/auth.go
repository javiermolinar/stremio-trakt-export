package trakt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type DeviceAuthorization struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type DevicePoll struct {
	Authorized bool `json:"authorized"`
	Interval   int  `json:"interval"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (c *Client) applyToken(token tokenResponse) error {
	if token.AccessToken == "" {
		return fmt.Errorf("Trakt token response contained no access token")
	}
	c.AccessToken = token.AccessToken
	if token.RefreshToken != "" {
		c.RefreshToken = token.RefreshToken
	}
	if token.ExpiresIn > 0 {
		c.TokenExpiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}
	return nil
}

// Refresh exchanges a single-use refresh token and stores the replacement
// tokens in memory. Callers persisting credentials must save the client after.
func (c *Client) Refresh(ctx context.Context) error {
	if c.RefreshToken == "" {
		return fmt.Errorf("no Trakt refresh token is available")
	}
	request := map[string]string{
		"refresh_token": c.RefreshToken,
		"client_id":     c.ClientID,
		"grant_type":    "refresh_token",
	}
	if c.ClientSecret != "" {
		request["client_secret"] = c.ClientSecret
	}
	if c.RedirectURI != "" {
		request["redirect_uri"] = c.RedirectURI
	}
	var token tokenResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/oauth/token", request, &token, false); err != nil {
		return err
	}
	return c.applyToken(token)
}

func (c *Client) StartDeviceAuthorization(ctx context.Context) (DeviceAuthorization, error) {
	var code DeviceAuthorization
	if _, err := c.doJSON(ctx, http.MethodPost, "/oauth/device/code", map[string]string{
		"client_id": c.ClientID,
	}, &code, false); err != nil {
		return DeviceAuthorization{}, err
	}
	if code.DeviceCode == "" || code.UserCode == "" || code.VerificationURL == "" {
		return DeviceAuthorization{}, fmt.Errorf("Trakt returned an incomplete device authorization response")
	}
	if code.Interval <= 0 {
		code.Interval = 5
	}
	if code.ExpiresIn <= 0 {
		return DeviceAuthorization{}, fmt.Errorf("Trakt returned an invalid device authorization expiry")
	}
	return code, nil
}

// PollDeviceAuthorization makes one device-token poll. A pending authorization
// is not an error. On success the access token is kept only in Client memory.
func (c *Client) PollDeviceAuthorization(ctx context.Context, code DeviceAuthorization) (DevicePoll, error) {
	status, body, err := c.requestDeviceToken(ctx, code.DeviceCode)
	if err != nil {
		return DevicePoll{}, err
	}
	switch status {
	case http.StatusOK:
		var token tokenResponse
		if err := json.Unmarshal(body, &token); err != nil {
			return DevicePoll{}, fmt.Errorf("decode Trakt token response: %w", err)
		}
		if err := c.applyToken(token); err != nil {
			return DevicePoll{}, err
		}
		return DevicePoll{Authorized: true, Interval: code.Interval}, nil
	case http.StatusBadRequest: // Authorization is still pending.
		return DevicePoll{Interval: code.Interval}, nil
	case http.StatusTooManyRequests:
		return DevicePoll{Interval: code.Interval + 5}, nil
	case http.StatusNotFound:
		return DevicePoll{}, fmt.Errorf("Trakt rejected the device code")
	case http.StatusConflict:
		return DevicePoll{}, fmt.Errorf("Trakt device code was already used")
	case http.StatusGone:
		return DevicePoll{}, fmt.Errorf("Trakt device authorization expired; start again")
	case http.StatusTeapot:
		return DevicePoll{}, fmt.Errorf("Trakt device authorization was denied")
	default:
		return DevicePoll{}, fmt.Errorf("Trakt device token request returned %d: %s", status, responseError(body))
	}
}

// AuthorizeDevice performs the full device OAuth flow for interactive CLIs.
func (c *Client) AuthorizeDevice(ctx context.Context, output io.Writer) error {
	code, err := c.StartDeviceAuthorization(ctx)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "\nOpen %s and enter code %s\nWaiting for authorization...\n", code.VerificationURL, code.UserCode); err != nil {
		return err
	}

	deadline := time.Now().Add(time.Duration(code.ExpiresIn) * time.Second)
	interval := time.Duration(code.Interval) * time.Second
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("Trakt device authorization expired; run the command again")
		}
		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}

		poll, err := c.PollDeviceAuthorization(ctx, code)
		if err != nil {
			return err
		}
		if poll.Authorized {
			return nil
		}
		code.Interval = poll.Interval
		interval = time.Duration(poll.Interval) * time.Second
	}
}

func (c *Client) requestDeviceToken(ctx context.Context, deviceCode string) (int, []byte, error) {
	encoded, err := json.Marshal(map[string]string{
		"client_id":     c.ClientID,
		"client_secret": c.ClientSecret,
		"code":          deviceCode,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("encode Trakt device token request: %w", err)
	}
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/oauth/device/token", bytes.NewReader(encoded))
	if err != nil {
		return 0, nil, fmt.Errorf("create Trakt device token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("trakt-api-key", c.ClientID)
	req.Header.Set("trakt-api-version", "2")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request Trakt device token: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, nil, fmt.Errorf("read Trakt device token response: %w", err)
	}
	return resp.StatusCode, body, nil
}
