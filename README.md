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

### Run on Windows

1. Download `stremio-trakt-export-windows-amd64.zip` from the release.
2. Extract the ZIP and run `stremio-trakt-export-windows-amd64.exe`. Go does not need to be installed.
3. Leave the console open. The app starts a local server and opens <http://127.0.0.1:8080> in the browser.
4. Use Ctrl+C or close the console to stop it.

Windows currently keeps tokens in memory only, so sign in again after restarting the app. macOS and Linux use the credential file described below.

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

On macOS and Linux, web mode saves the Stremio `authKey`, Trakt Client ID, access token, refresh token, expiry, and Redirect URI in:

```text
~/.config/stremio-trakt-export/credentials.json
```

The directory uses mode `0700`, the file uses `0600`, and updates are atomic. Trakt refresh tokens are single-use; the app saves each replacement after refreshing. Sessions are restored after a server restart, and **Reset session** deletes the file.

These tokens are not encrypted. Anyone able to read files as your OS user could copy them and access the connected accounts. Windows keeps tokens in memory instead. The legacy CLI does not use this credential file.
