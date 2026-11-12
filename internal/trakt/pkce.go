package trakt

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const DefaultAuthorizationURL = "https://auth.trakt.tv/oauth/authorize"

type PKCEAuthorization struct {
	RedirectURI      string
	CodeVerifier     string
	State            string
	AuthorizationURL string
}

func (c *Client) StartPKCE(redirectURI string) (PKCEAuthorization, error) {
	parsedRedirect, err := url.Parse(redirectURI)
	if err != nil || parsedRedirect.Scheme != "https" || parsedRedirect.Host == "" || parsedRedirect.RawQuery != "" || parsedRedirect.Fragment != "" {
		return PKCEAuthorization{}, fmt.Errorf("redirect URI must be an absolute HTTPS URL without query parameters or a fragment")
	}
	verifier, err := randomURLToken(32)
	if err != nil {
		return PKCEAuthorization{}, fmt.Errorf("generate PKCE verifier: %w", err)
	}
	state, err := randomURLToken(32)
	if err != nil {
		return PKCEAuthorization{}, fmt.Errorf("generate OAuth state: %w", err)
	}
	challengeBytes := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeBytes[:])
	authorizationEndpoint := c.AuthorizationURL
	if authorizationEndpoint == "" {
		authorizationEndpoint = DefaultAuthorizationURL
	}
	authorizationURL, err := url.Parse(authorizationEndpoint)
	if err != nil {
		return PKCEAuthorization{}, fmt.Errorf("parse Trakt authorization URL: %w", err)
	}
	query := authorizationURL.Query()
	query.Set("response_type", "code")
	query.Set("client_id", c.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("state", state)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	authorizationURL.RawQuery = query.Encode()
	return PKCEAuthorization{
		RedirectURI:      redirectURI,
		CodeVerifier:     verifier,
		State:            state,
		AuthorizationURL: authorizationURL.String(),
	}, nil
}

func (c *Client) ExchangePKCE(ctx context.Context, authorization PKCEAuthorization, callbackURL string) error {
	callback, err := url.Parse(strings.TrimSpace(callbackURL))
	if err != nil || callback.Scheme == "" || callback.Host == "" {
		return fmt.Errorf("paste the complete callback URL from the browser address bar")
	}
	registered, err := url.Parse(authorization.RedirectURI)
	if err != nil {
		return fmt.Errorf("invalid stored redirect URI: %w", err)
	}
	if !strings.EqualFold(callback.Scheme, registered.Scheme) || !strings.EqualFold(callback.Host, registered.Host) || callback.Path != registered.Path {
		return fmt.Errorf("callback URL does not match the registered redirect URI")
	}
	query := callback.Query()
	if oauthError := query.Get("error"); oauthError != "" {
		description := query.Get("error_description")
		if description == "" {
			description = oauthError
		}
		return fmt.Errorf("Trakt authorization failed: %s", description)
	}
	if subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(authorization.State)) != 1 {
		return fmt.Errorf("OAuth state does not match; restart Trakt authorization")
	}
	code := query.Get("code")
	if code == "" {
		return fmt.Errorf("callback URL contains no authorization code")
	}
	if authorization.CodeVerifier == "" {
		return fmt.Errorf("PKCE verifier is missing; restart Trakt authorization")
	}

	var token tokenResponse
	if _, err := c.doJSON(ctx, http.MethodPost, "/oauth/token", map[string]string{
		"code":          code,
		"client_id":     c.ClientID,
		"redirect_uri":  authorization.RedirectURI,
		"grant_type":    "authorization_code",
		"code_verifier": authorization.CodeVerifier,
	}, &token, false); err != nil {
		return err
	}
	if err := c.applyToken(token); err != nil {
		return err
	}
	c.RedirectURI = authorization.RedirectURI
	return nil
}

func randomURLToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
