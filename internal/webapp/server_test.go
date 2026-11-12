package webapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/javiermolinar/stremio-trakt-export/internal/cinemeta"
	"github.com/javiermolinar/stremio-trakt-export/internal/exporter"
	"github.com/javiermolinar/stremio-trakt-export/internal/stremio"
)

type testBrowser struct {
	t      *testing.T
	base   string
	client *http.Client
	cookie *http.Cookie
	csrf   string
}

func (b *testBrowser) request(method, path, body string) (int, []byte) {
	b.t.Helper()
	req, err := http.NewRequest(method, b.base+path, strings.NewReader(body))
	if err != nil {
		b.t.Fatal(err)
	}
	if b.cookie != nil {
		req.AddCookie(b.cookie)
	}
	if method != http.MethodGet {
		req.Header.Set("X-CSRF-Token", b.csrf)
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	defer resp.Body.Close()
	var buffer bytes.Buffer
	_, _ = buffer.ReadFrom(resp.Body)
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "stremio_export_session" {
			b.cookie = cookie
		}
	}
	return resp.StatusCode, buffer.Bytes()
}

func TestWebMigrationFlow(t *testing.T) {
	created := "2023-01-02T00:00:00Z"
	stremioServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/datastoreGet" {
			t.Errorf("unexpected Stremio path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": []stremio.LibraryItem{
			{
				ID: "tt1", Name: "Movie", Type: "movie", CTime: &created,
				State: stremio.State{TimesWatched: 0},
			},
			{ID: "tt2", Name: "Watched Movie", Type: "movie", State: stremio.State{TimesWatched: 1}},
		}})
	}))
	defer stremioServer.Close()
	metadataServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected Cinemeta request: %s", r.URL.Path)
	}))
	defer metadataServer.Close()

	var mutex sync.Mutex
	var importedPaths []string
	traktServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			var request map[string]string
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request["code"] != "auth-code" || request["code_verifier"] == "" || request["client_id"] != "client" {
				t.Errorf("unexpected PKCE token request: %#v", request)
			}
			if _, present := request["client_secret"]; present {
				t.Errorf("PKCE request must not contain a client secret")
			}
			_, _ = w.Write([]byte(`{"access_token":"token","refresh_token":"refresh","expires_in":3600}`))
		case "/users/me/lists":
			_, _ = w.Write([]byte(`[{"name":"Stremio My List","ids":{"trakt":42,"slug":"stremio-my-list"}}]`))
		case "/users/me/lists/42/items":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			mutex.Lock()
			importedPaths = append(importedPaths, r.URL.Path)
			mutex.Unlock()
			_, _ = w.Write([]byte(`{"added":{"movies":1,"shows":0}}`))
		case "/sync/history":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			mutex.Lock()
			importedPaths = append(importedPaths, r.URL.Path)
			mutex.Unlock()
			_, _ = w.Write([]byte(`{"added":{"movies":1,"episodes":0}}`))
		default:
			t.Errorf("unexpected Trakt path %s", r.URL.Path)
		}
	}))
	defer traktServer.Close()

	application := NewServer()
	application.Stremio = &stremio.Client{APIURL: stremioServer.URL + "/api", HTTPClient: stremioServer.Client()}
	application.Metadata = &cinemeta.Client{BaseURL: metadataServer.URL, HTTPClient: metadataServer.Client()}
	application.HTTPClient = traktServer.Client()
	application.TraktBaseURL = traktServer.URL
	application.TraktAuthorizationURL = traktServer.URL + "/oauth/authorize"
	credentialPath := filepath.Join(t.TempDir(), "credentials.json")
	application.Store = NewCredentialStore(credentialPath)
	server := httptest.NewServer(application.Handler())
	defer server.Close()
	browser := &testBrowser{t: t, base: server.URL, client: server.Client()}

	status, body := browser.request(http.MethodGet, "/", "")
	if status != http.StatusOK || !bytes.Contains(body, []byte("Stremio")) {
		t.Fatalf("static page returned %d: %s", status, body)
	}
	status, body = browser.request(http.MethodGet, "/api/session", "")
	if status != http.StatusOK {
		t.Fatalf("session returned %d: %s", status, body)
	}
	var sessionResponse struct {
		CSRF string `json:"csrf"`
	}
	_ = json.Unmarshal(body, &sessionResponse)
	browser.csrf = sessionResponse.CSRF
	if browser.csrf == "" || browser.cookie == nil {
		t.Fatal("session did not initialize cookie and CSRF token")
	}

	status, _ = browser.request(http.MethodPost, "/api/stremio/login", `{"auth_key":"secret"}`)
	if status != http.StatusOK {
		t.Fatalf("Stremio login returned %d", status)
	}
	status, body = browser.request(http.MethodPost, "/api/preview", `{}`)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"name":"Movie"`)) || !bytes.Contains(body, []byte(`"watchlist_movies":1`)) || !bytes.Contains(body, []byte(`"name":"Watched Movie"`)) {
		t.Fatalf("preview returned %d: %s", status, body)
	}

	status, body = browser.request(http.MethodPost, "/api/trakt/start", `{"client_id":"client","redirect_uri":"https://example.com/callback"}`)
	if status != http.StatusOK {
		t.Fatalf("PKCE start returned %d: %s", status, body)
	}
	var authorization struct {
		AuthorizationURL string `json:"authorization_url"`
	}
	if err := json.Unmarshal(body, &authorization); err != nil {
		t.Fatal(err)
	}
	parsedAuthorization, err := url.Parse(authorization.AuthorizationURL)
	if err != nil || parsedAuthorization.Query().Get("code_challenge") == "" {
		t.Fatalf("invalid authorization URL %q", authorization.AuthorizationURL)
	}
	callback := "https://example.com/callback?code=auth-code&state=" + url.QueryEscape(parsedAuthorization.Query().Get("state"))
	status, body = browser.request(http.MethodPost, "/api/trakt/exchange", `{"callback_url":`+strconv.Quote(callback)+`}`)
	if status != http.StatusOK {
		t.Fatalf("PKCE exchange returned %d: %s", status, body)
	}
	status, body = browser.request(http.MethodPost, "/api/import", `{"watchlist_movies":["tt1"]}`)
	if status != http.StatusOK {
		t.Fatalf("import returned %d: %s", status, body)
	}
	var result importResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if result.List.AddedMovies != 1 {
		t.Fatalf("unexpected import result: %#v", result)
	}
	status, body = browser.request(http.MethodPost, "/api/import", `{"history_movies":["tt2"]}`)
	if status != http.StatusOK {
		t.Fatalf("history import returned %d: %s", status, body)
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if result.History.AddedMovies != 1 {
		t.Fatalf("unexpected history import result: %#v", result)
	}
	mutex.Lock()
	defer mutex.Unlock()
	if fmt.Sprint(importedPaths) != "[/users/me/lists/42/items /sync/history]" {
		t.Fatalf("unexpected imported paths: %v", importedPaths)
	}

	status, _ = browser.request(http.MethodPost, "/api/logout", `{}`)
	if status != http.StatusOK {
		t.Fatalf("logout returned %d", status)
	}
	status, body = browser.request(http.MethodGet, "/api/session", "")
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"stremio_connected":false`)) || !bytes.Contains(body, []byte(`"trakt_connected":false`)) {
		t.Fatalf("session was not cleared: %d %s", status, body)
	}
	if _, err := os.Stat(credentialPath); !os.IsNotExist(err) {
		t.Fatalf("credentials were not deleted: %v", err)
	}
}

func TestMutationRequiresCSRF(t *testing.T) {
	application := NewServer()
	server := httptest.NewServer(application.Handler())
	defer server.Close()
	browser := &testBrowser{t: t, base: server.URL, client: server.Client()}
	status, _ := browser.request(http.MethodGet, "/", "")
	if status != http.StatusOK {
		t.Fatal(status)
	}
	browser.csrf = "wrong"
	status, _ = browser.request(http.MethodPost, "/api/stremio/login", `{"auth_key":"secret"}`)
	if status != http.StatusForbidden {
		t.Fatalf("mutation without valid CSRF returned %d", status)
	}
}

func TestSelectPlan(t *testing.T) {
	source := exporter.Plan{
		Movies: []exporter.Movie{{IDs: exporter.IDs{IMDb: "tt1"}}, {IDs: exporter.IDs{IMDb: "tt2"}}},
		Watchlist: exporter.Watchlist{
			Movies: []exporter.WatchlistItem{{IDs: exporter.IDs{IMDb: "tt1"}}},
			Shows:  []exporter.WatchlistItem{{IDs: exporter.IDs{IMDb: "tt3"}}},
		},
	}
	selected := selectPlan(source, selection{WatchlistShows: []string{"tt3"}})
	if len(selected.Movies) != 0 || len(selected.Watchlist.Movies) != 0 || len(selected.Watchlist.Shows) != 1 {
		t.Fatalf("unexpected selection: %#v", selected)
	}
	history := selectPlan(source, selection{HistoryMovies: []string{"tt2"}})
	if len(history.Movies) != 1 || history.Movies[0].IDs.IMDb != "tt2" || len(history.Watchlist.Shows) != 0 {
		t.Fatalf("unexpected history selection: %#v", history)
	}
}
