package exporter

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/javiermolinar/stremio-trakt-export/internal/cinemeta"
	"github.com/javiermolinar/stremio-trakt-export/internal/stremio"
)

type fakeMetadata struct {
	series map[string]cinemeta.Series
	errs   map[string]error
}

func (f fakeMetadata) Series(_ context.Context, imdbID string) (cinemeta.Series, error) {
	if err := f.errs[imdbID]; err != nil {
		return cinemeta.Series{}, err
	}
	return f.series[imdbID], nil
}

func stringPointer(value string) *string { return &value }

func watchedFixture(t *testing.T, anchor string, values ...bool) string {
	t.Helper()
	packed := make([]byte, (len(values)+7)/8)
	for index, value := range values {
		if value {
			packed[index/8] |= 1 << uint(index%8)
		}
	}
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	_, _ = writer.Write(packed)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%s:%d:%s", anchor, len(values), base64.StdEncoding.EncodeToString(compressed.Bytes()))
}

func TestBuilderBuild(t *testing.T) {
	lastWatched := "2024-02-03T04:05:06Z"
	metadata := fakeMetadata{series: map[string]cinemeta.Series{
		"tt100": {
			ID: "tt100",
			Videos: []cinemeta.Video{
				{ID: "tt100:1:1", Season: 1, Episode: 1},
				{ID: "tt100:1:2", Season: 1, Episode: 2},
				{ID: "tt100:2:1", Season: 2, Episode: 1},
			},
		},
	}}
	items := []stremio.LibraryItem{
		{ID: "tt200", Name: "Watched movie", Type: "movie", State: stremio.State{TimesWatched: 1, LastWatched: &lastWatched}},
		{ID: "tt201", Name: "In progress", Type: "movie", State: stremio.State{TimesWatched: 0}},
		{ID: "custom:movie", Name: "Unknown movie", Type: "movie", State: stremio.State{FlaggedWatched: 1}},
		{ID: "tt100", Name: "Watched show", Type: "series", State: stremio.State{
			LastWatched: &lastWatched,
			Watched:     stringPointer(watchedFixture(t, "tt100:2:1", true, false, true)),
		}},
	}
	fixedTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	builder := Builder{Metadata: metadata, Concurrency: 2, Now: func() time.Time { return fixedTime }}

	plan, err := builder.Build(context.Background(), items)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Stats.Movies != 1 || plan.Stats.Episodes != 2 || len(plan.Issues) != 1 {
		t.Fatalf("unexpected plan stats/issues: %#v", plan)
	}
	if plan.Stats.WatchlistMovies != 1 || plan.Stats.WatchlistShows != 0 || plan.Stats.MissingWatchlistTimestamps != 1 {
		t.Fatalf("unexpected watchlist stats: %#v", plan.Stats)
	}
	if len(plan.Movies) != 1 || plan.Movies[0].IDs.IMDb != "tt200" {
		t.Fatalf("unexpected movies: %#v", plan.Movies)
	}
	if len(plan.Shows) != 1 || len(plan.Shows[0].Seasons) != 2 {
		t.Fatalf("unexpected shows: %#v", plan.Shows)
	}
	if got := plan.Shows[0].Seasons[0].Episodes[0].WatchedAt; got == nil || got.Format(time.RFC3339) != lastWatched {
		t.Fatalf("unexpected episode timestamp: %v", got)
	}
	if !plan.GeneratedAt.Equal(fixedTime) {
		t.Fatalf("GeneratedAt = %v, want %v", plan.GeneratedAt, fixedTime)
	}
}

func TestBuilderWatchlistMatchesStremioNotWatched(t *testing.T) {
	lastWatched := "2024-02-03T04:05:06Z"
	items := []stremio.LibraryItem{
		{ID: "tt1", Type: "movie"},
		{ID: "tt2", Type: "movie", State: stremio.State{TimesWatched: 1}},
		{ID: "tt3", Type: "movie", State: stremio.State{TimeOffset: 1}},
		{ID: "tt4", Type: "series"},
		{ID: "tt5", Type: "series", State: stremio.State{LastWatched: &lastWatched}},
		{ID: "tt6", Type: "movie", Removed: true},
		{ID: "tt7", Type: "movie", Temp: true},
	}
	builder := Builder{Metadata: fakeMetadata{}}
	plan, err := builder.Build(context.Background(), items)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Watchlist.Movies) != 2 || plan.Watchlist.Movies[0].IDs.IMDb != "tt1" || plan.Watchlist.Movies[1].IDs.IMDb != "tt3" {
		t.Fatalf("unexpected watchlist movies: %#v", plan.Watchlist.Movies)
	}
	if len(plan.Watchlist.Shows) != 2 || plan.Watchlist.Shows[0].IDs.IMDb != "tt4" || plan.Watchlist.Shows[1].IDs.IMDb != "tt5" {
		t.Fatalf("unexpected watchlist shows: %#v", plan.Watchlist.Shows)
	}
}

func TestBuilderReportsMetadataFailure(t *testing.T) {
	metadata := fakeMetadata{errs: map[string]error{"tt100": fmt.Errorf("metadata unavailable")}}
	builder := Builder{Metadata: metadata}
	plan, err := builder.Build(context.Background(), []stremio.LibraryItem{{
		ID: "tt100", Type: "series", State: stremio.State{Watched: stringPointer("bitfield")},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Issues) != 1 || plan.Issues[0].Reason != "metadata unavailable" {
		t.Fatalf("unexpected issues: %#v", plan.Issues)
	}
}

func TestWithoutExisting(t *testing.T) {
	plan := Plan{
		Movies: []Movie{{IDs: IDs{IMDb: "tt1"}}, {IDs: IDs{IMDb: "tt2"}}},
		Shows: []Show{{
			IDs:     IDs{IMDb: "tt3"},
			Seasons: []Season{{Number: 1, Episodes: []Episode{{Number: 1}, {Number: 2}}}},
		}},
	}
	plan.RecalculateStats()
	existing := map[string]struct{}{MovieKey("tt1"): {}, EpisodeKey("tt3", 1, 1): {}}

	filtered, skipped := plan.WithoutExisting(existing)
	if skipped != 2 {
		t.Fatalf("skipped = %d, want 2", skipped)
	}
	if filtered.Stats.Movies != 1 || filtered.Stats.Episodes != 1 {
		t.Fatalf("unexpected filtered stats: %#v", filtered.Stats)
	}
	got := []string{filtered.Movies[0].IDs.IMDb, fmt.Sprint(filtered.Shows[0].Seasons[0].Episodes[0].Number)}
	want := []string{"tt2", "2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("remaining items = %v, want %v", got, want)
	}
}
