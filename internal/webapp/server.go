package webapp

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/javiermolinar/stremio-trakt-export/internal/cinemeta"
	"github.com/javiermolinar/stremio-trakt-export/internal/exporter"
	"github.com/javiermolinar/stremio-trakt-export/internal/stremio"
	"github.com/javiermolinar/stremio-trakt-export/internal/trakt"
)

//go:embed static/*
var staticFiles embed.FS

const sessionLifetime = 2 * time.Hour

type Server struct {
	Stremio               *stremio.Client
	Metadata              *cinemeta.Client
	HTTPClient            *http.Client
	TraktBaseURL          string
	TraktAuthorizationURL string
	Store                 *CredentialStore

	mu       sync.Mutex
	sessions map[string]*session
}

type session struct {
	mu             sync.Mutex
	id             string
	csrf           string
	expires        time.Time
	stremioAuthKey string
	trakt          *trakt.Client
	oauth          *trakt.PKCEAuthorization
	oauthExpires   time.Time
	plan           *exporter.Plan
	items          []stremio.LibraryItem
}

func NewServer() *Server {
	return &Server{sessions: make(map[string]*session)}
}

func (s *Server) Handler() http.Handler {
	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", s.withSession(http.HandlerFunc(s.handleAPI)))
	mux.Handle("/", s.withSession(http.FileServer(http.FS(static))))
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		host = strings.Trim(host, "[]")
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			http.Error(w, "local hostname required", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, isNew, err := s.sessionForRequest(r)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if isNew {
			http.SetCookie(w, &http.Cookie{
				Name:     "stremio_export_session",
				Value:    sess.id,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
			})
		}
		next.ServeHTTP(w, r.WithContext(withRequestSession(r.Context(), sess)))
	})
}

func withRequestSession(ctx context.Context, sess *session) context.Context {
	return context.WithValue(ctx, requestSessionKey{}, sess)
}

type requestSessionKey struct{}

func requestSession(r *http.Request) *session {
	return r.Context().Value(requestSessionKey{}).(*session)
}

func (s *Server) sessionForRequest(r *http.Request) (*session, bool, error) {
	if cookie, err := r.Cookie("stremio_export_session"); err == nil && cookie.Value != "" {
		s.mu.Lock()
		sess := s.sessions[cookie.Value]
		s.mu.Unlock()
		if sess != nil {
			sess.mu.Lock()
			valid := time.Now().Before(sess.expires)
			sess.mu.Unlock()
			if valid {
				return sess, false, nil
			}
		}
	}

	id := randomToken(32)
	csrf := randomToken(32)
	sess := &session{id: id, csrf: csrf, expires: time.Now().Add(sessionLifetime)}
	if s.Store != nil {
		credentials, err := s.Store.Load()
		if err != nil {
			return nil, false, err
		}
		sess.stremioAuthKey = credentials.StremioAuthKey
		if credentials.TraktClientID != "" {
			sess.trakt = &trakt.Client{
				BaseURL:          s.TraktBaseURL,
				AuthorizationURL: s.TraktAuthorizationURL,
				ClientID:         credentials.TraktClientID,
				AccessToken:      credentials.TraktAccessToken,
				RefreshToken:     credentials.TraktRefreshToken,
				TokenExpiry:      credentials.TraktTokenExpiry,
				RedirectURI:      credentials.TraktRedirectURI,
				HTTPClient:       s.HTTPClient,
			}
		}
	}
	s.mu.Lock()
	if s.sessions == nil {
		s.sessions = make(map[string]*session)
	}
	s.sessions[id] = sess
	// Opportunistic cleanup keeps this single-user local server bounded.
	for existingID, existing := range s.sessions {
		existing.mu.Lock()
		expired := time.Now().After(existing.expires)
		existing.mu.Unlock()
		if expired {
			delete(s.sessions, existingID)
		}
	}
	s.mu.Unlock()
	return sess, true, nil
}

func randomToken(size int) string {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		panic(fmt.Sprintf("generate session token: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(buffer)
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	sess := requestSession(r)
	sess.mu.Lock()
	defer sess.mu.Unlock()

	if r.Method == http.MethodPost && !validMutationRequest(r, sess) {
		writeError(w, http.StatusForbidden, "invalid origin or CSRF token")
		return
	}

	switch r.URL.Path {
	case "/api/session":
		if !requireMethod(w, r, http.MethodGet) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"csrf":                   sess.csrf,
			"stremio_connected":      sess.stremioAuthKey != "",
			"trakt_connected":        sess.trakt != nil && (sess.trakt.AccessToken != "" || sess.trakt.RefreshToken != ""),
			"trakt_configured":       sess.trakt != nil,
			"persistent_credentials": s.Store != nil,
			"has_preview":            sess.plan != nil,
		})
	case "/api/stremio/login":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		s.loginStremio(w, r, sess)
	case "/api/preview":
		if r.Method == http.MethodPost {
			s.buildPreview(w, r, sess)
		} else if r.Method == http.MethodGet {
			s.sendPreview(w, sess)
		} else {
			w.Header().Set("Allow", "GET, POST")
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	case "/api/trakt/start":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		s.startTraktPKCE(w, r, sess)
	case "/api/trakt/exchange":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		s.exchangeTraktPKCE(w, r, sess)
	case "/api/import":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		s.importSelected(w, r, sess)
	case "/api/logout":
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		if s.Store != nil {
			if err := s.Store.Clear(); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		sess.stremioAuthKey = ""
		sess.trakt = nil
		sess.oauth = nil
		sess.plan = nil
		sess.items = nil
		sess.expires = time.Now().Add(sessionLifetime)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeError(w, http.StatusNotFound, "unknown API endpoint")
	}
}

func validMutationRequest(r *http.Request, sess *session) bool {
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host && origin != "https://"+r.Host {
		return false
	}
	token := r.Header.Get("X-CSRF-Token")
	return subtle.ConstantTimeCompare([]byte(token), []byte(sess.csrf)) == 1
}

func (s *Server) loginStremio(w http.ResponseWriter, r *http.Request, sess *session) {
	var input struct {
		AuthKey  string `json:"auth_key"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	authKey := strings.TrimSpace(input.AuthKey)
	if authKey == "" {
		if strings.TrimSpace(input.Email) == "" || input.Password == "" {
			writeError(w, http.StatusBadRequest, "provide an authKey or both email and password")
			return
		}
		var err error
		authKey, err = s.Stremio.Login(r.Context(), strings.TrimSpace(input.Email), input.Password)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
	}
	sess.stremioAuthKey = authKey
	sess.plan = nil
	sess.items = nil
	if s.Store != nil {
		if err := s.Store.SaveStremio(authKey); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"stremio_connected": true})
}

func (s *Server) buildPreview(w http.ResponseWriter, r *http.Request, sess *session) {
	if sess.stremioAuthKey == "" {
		writeError(w, http.StatusUnauthorized, "connect Stremio first")
		return
	}
	items, err := s.Stremio.Library(r.Context(), sess.stremioAuthKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	builder := exporter.Builder{Metadata: s.Metadata, Concurrency: 4}
	plan, err := builder.Build(r.Context(), items)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	sess.plan = &plan
	sess.items = items
	writePreview(w, plan, items)
}

func (s *Server) sendPreview(w http.ResponseWriter, sess *session) {
	if sess.plan == nil {
		writeError(w, http.StatusNotFound, "no preview loaded")
		return
	}
	writePreview(w, *sess.plan, sess.items)
}

type mediaItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Watchlist    bool   `json:"watchlist,omitempty"`
	WatchedCount int    `json:"watched_count,omitempty"`
}

type previewResponse struct {
	GeneratedAt  time.Time        `json:"generated_at"`
	Stats        exporter.Stats   `json:"stats"`
	Items        []mediaItem      `json:"items"`
	HistoryItems []mediaItem      `json:"history_items"`
	Issues       []exporter.Issue `json:"issues,omitempty"`
}

func writePreview(w http.ResponseWriter, plan exporter.Plan, items []stremio.LibraryItem) {
	historyMovies := make(map[string]bool)
	historyShows := make(map[string]int)
	watchlistMovies := make(map[string]bool)
	watchlistShows := make(map[string]bool)
	for _, movie := range plan.Movies {
		historyMovies[movie.IDs.IMDb] = true
	}
	for _, show := range plan.Shows {
		for _, season := range show.Seasons {
			historyShows[show.IDs.IMDb] += len(season.Episodes)
		}
	}
	for _, movie := range plan.Watchlist.Movies {
		watchlistMovies[movie.IDs.IMDb] = true
	}
	for _, show := range plan.Watchlist.Shows {
		watchlistShows[show.IDs.IMDb] = true
	}

	response := previewResponse{GeneratedAt: plan.GeneratedAt, Stats: plan.Stats, Issues: plan.Issues}
	seen := make(map[string]bool)
	historySeen := make(map[string]bool)
	for _, item := range items {
		if seen[item.Type+":"+item.ID] {
			continue
		}
		entry := mediaItem{ID: item.ID, Name: item.Name, Type: item.Type}
		if item.Type == "movie" {
			entry.Watchlist = watchlistMovies[item.ID]
		} else if item.Type == "series" {
			entry.Watchlist = watchlistShows[item.ID]
		}
		if entry.Watchlist {
			if entry.Name == "" {
				entry.Name = entry.ID
			}
			response.Items = append(response.Items, entry)
			seen[item.Type+":"+item.ID] = true
		}

		historyEntry := mediaItem{ID: item.ID, Name: item.Name, Type: item.Type}
		if item.Type == "movie" && historyMovies[item.ID] {
			historyEntry.WatchedCount = 1
		} else if item.Type == "series" && historyShows[item.ID] > 0 {
			historyEntry.WatchedCount = historyShows[item.ID]
		}
		if historyEntry.WatchedCount > 0 && !historySeen[item.Type+":"+item.ID] {
			if historyEntry.Name == "" {
				historyEntry.Name = historyEntry.ID
			}
			response.HistoryItems = append(response.HistoryItems, historyEntry)
			historySeen[item.Type+":"+item.ID] = true
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) startTraktPKCE(w http.ResponseWriter, r *http.Request, sess *session) {
	var input struct {
		ClientID    string `json:"client_id"`
		RedirectURI string `json:"redirect_uri"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	clientID := strings.TrimSpace(input.ClientID)
	if clientID != "" {
		sess.trakt = &trakt.Client{
			BaseURL:          s.TraktBaseURL,
			AuthorizationURL: s.TraktAuthorizationURL,
			ClientID:         clientID,
			HTTPClient:       s.HTTPClient,
		}
		sess.oauth = nil
	}
	if sess.trakt == nil {
		writeError(w, http.StatusBadRequest, "enter your Trakt Client ID")
		return
	}
	redirectURI := strings.TrimSpace(input.RedirectURI)
	if redirectURI == "" {
		redirectURI = "https://example.com/callback"
	}
	authorization, err := sess.trakt.StartPKCE(redirectURI)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sess.oauth = &authorization
	sess.oauthExpires = time.Now().Add(10 * time.Minute)
	writeJSON(w, http.StatusOK, map[string]string{
		"authorization_url": authorization.AuthorizationURL,
		"redirect_uri":      authorization.RedirectURI,
	})
}

func (s *Server) exchangeTraktPKCE(w http.ResponseWriter, r *http.Request, sess *session) {
	if sess.oauth == nil || sess.trakt == nil {
		writeError(w, http.StatusConflict, "start Trakt authorization first")
		return
	}
	if time.Now().After(sess.oauthExpires) {
		sess.oauth = nil
		writeError(w, http.StatusGone, "Trakt authorization expired; start again")
		return
	}
	var input struct {
		CallbackURL string `json:"callback_url"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := sess.trakt.ExchangePKCE(r.Context(), *sess.oauth, input.CallbackURL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sess.oauth = nil
	if s.Store != nil {
		if err := s.Store.SaveTrakt(sess.trakt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"trakt_connected": true})
}

type selection struct {
	HistoryMovies   []string `json:"history_movies"`
	HistoryShows    []string `json:"history_shows"`
	WatchlistMovies []string `json:"watchlist_movies"`
	WatchlistShows  []string `json:"watchlist_shows"`
}

type importResponse struct {
	History        trakt.SyncSummary `json:"history"`
	List           trakt.ListSummary `json:"list"`
	HistorySkipped int               `json:"history_skipped"`
	ListSkipped    int               `json:"list_skipped"`
}

func (s *Server) ensureTraktToken(r *http.Request, sess *session) error {
	if sess.trakt.AccessToken != "" && (sess.trakt.TokenExpiry.IsZero() || time.Until(sess.trakt.TokenExpiry) > time.Minute) {
		return nil
	}
	if sess.trakt.RefreshToken == "" {
		return fmt.Errorf("Trakt authorization expired; connect Trakt again")
	}
	if err := sess.trakt.Refresh(r.Context()); err != nil {
		sess.trakt = nil
		if s.Store != nil {
			_ = s.Store.ClearTrakt()
		}
		return fmt.Errorf("Trakt authorization could not be refreshed; connect Trakt again: %w", err)
	}
	if s.Store != nil {
		if err := s.Store.SaveTrakt(sess.trakt); err != nil {
			return fmt.Errorf("save refreshed Trakt token: %w", err)
		}
	}
	return nil
}

func (s *Server) importSelected(w http.ResponseWriter, r *http.Request, sess *session) {
	if sess.plan == nil {
		writeError(w, http.StatusConflict, "load a preview first")
		return
	}
	if sess.trakt == nil || (sess.trakt.AccessToken == "" && sess.trakt.RefreshToken == "") {
		writeError(w, http.StatusUnauthorized, "connect Trakt first")
		return
	}
	if err := s.ensureTraktToken(r, sess); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var selected selection
	if err := decodeJSON(r, &selected); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	plan := selectPlan(*sess.plan, selected)
	if plan.Stats.Movies+plan.Stats.Episodes+plan.Stats.WatchlistMovies+plan.Stats.WatchlistShows == 0 {
		writeError(w, http.StatusBadRequest, "select at least one item")
		return
	}

	var response importResponse
	if plan.Stats.Movies+plan.Stats.Episodes > 0 {
		existing, err := sess.trakt.ExistingKeys(r.Context())
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		pending, skipped := plan.WithoutExisting(existing)
		response.HistorySkipped = skipped
		if pending.Stats.Movies+pending.Stats.Episodes > 0 {
			response.History, err = sess.trakt.SyncHistory(r.Context(), pending.History())
			if err != nil {
				writeError(w, http.StatusBadGateway, "history import stopped after a possible partial run; retry to deduplicate: "+err.Error())
				return
			}
		}
	}
	if plan.Stats.WatchlistMovies+plan.Stats.WatchlistShows > 0 {
		summary, skipped, err := sess.trakt.SyncPersonalList(r.Context(), trakt.StremioListName, plan.Watchlist)
		response.List = summary
		response.ListSkipped = skipped
		if err != nil {
			writeError(w, http.StatusBadGateway, "personal list import stopped after a possible partial run; retry to deduplicate: "+err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func selectPlan(source exporter.Plan, selected selection) exporter.Plan {
	result := exporter.Plan{GeneratedAt: source.GeneratedAt, DatePolicy: source.DatePolicy, Issues: source.Issues}
	historyMovies := stringSet(selected.HistoryMovies)
	historyShows := stringSet(selected.HistoryShows)
	watchlistMovies := stringSet(selected.WatchlistMovies)
	watchlistShows := stringSet(selected.WatchlistShows)
	for _, movie := range source.Movies {
		if historyMovies[movie.IDs.IMDb] {
			result.Movies = append(result.Movies, movie)
		}
	}
	for _, show := range source.Shows {
		if historyShows[show.IDs.IMDb] {
			result.Shows = append(result.Shows, show)
		}
	}
	for _, movie := range source.Watchlist.Movies {
		if watchlistMovies[movie.IDs.IMDb] {
			result.Watchlist.Movies = append(result.Watchlist.Movies, movie)
		}
	}
	for _, show := range source.Watchlist.Shows {
		if watchlistShows[show.IDs.IMDb] {
			result.Watchlist.Shows = append(result.Watchlist.Shows, show)
		}
	}
	result.RecalculateStats()
	return result
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	return false
}

func decodeJSON(r *http.Request, destination any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("invalid JSON request: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("request must contain one JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
