package trakt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/javiermolinar/stremio-trakt-export/internal/exporter"
)

func TestSyncPersonalListCreatesListAndSkipsExistingItems(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/users/me/lists":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			var request map[string]any
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			if request["name"] != StremioListName || request["privacy"] != "private" {
				t.Errorf("unexpected list creation request: %#v", request)
			}
			_, _ = w.Write([]byte(`{"name":"Stremio My List","ids":{"trakt":7,"slug":"stremio-my-list"}}`))
		case "/users/me/lists/7/items":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`[{"type":"movie","movie":{"ids":{"imdb":"tt1"}}}]`))
				return
			}
			var request exporter.Watchlist
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			_, _ = fmt.Fprintf(w, `{"added":{"movies":%d,"shows":%d}}`, len(request.Movies), len(request.Shows))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := Client{BaseURL: server.URL, ClientID: "client", AccessToken: "token", HTTPClient: server.Client()}
	watchlist := exporter.Watchlist{
		Movies: []exporter.WatchlistItem{{IDs: exporter.IDs{IMDb: "tt1"}}, {IDs: exporter.IDs{IMDb: "tt2"}}},
		Shows:  []exporter.WatchlistItem{{IDs: exporter.IDs{IMDb: "tt3"}}},
	}

	summary, skipped, err := client.SyncPersonalList(context.Background(), StremioListName, watchlist)
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 1 || summary.AddedMovies != 1 || summary.AddedShows != 1 {
		t.Fatalf("unexpected result: summary=%#v skipped=%d", summary, skipped)
	}
	want := []string{"GET /users/me/lists", "POST /users/me/lists", "GET /users/me/lists/7/items", "POST /users/me/lists/7/items", "POST /users/me/lists/7/items"}
	if fmt.Sprint(paths) != fmt.Sprint(want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}
