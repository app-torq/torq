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
	"github.com/app-torq/torq/internal/auth/mfa"
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
	// Box encrypts authenticator secrets; when nil, NewServer opens the key
	// in the data directory (main opens it first to report errors).
	Box *mfa.Box
}

// level is who may call a route at all.
type level int

const (
	levelPublic level = iota
	levelMember       // any signed-in user
	levelAdmin
)

// access is what a route requires: a level and optionally a permission.
// Garage-wide permissions are checked before the handler runs; per-vehicle
// routes (vehicle true) call vehicleGuard once they know the vehicle, with
// perm "" for reads. Routes with setup true stay open to users who must set
// up a second factor first.
type access struct {
	level   level
	perm    auth.Permission
	vehicle bool
	setup   bool
	// see is the "see" permission of the area the route belongs to (#49).
	see auth.Permission
}

// seeing marks a route as part of an area a role may hide.
func (a access) seeing(p auth.Permission) access {
	a.see = p
	return a
}

var (
	public = access{level: levelPublic}
	member = access{level: levelMember}
	admin  = access{level: levelAdmin}
	// self is the caller's own account; usable while a second factor must be
	// set up.
	self = access{level: levelMember, setup: true}
)

// garage requires a garage-wide permission.
func garage(p auth.Permission) access { return access{level: levelMember, perm: p} }

// onVehicle marks a route about one vehicle's data; p is the permission a
// change needs, "" for reads. Its handler must call vehicleGuard.
func onVehicle(p auth.Permission) access { return access{level: levelMember, perm: p, vehicle: true} }

// APIVersion is the API level reported by GET /instance and stated as
// info.version in openapi.yaml. Within /api/v1 changes are additive only, and
// every release that adds operations, fields or capabilities raises it by one
// so clients can detect features (research M7).
const APIVersion = 13

// Server holds the handler dependencies, domain services and the route table.
type Server struct {
	Deps
	mux      *http.ServeMux
	sessions *auth.Sessions
	users    *auth.Users
	roles    *auth.Roles
	mfa      *mfa.Service
	password auth.PasswordAuthenticator
	// throttle blocks an IP, or a username from one IP, after a few failed
	// logins; userThrottle blocks a username everywhere only after many, so
	// a stranger can't easily lock someone out.
	throttle     *auth.Throttle
	userThrottle *auth.Throttle
	// shareThrottle slows down an IP that keeps opening invalid share links.
	shareThrottle *auth.Throttle
	// sharingThrottle slows down a user whose attempts to share a vehicle
	// keep being refused, so usernames can't be guessed in bulk (#50).
	sharingThrottle *auth.Throttle
	vehicles        *vehicles.Service
	servicelog      *servicelog.Service
	reminders       *reminders.Service
	repairs         *repairs.Service
	files           *attachments.Store
	notify          *notify.Service
	reports         *reports.Service
	// catalogUpdate runs admin-triggered online catalog updates.
	catalogUpdate *catalogUpdater
	// specfill looks up spec sheets on the web; nil unless GEMINI_API_KEY and
	// TAVILY_API_KEY are set.
	specfill specLookup
	// patterns records every registered API route and routeAccess its access
	// level (used by the CSRF route-walk and the contract tests).
	patterns    []string
	routeAccess map[string]access
}

// NewRouter builds the full HTTP handler including middleware.
func NewRouter(d Deps) http.Handler { return NewServer(d).Handler() }

// NewServer wires the domain services and routes.
func NewServer(d Deps) *Server {
	s := &Server{Deps: d, mux: http.NewServeMux()}
	s.sessions = &auth.Sessions{DB: d.DB, Clock: d.Clock}
	s.users = &auth.Users{DB: d.DB, Clock: d.Clock, Sessions: s.sessions}
	s.roles = &auth.Roles{DB: d.DB, Clock: d.Clock}
	box := d.Box
	if box == nil {
		var err error
		if box, err = mfa.OpenBox(d.Config.DataDir); err != nil {
			panic(err)
		}
	}
	passkeys, err := mfa.NewPasskeys(d.Config.BaseURL, "torq", d.Clock)
	if err != nil {
		d.Log.Error("passkeys unavailable", "err", err)
	}
	s.mfa = mfa.New(d.DB, d.Clock, box, passkeys)
	s.password = &local.Provider{DB: d.DB, Clock: d.Clock}
	s.throttle = auth.NewThrottle(d.Clock.Now, 5)
	s.userThrottle = auth.NewThrottle(d.Clock.Now, 50)
	s.shareThrottle = auth.NewThrottle(d.Clock.Now, 10)
	s.sharingThrottle = auth.NewThrottle(d.Clock.Now, 10)
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
	if s.routeAccess == nil {
		s.routeAccess = map[string]access{}
	}
	s.routeAccess[method+" /api/v1"+p] = level
	s.mux.Handle(method+" /api/v1"+p, s.wrap(level, fn))
}

func (s *Server) wrap(a access, fn handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.level != levelPublic {
			var err error
			if r, err = s.authorize(r, a); err != nil {
				writeError(w, r, s.Log, err)
				return
			}
		}
		var g *guardState
		if a.vehicle {
			g = &guardState{perm: a.perm}
			r = r.WithContext(context.WithValue(r.Context(), guardKey{}, g))
		}
		err := fn(w, r)
		if err != nil {
			writeError(w, r, s.Log, err)
		}
		if g != nil && !g.checked && err == nil {
			// Every per-vehicle handler must check the vehicle; the HTTP
			// tests fail on this message.
			s.Log.Error(skippedGuard, "route", r.Pattern)
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
