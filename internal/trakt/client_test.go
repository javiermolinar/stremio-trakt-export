package trakt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/javiermolinar/stremio-trakt-export/internal/exporter"
)

func TestExistingKeysPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" || r.Header.Get("trakt-api-key") != "client" {
			t.Errorf("missing Trakt authentication headers")
		}
		w.Header().Set("X-Pagination-Page-Count", "2")
		switch r.URL.Query().Get("page") {
		case "1":
			_, _ = w.Write([]byte(`[{"type":"movie","movie":{"ids":{"imdb":"tt1"}}}]`))
		case "2":
			_, _ = w.Write([]byte(`[{"type":"episode","show":{"ids":{"imdb":"tt2"}},"episode":{"season":3,"number":4}}]`))
		default:
			t.Errorf("unexpected page: %s", r.URL.RawQuery)
		}
	}))
	defer server.Close()
	client := Client{BaseURL: server.URL, ClientID: "client", AccessToken: "token", HTTPClient: server.Client()}

	keys, err := client.ExistingKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{exporter.MovieKey("tt1"), exporter.EpisodeKey("tt2", 3, 4)} {
		if _, ok := keys[key]; !ok {
			t.Errorf("missing key %q in %#v", key, keys)
		}
	}
}

func TestSyncHistoryBatchesMoviesAndPreservesShows(t *testing.T) {
	var mutex sync.Mutex
	var requests []exporter.History
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/sync/history" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var history exporter.History
		if err := json.NewDecoder(r.Body).Decode(&history); err != nil {
			t.Error(err)
		}
		mutex.Lock()
		requests = append(requests, history)
		mutex.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"added":{"movies":%d,"episodes":%d}}`, len(history.Movies), episodeCount(history))
	}))
	defer server.Close()
	client := Client{BaseURL: server.URL, ClientID: "client", AccessToken: "token", HTTPClient: server.Client()}
	movies := make([]exporter.Movie, 101)
	for index := range movies {
		movies[index] = exporter.Movie{IDs: exporter.IDs{IMDb: fmt.Sprintf("tt%d", index)}}
	}
	history := exporter.History{
		Movies: movies,
		Shows: []exporter.Show{{
			IDs:     exporter.IDs{IMDb: "ttshow"},
			Seasons: []exporter.Season{{Number: 1, Episodes: []exporter.Episode{{Number: 1}, {Number: 2}}}},
		}},
	}

	summary, err := client.SyncHistory(context.Background(), history)
	if err != nil {
		t.Fatal(err)
	}
	if summary.AddedMovies != 101 || summary.AddedEpisodes != 2 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	mutex.Lock()
	defer mutex.Unlock()
	if len(requests) != 3 || len(requests[0].Movies) != 100 || len(requests[1].Movies) != 1 {
		t.Fatalf("unexpected movie batches: %#v", requests)
	}
	if len(requests[2].Shows) != 1 || requests[2].Shows[0].IDs.IMDb != "ttshow" {
		t.Fatalf("show hierarchy was not preserved: %#v", requests[2])
	}
}

func TestRefreshRotatesTokensWithoutClientSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]string
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request["refresh_token"] != "old-refresh" || request["grant_type"] != "refresh_token" || request["client_id"] != "client" {
			t.Errorf("unexpected refresh request: %#v", request)
		}
		if _, present := request["client_secret"]; present {
			t.Error("PKCE refresh must not contain a client secret")
		}
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`))
	}))
	defer server.Close()
	client := Client{BaseURL: server.URL, ClientID: "client", RefreshToken: "old-refresh", HTTPClient: server.Client()}
	if err := client.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.AccessToken != "new-access" || client.RefreshToken != "new-refresh" || client.TokenExpiry.IsZero() {
		t.Fatalf("tokens were not rotated: %#v", client)
	}
}

func episodeCount(history exporter.History) int {
	count := 0
	for _, show := range history.Shows {
		for _, season := range show.Seasons {
			count += len(season.Episodes)
		}
	}
	return count
}
