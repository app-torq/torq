// Package httpapi is the only package that knows about HTTP. Handlers decode
// requests, call domain packages, and render JSON per contracts/rest-api.md.
package httpapi

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/app-torq/torq/internal/attachments"
	"github.com/app-torq/torq/internal/auth"
	"github.com/app-torq/torq/internal/auth/local"
	"github.com/app-torq/torq/internal/catalog"
	"github.com/app-torq/torq/internal/catalog/catalogsync"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/config"
	"github.com/app-torq/torq/internal/db"
	"github.com/app-torq/torq/internal/notify"
	"github.com/app-torq/torq/internal/reminders"
	"github.com/app-torq/torq/internal/repairs"
	"github.com/app-torq/torq/internal/reports"
	"github.com/app-torq/torq/internal/servicelog"
	"github.com/app-torq/torq/internal/specfill"
	"github.com/app-torq/torq/internal/vehicles"
)

// Deps are the dependencies shared by all handlers.
type Deps struct {
	Config  config.Config
	DB      *db.DB
	Clock   clock.Clock
	Log     *slog.Logger
	Version string
	Dist    fs.FS
}

// access is the authorization level a route requires.
type access int

const (
	public access = iota
	member
	admin
)

// Server holds the handler dependencies, domain services and the route table.
type Server struct {
	Deps
	mux        *http.ServeMux
	sessions   *auth.Sessions
	users      *auth.Users
	password   auth.PasswordAuthenticator
	throttle   *auth.Throttle
	vehicles   *vehicles.Service
	servicelog *servicelog.Service
	reminders  *reminders.Service
	repairs    *repairs.Service
	files      *attachments.Store
	notify     *notify.Service
	reports    *reports.Service
	// catalogUpdate runs admin-triggered online catalog updates.
	catalogUpdate *catalogUpdater
	// specfill looks up spec sheets on the web; nil unless GEMINI_API_KEY and
	// TAVILY_API_KEY are set.
	specfill specLookup
	// patterns records every registered API route (used by the CSRF route-walk test).
	patterns []string
}

// NewRouter builds the full HTTP handler including middleware.
func NewRouter(d Deps) http.Handler { return NewServer(d).Handler() }

// NewServer wires the domain services and routes.
func NewServer(d Deps) *Server {
	s := &Server{Deps: d, mux: http.NewServeMux()}
	s.sessions = &auth.Sessions{DB: d.DB, Clock: d.Clock}
	s.users = &auth.Users{DB: d.DB, Clock: d.Clock, Sessions: s.sessions}
	s.password = &local.Provider{DB: d.DB, Clock: d.Clock}
	s.throttle = auth.NewThrottle(d.Clock.Now)
	s.vehicles = &vehicles.Service{DB: d.DB, Clock: d.Clock}
	s.notify = notify.New(d.DB, d.Clock, d.Config.BaseURL, d.Version, d.Log)
	s.reminders = &reminders.Service{DB: d.DB, Clock: d.Clock, Vehicles: s.vehicles, OnTransitions: s.notify.EnqueueTransitions}
	s.vehicles.AfterOdometerChange = s.reminders.EvaluateVehicle
	s.files = &attachments.Store{
		DB: d.DB, Clock: d.Clock, Dir: filepath.Join(d.Config.DataDir, "attachments"),
		MaxBytes: d.Config.MaxUploadBytes(), MaxMB: d.Config.MaxUploadMB, Log: d.Log,
	}
	s.vehicles.BeforeDelete = s.files.KeysForVehicle
	s.vehicles.AfterCommitDelete = s.files.RemoveFiles
	s.servicelog = &servicelog.Service{
		DB: d.DB, Clock: d.Clock, Vehicles: s.vehicles,
		CollectFiles: s.files.KeysForServiceLog, RemoveFiles: s.files.RemoveFiles,
	}
	s.repairs = &repairs.Service{
		DB: d.DB, Clock: d.Clock, Vehicles: s.vehicles,
		CollectFiles: s.files.KeysForRepair, RemoveFiles: s.files.RemoveFiles,
	}
	s.reports = &reports.Service{DB: d.DB, Vehicles: s.vehicles, ServiceLog: s.servicelog, Repairs: s.repairs}
	fetcher := catalogsync.NewFetcher("torq/" + d.Version + " (+https://github.com/app-torq/torq)")
	if d.Config.GeminiAPIKey != "" && d.Config.TavilyAPIKey != "" {
		s.specfill = specfill.New(d.Config.GeminiAPIKey, d.Config.TavilyAPIKey, d.Config.GeminiModel)
	}
	s.catalogUpdate = &catalogUpdater{log: d.Log, now: d.Clock.Now, run: func(ctx context.Context) (catalog.Result, error) {
		return fetcher.Update(ctx, d.DB, d.Clock.Now())
	}}

	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.Handle("/api/v1/", s.wrap(public, func(http.ResponseWriter, *http.Request) error { return errNotFound }))
	s.routes()
	s.mux.Handle("/", spaHandler(d.Dist))
	return s
}

// Handler returns the HTTP handler with middleware applied.
func (s *Server) Handler() http.Handler {
	d := s.Deps
	var h http.Handler = s.mux
	h = CSRF(d.Config.BaseURL, h)
	h = SecurityHeaders(h)
	h = RequestLog(d.Log, h)
	h = Recover(d.Log, h)
	return h
}

// Sessions exposes session maintenance for the scheduler.
func (s *Server) Sessions() *auth.Sessions { return s.sessions }

// Notify exposes the webhook worker and digests for main and the scheduler.
func (s *Server) Notify() *notify.Service { return s.notify }

// Files exposes the attachment store for the orphan sweep.
func (s *Server) Files() *attachments.Store { return s.files }

// Reminders exposes reminder evaluation for the scheduler.
func (s *Server) Reminders() *reminders.Service { return s.reminders }

// route registers an API handler under /api/v1 with the given access level.
// pattern is "METHOD /path".
func (s *Server) route(pattern string, level access, fn handlerFunc) {
	method, p, ok := strings.Cut(pattern, " ")
	if !ok {
		panic("route pattern must be \"METHOD /path\": " + pattern)
	}
	s.patterns = append(s.patterns, method+" /api/v1"+p)
	s.mux.Handle(method+" /api/v1"+p, s.wrap(level, fn))
}

func (s *Server) wrap(level access, fn handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if level != public {
			var err error
			if r, err = s.authorize(r, level); err != nil {
				writeError(w, r, s.Log, err)
				return
			}
		}
		if err := fn(w, r); err != nil {
			writeError(w, r, s.Log, err)
		}
	})
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2e9)
	defer cancel()
	var one int
	if err := s.DB.Reader.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "version": s.Version})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.Version})
}

// specLookup is the web spec lookup (specfill.Client); tests replace it.
type specLookup interface {
	Lookup(ctx context.Context, v specfill.Vehicle) (map[string]specfill.Value, error)
}
