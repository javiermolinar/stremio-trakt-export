package webapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/javiermolinar/stremio-trakt-export/internal/trakt"
)

type storedCredentials struct {
	StremioAuthKey    string    `json:"stremio_auth_key,omitempty"`
	TraktClientID     string    `json:"trakt_client_id,omitempty"`
	TraktAccessToken  string    `json:"trakt_access_token,omitempty"`
	TraktRefreshToken string    `json:"trakt_refresh_token,omitempty"`
	TraktTokenExpiry  time.Time `json:"trakt_token_expiry,omitempty"`
	TraktRedirectURI  string    `json:"trakt_redirect_uri,omitempty"`
}

// CredentialStore persists account tokens for this single-user local app.
// Its file permissions restrict access but do not encrypt the tokens.
type CredentialStore struct {
	path string
	mu   sync.Mutex
}

func NewCredentialStore(path string) *CredentialStore {
	if path == "" {
		homeDirectory, err := os.UserHomeDir()
		if err != nil {
			homeDirectory = "."
		}
		path = filepath.Join(homeDirectory, ".config", "stremio-trakt-export", "credentials.json")
	}
	return &CredentialStore{path: path}
}

func (s *CredentialStore) Path() string { return s.path }

func (s *CredentialStore) loadLocked() (storedCredentials, error) {
	var credentials storedCredentials
	body, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return credentials, nil
	}
	if err != nil {
		return credentials, fmt.Errorf("read saved credentials: %w", err)
	}
	if err := json.Unmarshal(body, &credentials); err != nil {
		return credentials, fmt.Errorf("decode saved credentials: %w", err)
	}
	return credentials, nil
}

func (s *CredentialStore) Load() (storedCredentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *CredentialStore) SaveStremio(authKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	credentials, err := s.loadLocked()
	if err != nil {
		return err
	}
	credentials.StremioAuthKey = authKey
	return s.saveLocked(credentials)
}

func (s *CredentialStore) SaveTrakt(client *trakt.Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	credentials, err := s.loadLocked()
	if err != nil {
		return err
	}
	credentials.TraktClientID = client.ClientID
	credentials.TraktAccessToken = client.AccessToken
	credentials.TraktRefreshToken = client.RefreshToken
	credentials.TraktTokenExpiry = client.TokenExpiry
	credentials.TraktRedirectURI = client.RedirectURI
	return s.saveLocked(credentials)
}

func (s *CredentialStore) ClearTrakt() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	credentials, err := s.loadLocked()
	if err != nil {
		return err
	}
	credentials.TraktClientID = ""
	credentials.TraktAccessToken = ""
	credentials.TraktRefreshToken = ""
	credentials.TraktTokenExpiry = time.Time{}
	credentials.TraktRedirectURI = ""
	return s.saveLocked(credentials)
}

func (s *CredentialStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete saved credentials: %w", err)
	}
	return nil
}

func (s *CredentialStore) saveLocked(credentials storedCredentials) error {
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create credential directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return fmt.Errorf("secure credential directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".credentials-*")
	if err != nil {
		return fmt.Errorf("create credential file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("secure credential file: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(credentials); err != nil {
		temporary.Close()
		return fmt.Errorf("write credentials: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync credentials: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close credentials: %w", err)
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("replace credentials: %w", err)
	}
	return nil
}
