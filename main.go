package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/javiermolinar/stremio-trakt-export/internal/cinemeta"
	"github.com/javiermolinar/stremio-trakt-export/internal/exporter"
	"github.com/javiermolinar/stremio-trakt-export/internal/stremio"
	"github.com/javiermolinar/stremio-trakt-export/internal/trakt"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var err error
	switch {
	case len(os.Args) == 1:
		err = runServe(ctx, nil)
	case os.Args[1] == "serve":
		err = runServe(ctx, os.Args[2:])
	case os.Args[1] == "--no-open" || os.Args[1] == "--addr":
		err = runServe(ctx, os.Args[1:])
	case os.Args[1] == "cli":
		err = run(ctx, os.Args[2:])
	default:
		err = run(ctx, os.Args[1:])
	}
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("stremio-trakt-export", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	commit := flags.Bool("commit", false, "write missing items to Trakt (default is dry-run)")
	output := flags.String("output", "stremio-trakt-preview.json", "path for the local JSON preview")
	concurrency := flags.Int("concurrency", 4, "maximum concurrent Cinemeta metadata requests")
	timeout := flags.Duration("http-timeout", 30*time.Second, "timeout for each HTTP request")
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: %s cli [--output file] [--commit]\n       %s [serve] [--addr 127.0.0.1:8080]\n\n", flags.Name(), flags.Name())
		fmt.Fprintln(flags.Output(), "Environment:")
		fmt.Fprintln(flags.Output(), "  STREMIO_AUTH_KEY       Stremio account authKey (required)")
		fmt.Fprintln(flags.Output(), "  TRAKT_CLIENT_ID        Trakt API application client ID (required with --commit)")
		fmt.Fprintln(flags.Output(), "  TRAKT_CLIENT_SECRET    Trakt API application client secret (required with --commit)")
		fmt.Fprintln(flags.Output(), "\nOptions:")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *concurrency < 1 {
		return fmt.Errorf("--concurrency must be at least 1")
	}
	if *timeout <= 0 {
		return fmt.Errorf("--http-timeout must be greater than zero")
	}
	authKey := strings.TrimSpace(os.Getenv("STREMIO_AUTH_KEY"))
	if authKey == "" {
		flags.Usage()
		return fmt.Errorf("STREMIO_AUTH_KEY is required")
	}

	httpClient := &http.Client{Timeout: *timeout}
	stremioClient := &stremio.Client{HTTPClient: httpClient}
	metadataClient := &cinemeta.Client{HTTPClient: httpClient}
	builder := &exporter.Builder{Metadata: metadataClient, Concurrency: *concurrency}

	fmt.Fprintln(os.Stderr, "Reading Stremio library...")
	items, err := stremioClient.Library(ctx, authKey)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Building export from %d library items...\n", len(items))
	plan, err := builder.Build(ctx, items)
	if err != nil {
		return err
	}
	if err := writePreview(*output, plan); err != nil {
		return err
	}
	printPlan(plan, *output)
	if !*commit {
		fmt.Fprintln(os.Stderr, "Dry-run only. Set --commit to authorize Trakt and import missing items.")
		return nil
	}
	hasHistory := plan.Stats.Movies+plan.Stats.Episodes > 0
	hasWatchlist := plan.Stats.WatchlistMovies+plan.Stats.WatchlistShows > 0
	if !hasHistory && !hasWatchlist {
		fmt.Fprintln(os.Stderr, "Nothing to import.")
		return nil
	}

	clientID := strings.TrimSpace(os.Getenv("TRAKT_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("TRAKT_CLIENT_SECRET"))
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("TRAKT_CLIENT_ID and TRAKT_CLIENT_SECRET are required with --commit")
	}
	traktClient := &trakt.Client{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HTTPClient:   httpClient,
	}
	if err := traktClient.AuthorizeDevice(ctx, os.Stderr); err != nil {
		return err
	}
	if hasHistory {
		fmt.Fprintln(os.Stderr, "Reading existing Trakt history...")
		existing, err := traktClient.ExistingKeys(ctx)
		if err != nil {
			return err
		}
		pending, skipped := plan.WithoutExisting(existing)
		fmt.Fprintf(os.Stderr, "Skipping %d items already present in Trakt history.\n", skipped)
		if pending.Stats.Movies+pending.Stats.Episodes > 0 {
			fmt.Fprintf(os.Stderr, "Importing %d movies and %d episodes...\n", pending.Stats.Movies, pending.Stats.Episodes)
			summary, err := traktClient.SyncHistory(ctx, pending.History())
			if err != nil {
				return fmt.Errorf("history import stopped after a partial run: %w", err)
			}
			fmt.Fprintf(os.Stderr, "History import complete: added %d movies and %d episodes; existing %d movies and %d episodes; not found %d.\n",
				summary.AddedMovies, summary.AddedEpisodes, summary.ExistingMovies, summary.ExistingEpisodes, summary.NotFound)
		}
	}
	if hasWatchlist {
		fmt.Fprintf(os.Stderr, "Importing My List into Trakt personal list %q...\n", trakt.StremioListName)
		summary, skipped, err := traktClient.SyncPersonalList(ctx, trakt.StremioListName, plan.Watchlist)
		if err != nil {
			return fmt.Errorf("personal list import stopped after a partial run: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Skipping %d items already present in the personal list.\n", skipped)
		fmt.Fprintf(os.Stderr, "Personal list import complete: added %d movies and %d shows; existing %d movies and %d shows; not found %d.\n",
			summary.AddedMovies, summary.AddedShows, summary.ExistingMovies, summary.ExistingShows, summary.NotFound)
	}
	return nil
}

func writePreview(path string, plan exporter.Plan) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("--output cannot be empty")
	}
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".stremio-trakt-preview-*")
	if err != nil {
		return fmt.Errorf("create preview file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("secure preview file: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(plan); err != nil {
		temporary.Close()
		return fmt.Errorf("write preview: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync preview: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close preview: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace preview: %w", err)
	}
	return nil
}

func printPlan(plan exporter.Plan, output string) {
	fmt.Fprintf(os.Stderr, "Preview: %s\n", output)
	fmt.Fprintf(os.Stderr, "Found %d watched movies, %d watched episodes, and %d My List items (%d movies, %d shows); %d history items lack timestamps.\n",
		plan.Stats.Movies, plan.Stats.Episodes, plan.Stats.WatchlistMovies+plan.Stats.WatchlistShows,
		plan.Stats.WatchlistMovies, plan.Stats.WatchlistShows, plan.Stats.MissingTimestamps)
	if plan.Stats.ApproximateEpisodeTime > 0 {
		fmt.Fprintf(os.Stderr, "%d episodes will use their show's lastWatched timestamp (approximate).\n", plan.Stats.ApproximateEpisodeTime)
	}
	if len(plan.Issues) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "%d issues need review:\n", len(plan.Issues))
	const displayLimit = 10
	for i, issue := range plan.Issues {
		if i == displayLimit {
			fmt.Fprintf(os.Stderr, "  ... %d more in the preview file\n", len(plan.Issues)-displayLimit)
			break
		}
		name := issue.Name
		if name == "" {
			name = issue.ID
		}
		fmt.Fprintf(os.Stderr, "  - %s: %s\n", name, issue.Reason)
	}
}
