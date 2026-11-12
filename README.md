# Stremio to Trakt exporter

A local Go application that migrates Stremio watch history and My List to Trakt. It includes:

- An embedded local web interface with separate migrations for Stremio Not Watched My List → Trakt personal list and watched history → Trakt History.
- A CLI for a full dry-run or full import.

The backfill uses Stremio's internal login and `datastoreGet` account endpoints, not the addon protocol. Those endpoints may change without notice. Stremio credentials are never sent to Trakt.

## Build

Go 1.23 or later is required. There are no third-party Go or JavaScript dependencies.

```sh
go build -o stremio-trakt-export .
```

## Prebuilt binaries

The GitHub Actions workflow in `.github/workflows/build.yml` tests the project and produces:

- Linux: `amd64`, `arm64`
- macOS: Intel (`amd64`), Apple Silicon (`arm64`)
- Windows: `amd64`

Every build includes a SHA-256 checksum. Pushes and pull requests upload workflow artifacts; tags beginning with `v` also create or update a GitHub Release with the binaries.

The binaries are not signed or notarized, so macOS Gatekeeper or Windows SmartScreen may display a warning.

## Local web interface

Run:

```sh
./stremio-trakt-export
```

This starts the local server and opens <http://127.0.0.1:8080> automatically. No environment variables are required. Use `--no-open` to suppress the browser launch, or `serve --addr 127.0.0.1:8080` to select a different loopback address.

Create a Trakt API application once at <https://app.trakt.tv/settings/apps/api/new>. Set its Redirect URI to `https://example.com/callback` and enter its public Client ID in the local setup form. Web mode uses PKCE, so no Client Secret is required or stored.

The workflow is:

1. Connect Stremio with email/password or an existing `authKey`.
2. Enter the Trakt Client ID and open the regular Trakt login. After authorization, copy the complete callback URL from the browser address bar and paste it into the exporter. The callback page may not load; the exporter only needs its URL. The app never sees your Trakt password.
3. Fetch the Stremio library.
4. Choose a migration view. **Not Watched → Trakt List** uses Stremio’s watched flags and episode bitfields; playback progress alone does not exclude an item. **Watched → History** lists watched movies and shows separately, including watched items removed from My List. Selecting a show includes all its watched episodes.
5. Explicitly confirm the import.

Each view imports only into its displayed destination and skips entries already present in that Trakt destination. A failed import can be retried to deduplicate any successfully written items.

The server only accepts loopback addresses. It uses per-session CSRF tokens, same-origin checks, a restrictive Content Security Policy, and in-memory sessions that expire after two hours. Do not expose this application to a network or deploy it as a shared service.

### Saved credentials

Web mode saves the Stremio `authKey`, Trakt Client ID, access token, refresh token, expiry, and Redirect URI in:

```text
~/.config/stremio-trakt-export/credentials.json
```

The directory uses mode `0700`, the file uses `0600`, and updates are atomic. Trakt refresh tokens are single-use; the app saves each replacement after refreshing. Sessions are restored after a server restart, and **Reset session** deletes the file.

These tokens are not encrypted. Anyone able to read files as your OS user could copy them and access the connected accounts. The legacy CLI does not use this credential file.

## CLI

### Dry-run

Set `STREMIO_AUTH_KEY` without placing it directly in shell history:

```sh
read -rsp "Stremio authKey: " STREMIO_AUTH_KEY; echo
export STREMIO_AUTH_KEY
./stremio-trakt-export cli
```

This creates `stremio-trakt-preview.json` with mode `0600`. It contains Trakt-shaped history and personal-list payloads, conversion issues, and statistics. Select another path with `--output`. No Trakt credentials are needed for the dry-run.

### Full import

The legacy CLI reads `TRAKT_CLIENT_ID` and `TRAKT_CLIENT_SECRET` from the environment. Set them, then run:

```sh
./stremio-trakt-export cli --commit
```

The CLI imports all resolved history and Not Watched My List items. Use the web interface for selective personal-list or History migration. Each committed CLI run requires Trakt device authorization; tokens are not cached.

## My List mapping

Stremio My List maps to a private **Trakt personal list** named `Stremio My List`. The app finds or creates that list through `/users/me/lists`, then adds items through `/users/me/lists/{id}/items`. It does not use `/sync/watchlist`.

- Only explicit, Not Watched library entries (`removed=false`, `temp=false`) are included. Watched flags and watched episode bitfields exclude an item; `lastWatched` and playback progress alone do not.
- Movies and series are supported; individual episodes are not list entries.
- Stremio `_ctime` becomes Trakt `listed_at` when available.
- Existing items in the personal list are skipped.
- Trakt account and personal-list limits still apply. Personal-list items are not automatically removed when watched.

History migration remains independent of My List membership, so removed or temporary Stremio entries can still contribute watched history.

## Data limitations

- **One play per item.** Stremio does not retain enough information to reconstruct every Trakt rewatch. The exporter imports one watched entry per movie or episode.
- **Episode dates are approximate.** Every exported episode in a show uses that show's `lastWatched` timestamp. Stremio generally does not provide an individual timestamp for every watched episode.
- **Missing dates become the import time.** If Stremio has no `lastWatched`, `watched_at` is omitted and Trakt applies its normal behavior. Review `missing_timestamps` before committing.
- **IMDb IDs are required.** Other Stremio ID schemes are reported as unresolved rather than guessed.
- **Cinemeta defines the episode mapping.** The watched-series bitfield is decoded against Cinemeta's ordered video IDs. Missing anchors or unavailable metadata are reported; the exporter does not silently mark a whole show watched.
- The JSON preview contains watch history. Delete it when no longer needed.

## Development

```sh
go test -race ./...
go vet ./...
```

Tests use mock Stremio, Cinemeta, and Trakt servers. The web flow test covers session security, preview generation, PKCE authorization without a Client Secret, separate personal-list and History imports, and logout.
