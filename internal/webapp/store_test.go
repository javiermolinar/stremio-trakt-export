package webapp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/javiermolinar/stremio-trakt-export/internal/trakt"
)

func TestCredentialStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "credentials.json")
	store := NewCredentialStore(path)
	if err := store.SaveStremio("stremio-key"); err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	client := &trakt.Client{
		ClientID:     "client",
		AccessToken:  "access",
		RefreshToken: "refresh",
		TokenExpiry:  expiry,
		RedirectURI:  "https://example.com/callback",
	}
	if err := store.SaveTrakt(client); err != nil {
		t.Fatal(err)
	}
	credentials, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if credentials.StremioAuthKey != "stremio-key" || credentials.TraktAccessToken != "access" || credentials.TraktRefreshToken != "refresh" || !credentials.TraktTokenExpiry.Equal(expiry) {
		t.Fatalf("unexpected credentials: %#v", credentials)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("credential permissions = %o, want 600", info.Mode().Perm())
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("credential file still exists: %v", err)
	}
}
