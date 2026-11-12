package exporter

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/javiermolinar/stremio-trakt-export/internal/cinemeta"
	"github.com/javiermolinar/stremio-trakt-export/internal/stremio"
)

type MetadataClient interface {
	Series(ctx context.Context, imdbID string) (cinemeta.Series, error)
}

type Builder struct {
	Metadata    MetadataClient
	Concurrency int
	Now         func() time.Time
}

type seriesCandidate struct {
	item stremio.LibraryItem
}

type seriesResult struct {
	metadata cinemeta.Series
	err      error
}

func (b *Builder) Build(ctx context.Context, items []stremio.LibraryItem) (Plan, error) {
	if b.Metadata == nil {
		return Plan{}, fmt.Errorf("Cinemeta metadata client is required")
	}
	now := time.Now
	if b.Now != nil {
		now = b.Now
	}
	plan := Plan{
		GeneratedAt: now().UTC(),
		DatePolicy:  "history uses Stremio lastWatched (shared by every episode in a show); watchlist listed_at uses library _ctime; missing dates are omitted",
	}
	var candidates []seriesCandidate

	for _, item := range items {
		// Stremio's My List contains explicit library items. Temporary entries
		// (for example, Continue Watching suggestions) and removed entries do
		// not belong in the Trakt Watchlist.
		if !item.Removed && !item.Temp && !isMarkedWatched(item) && (item.Type == "movie" || item.Type == "series") {
			if !validIMDbID(item.ID) {
				plan.Issues = append(plan.Issues, Issue{ID: item.ID, Name: item.Name, Reason: "My List item has no usable IMDb ID"})
			} else {
				listedAt, dateErr := parseTimestamp(item.CTime, "_ctime")
				if dateErr != nil {
					plan.Issues = append(plan.Issues, Issue{ID: item.ID, Name: item.Name, Reason: dateErr.Error()})
				}
				watchlistItem := WatchlistItem{IDs: IDs{IMDb: item.ID}, ListedAt: listedAt}
				if item.Type == "movie" {
					plan.Watchlist.Movies = append(plan.Watchlist.Movies, watchlistItem)
				} else {
					plan.Watchlist.Shows = append(plan.Watchlist.Shows, watchlistItem)
				}
			}
		}

		switch item.Type {
		case "movie":
			if item.State.TimesWatched == 0 && item.State.FlaggedWatched == 0 {
				continue
			}
			if !validIMDbID(item.ID) {
				plan.Issues = append(plan.Issues, Issue{ID: item.ID, Name: item.Name, Reason: "watched movie has no usable IMDb ID"})
				continue
			}
			watchedAt, dateErr := parseTimestamp(item.State.LastWatched, "lastWatched")
			if dateErr != nil {
				plan.Issues = append(plan.Issues, Issue{ID: item.ID, Name: item.Name, Reason: dateErr.Error()})
			}
			plan.Movies = append(plan.Movies, Movie{IDs: IDs{IMDb: item.ID}, WatchedAt: watchedAt})
		case "series":
			hasBitfield := item.State.Watched != nil && *item.State.Watched != ""
			if !hasBitfield {
				if item.State.FlaggedWatched > 0 || item.State.TimesWatched > 0 {
					plan.Issues = append(plan.Issues, Issue{ID: item.ID, Name: item.Name, Reason: "series has watched flags but no episode bitfield; refusing to mark all episodes watched"})
				}
				continue
			}
			if !validIMDbID(item.ID) {
				plan.Issues = append(plan.Issues, Issue{ID: item.ID, Name: item.Name, Reason: "watched series has no usable IMDb ID"})
				continue
			}
			candidates = append(candidates, seriesCandidate{item: item})
		}
	}

	results := b.fetchSeries(ctx, candidates)
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	for i, candidate := range candidates {
		item := candidate.item
		result := results[i]
		if result.err != nil {
			plan.Issues = append(plan.Issues, Issue{ID: item.ID, Name: item.Name, Reason: result.err.Error()})
			continue
		}

		videoIDs := make([]string, len(result.metadata.Videos))
		for j, video := range result.metadata.Videos {
			videoIDs[j] = video.ID
		}
		watched, err := stremio.DecodeWatchedVideos(*item.State.Watched, videoIDs)
		if err != nil {
			plan.Issues = append(plan.Issues, Issue{ID: item.ID, Name: item.Name, Reason: err.Error()})
			continue
		}
		watchedAt, dateErr := parseTimestamp(item.State.LastWatched, "lastWatched")
		if dateErr != nil {
			plan.Issues = append(plan.Issues, Issue{ID: item.ID, Name: item.Name, Reason: dateErr.Error()})
		}

		seasons := make(map[int][]Episode)
		for _, video := range result.metadata.Videos {
			if !watched[video.ID] {
				continue
			}
			season, episode := video.Season, video.Episode
			if season == 0 && episode == 0 {
				season, episode, err = parseVideoID(video.ID)
				if err != nil {
					plan.Issues = append(plan.Issues, Issue{ID: item.ID, Name: item.Name, Reason: err.Error()})
					continue
				}
			}
			seasons[season] = append(seasons[season], Episode{Number: episode, WatchedAt: watchedAt})
		}
		show := Show{IDs: IDs{IMDb: item.ID}}
		seasonNumbers := make([]int, 0, len(seasons))
		for season := range seasons {
			seasonNumbers = append(seasonNumbers, season)
		}
		sort.Ints(seasonNumbers)
		for _, season := range seasonNumbers {
			episodes := seasons[season]
			sort.Slice(episodes, func(i, j int) bool { return episodes[i].Number < episodes[j].Number })
			show.Seasons = append(show.Seasons, Season{Number: season, Episodes: episodes})
		}
		if len(show.Seasons) > 0 {
			plan.Shows = append(plan.Shows, show)
		}
	}

	sort.Slice(plan.Movies, func(i, j int) bool { return plan.Movies[i].IDs.IMDb < plan.Movies[j].IDs.IMDb })
	sort.Slice(plan.Shows, func(i, j int) bool { return plan.Shows[i].IDs.IMDb < plan.Shows[j].IDs.IMDb })
	sort.Slice(plan.Watchlist.Movies, func(i, j int) bool { return plan.Watchlist.Movies[i].IDs.IMDb < plan.Watchlist.Movies[j].IDs.IMDb })
	sort.Slice(plan.Watchlist.Shows, func(i, j int) bool { return plan.Watchlist.Shows[i].IDs.IMDb < plan.Watchlist.Shows[j].IDs.IMDb })
	sort.Slice(plan.Issues, func(i, j int) bool {
		if plan.Issues[i].ID == plan.Issues[j].ID {
			return plan.Issues[i].Reason < plan.Issues[j].Reason
		}
		return plan.Issues[i].ID < plan.Issues[j].ID
	})
	plan.RecalculateStats()
	return plan, nil
}

func (b *Builder) fetchSeries(ctx context.Context, candidates []seriesCandidate) []seriesResult {
	results := make([]seriesResult, len(candidates))
	if len(candidates) == 0 {
		return results
	}
	concurrency := b.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	if concurrency > len(candidates) {
		concurrency = len(candidates)
	}

	jobs := make(chan int)
	var workers sync.WaitGroup
	workers.Add(concurrency)
	for worker := 0; worker < concurrency; worker++ {
		go func() {
			defer workers.Done()
			for index := range jobs {
				metadata, err := b.Metadata.Series(ctx, candidates[index].item.ID)
				results[index] = seriesResult{metadata: metadata, err: err}
			}
		}()
	}
	for index := range candidates {
		select {
		case jobs <- index:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return results
		}
	}
	close(jobs)
	workers.Wait()
	return results
}

func isMarkedWatched(item stremio.LibraryItem) bool {
	// Stremio's Not Watched library view is based on watched flags and the
	// episode bitfield. lastWatched and playback progress can be populated on
	// entries that Stremio still classifies as Not Watched.
	return item.State.TimesWatched > 0 || item.State.FlaggedWatched > 0 ||
		(item.State.Watched != nil && strings.TrimSpace(*item.State.Watched) != "")
}

func validIMDbID(id string) bool {
	if len(id) < 3 || !strings.HasPrefix(id, "tt") {
		return false
	}
	for _, character := range id[2:] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func parseTimestamp(value *string, field string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, *value)
	if err != nil {
		return nil, fmt.Errorf("invalid %s timestamp %q; date will be omitted", field, *value)
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func parseVideoID(id string) (int, int, error) {
	parts := strings.Split(id, ":")
	if len(parts) < 3 {
		return 0, 0, fmt.Errorf("cannot determine season and episode from video ID %q", id)
	}
	season, seasonErr := strconv.Atoi(parts[len(parts)-2])
	episode, episodeErr := strconv.Atoi(parts[len(parts)-1])
	if seasonErr != nil || episodeErr != nil {
		return 0, 0, fmt.Errorf("cannot determine season and episode from video ID %q", id)
	}
	return season, episode, nil
}
