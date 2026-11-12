package trakt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/javiermolinar/stremio-trakt-export/internal/exporter"
)

const StremioListName = "Stremio My List"

type PersonalList struct {
	Name string `json:"name"`
	IDs  struct {
		Trakt int    `json:"trakt"`
		Slug  string `json:"slug"`
	} `json:"ids"`
}

func (c *Client) findOrCreateList(ctx context.Context, name string) (PersonalList, error) {
	var lists []PersonalList
	if _, err := c.doJSON(ctx, http.MethodGet, "/users/me/lists", nil, &lists, true); err != nil {
		return PersonalList{}, err
	}
	for _, list := range lists {
		if list.Name == name && list.IDs.Trakt != 0 {
			return list, nil
		}
	}
	var created PersonalList
	if _, err := c.doJSON(ctx, http.MethodPost, "/users/me/lists", map[string]any{
		"name":            name,
		"description":     "Never-watched items exported from Stremio My List.",
		"privacy":         "private",
		"display_numbers": false,
		"allow_comments":  false,
	}, &created, true); err != nil {
		return PersonalList{}, err
	}
	if created.IDs.Trakt == 0 {
		return PersonalList{}, fmt.Errorf("Trakt returned no ID for the new personal list")
	}
	return created, nil
}

func (c *Client) existingListKeys(ctx context.Context, listID int) (map[string]struct{}, error) {
	keys := make(map[string]struct{})
	const pageSize = 100
	for page := 1; ; page++ {
		var entries []historyEntry
		path := fmt.Sprintf("/users/me/lists/%d/items?page=%d&limit=%d", listID, page, pageSize)
		headers, err := c.doJSON(ctx, http.MethodGet, path, nil, &entries, true)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			switch entry.Type {
			case "movie":
				if entry.Movie != nil && entry.Movie.IDs.IMDb != "" {
					keys[exporter.MovieKey(entry.Movie.IDs.IMDb)] = struct{}{}
				}
			case "show":
				if entry.Show != nil && entry.Show.IDs.IMDb != "" {
					keys[exporter.ShowKey(entry.Show.IDs.IMDb)] = struct{}{}
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

type ListSummary struct {
	AddedMovies    int `json:"added_movies"`
	AddedShows     int `json:"added_shows"`
	ExistingMovies int `json:"existing_movies"`
	ExistingShows  int `json:"existing_shows"`
	NotFound       int `json:"not_found"`
}

type listResponse struct {
	Added struct {
		Movies int `json:"movies"`
		Shows  int `json:"shows"`
	} `json:"added"`
	Existing struct {
		Movies int `json:"movies"`
		Shows  int `json:"shows"`
	} `json:"existing"`
	NotFound struct {
		Movies []json.RawMessage `json:"movies"`
		Shows  []json.RawMessage `json:"shows"`
	} `json:"not_found"`
}

// SyncPersonalList finds or creates a private list, skips its existing items,
// and adds the remaining movies/shows. It does not use the Trakt Watchlist.
func (c *Client) SyncPersonalList(ctx context.Context, name string, watchlist exporter.Watchlist) (ListSummary, int, error) {
	var total ListSummary
	list, err := c.findOrCreateList(ctx, name)
	if err != nil {
		return total, 0, err
	}
	existing, err := c.existingListKeys(ctx, list.IDs.Trakt)
	if err != nil {
		return total, 0, err
	}
	pending, skipped := watchlist.WithoutExisting(existing)
	const batchSize = 100
	for start := 0; start < len(pending.Movies); start += batchSize {
		end := min(start+batchSize, len(pending.Movies))
		summary, err := c.syncListBatch(ctx, list.IDs.Trakt, exporter.Watchlist{Movies: pending.Movies[start:end]})
		if err != nil {
			return total, skipped, err
		}
		total.add(summary)
	}
	for start := 0; start < len(pending.Shows); start += batchSize {
		end := min(start+batchSize, len(pending.Shows))
		summary, err := c.syncListBatch(ctx, list.IDs.Trakt, exporter.Watchlist{Shows: pending.Shows[start:end]})
		if err != nil {
			return total, skipped, err
		}
		total.add(summary)
	}
	return total, skipped, nil
}

func (s *ListSummary) add(other ListSummary) {
	s.AddedMovies += other.AddedMovies
	s.AddedShows += other.AddedShows
	s.ExistingMovies += other.ExistingMovies
	s.ExistingShows += other.ExistingShows
	s.NotFound += other.NotFound
}

func (c *Client) syncListBatch(ctx context.Context, listID int, watchlist exporter.Watchlist) (ListSummary, error) {
	var response listResponse
	path := fmt.Sprintf("/users/me/lists/%d/items", listID)
	if _, err := c.doJSON(ctx, http.MethodPost, path, watchlist, &response, true); err != nil {
		return ListSummary{}, err
	}
	return ListSummary{
		AddedMovies:    response.Added.Movies,
		AddedShows:     response.Added.Shows,
		ExistingMovies: response.Existing.Movies,
		ExistingShows:  response.Existing.Shows,
		NotFound:       len(response.NotFound.Movies) + len(response.NotFound.Shows),
	}, nil
}
