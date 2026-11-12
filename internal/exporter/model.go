package exporter

import (
	"fmt"
	"time"
)

type IDs struct {
	IMDb string `json:"imdb"`
}

type Movie struct {
	IDs       IDs        `json:"ids"`
	WatchedAt *time.Time `json:"watched_at,omitempty"`
}

type Episode struct {
	Number    int        `json:"number"`
	WatchedAt *time.Time `json:"watched_at,omitempty"`
}

type Season struct {
	Number   int       `json:"number"`
	Episodes []Episode `json:"episodes"`
}

type Show struct {
	IDs     IDs      `json:"ids"`
	Seasons []Season `json:"seasons"`
}

// History is the request body accepted by Trakt's POST /sync/history endpoint.
type History struct {
	Movies []Movie `json:"movies,omitempty"`
	Shows  []Show  `json:"shows,omitempty"`
}

// WatchlistItem represents a movie or show saved in Stremio My List.
// ListedAt maps Stremio's library creation time to Trakt's listed_at.
type WatchlistItem struct {
	IDs      IDs        `json:"ids"`
	ListedAt *time.Time `json:"listed_at,omitempty"`
}

type Watchlist struct {
	Movies []WatchlistItem `json:"movies,omitempty"`
	Shows  []WatchlistItem `json:"shows,omitempty"`
}

func (w Watchlist) WithoutExisting(existing map[string]struct{}) (Watchlist, int) {
	filtered := Watchlist{}
	skipped := 0
	for _, movie := range w.Movies {
		if _, ok := existing[MovieKey(movie.IDs.IMDb)]; ok {
			skipped++
			continue
		}
		filtered.Movies = append(filtered.Movies, movie)
	}
	for _, show := range w.Shows {
		if _, ok := existing[ShowKey(show.IDs.IMDb)]; ok {
			skipped++
			continue
		}
		filtered.Shows = append(filtered.Shows, show)
	}
	return filtered, skipped
}

type Issue struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	Reason string `json:"reason"`
}

type Stats struct {
	Movies                     int `json:"movies"`
	Episodes                   int `json:"episodes"`
	WatchlistMovies            int `json:"watchlist_movies"`
	WatchlistShows             int `json:"watchlist_shows"`
	MissingTimestamps          int `json:"missing_timestamps"`
	MissingWatchlistTimestamps int `json:"missing_watchlist_timestamps"`
	ApproximateEpisodeTime     int `json:"episodes_using_show_last_watched"`
}

type Plan struct {
	GeneratedAt time.Time `json:"generated_at"`
	DatePolicy  string    `json:"date_policy"`
	Stats       Stats     `json:"stats"`
	Movies      []Movie   `json:"movies,omitempty"`
	Shows       []Show    `json:"shows,omitempty"`
	Watchlist   Watchlist `json:"watchlist"`
	Issues      []Issue   `json:"issues,omitempty"`
}

func (p Plan) History() History {
	return History{Movies: p.Movies, Shows: p.Shows}
}

func MovieKey(imdbID string) string {
	return "movie:" + imdbID
}

func EpisodeKey(imdbID string, season, episode int) string {
	return fmt.Sprintf("episode:%s:%d:%d", imdbID, season, episode)
}

func ShowKey(imdbID string) string {
	return "show:" + imdbID
}

// WithoutExisting removes any movie or episode that already has at least one
// Trakt history entry. Stremio cannot reconstruct every individual rewatch.
func (p Plan) WithoutExisting(existing map[string]struct{}) (Plan, int) {
	filtered := p
	filtered.Movies = nil
	filtered.Shows = nil
	skipped := 0

	for _, movie := range p.Movies {
		if _, ok := existing[MovieKey(movie.IDs.IMDb)]; ok {
			skipped++
			continue
		}
		filtered.Movies = append(filtered.Movies, movie)
	}
	for _, show := range p.Shows {
		filteredShow := Show{IDs: show.IDs}
		for _, season := range show.Seasons {
			filteredSeason := Season{Number: season.Number}
			for _, episode := range season.Episodes {
				if _, ok := existing[EpisodeKey(show.IDs.IMDb, season.Number, episode.Number)]; ok {
					skipped++
					continue
				}
				filteredSeason.Episodes = append(filteredSeason.Episodes, episode)
			}
			if len(filteredSeason.Episodes) > 0 {
				filteredShow.Seasons = append(filteredShow.Seasons, filteredSeason)
			}
		}
		if len(filteredShow.Seasons) > 0 {
			filtered.Shows = append(filtered.Shows, filteredShow)
		}
	}
	filtered.RecalculateStats()
	return filtered, skipped
}

func (p *Plan) RecalculateStats() {
	p.Stats = Stats{}
	p.Stats.WatchlistMovies = len(p.Watchlist.Movies)
	p.Stats.WatchlistShows = len(p.Watchlist.Shows)
	for _, movie := range p.Watchlist.Movies {
		if movie.ListedAt == nil {
			p.Stats.MissingWatchlistTimestamps++
		}
	}
	for _, show := range p.Watchlist.Shows {
		if show.ListedAt == nil {
			p.Stats.MissingWatchlistTimestamps++
		}
	}
	for _, movie := range p.Movies {
		p.Stats.Movies++
		if movie.WatchedAt == nil {
			p.Stats.MissingTimestamps++
		}
	}
	for _, show := range p.Shows {
		for _, season := range show.Seasons {
			for _, episode := range season.Episodes {
				p.Stats.Episodes++
				if episode.WatchedAt == nil {
					p.Stats.MissingTimestamps++
				} else {
					p.Stats.ApproximateEpisodeTime++
				}
			}
		}
	}
}
