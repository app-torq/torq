package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/catalog"
)

// catalogUpdateTimeout bounds one online update (download, build, import).
const catalogUpdateTimeout = 15 * time.Minute

// catalogUpdateJSON is the state of the online catalog update shown on the
// About page. State is "idle", "running", "succeeded", "unchanged" or
// "failed"; Error carries the failure for admins to act on.
type catalogUpdateJSON struct {
	Enabled    bool    `json:"enabled"`
	State      string  `json:"state"`
	StartedAt  *string `json:"started_at"`
	FinishedAt *string `json:"finished_at"`
	Error      *string `json:"error"`
}

// catalogUpdater runs at most one online catalog update at a time, in the
// background, because downloading and building the catalog takes minutes.
type catalogUpdater struct {
	log *slog.Logger
	now func() time.Time
	run func(context.Context) (catalog.Result, error)

	mu    sync.Mutex
	state catalogUpdateJSON
	done  chan struct{} // closed when the running update finishes; for tests
}

func (u *catalogUpdater) status(enabled bool) catalogUpdateJSON {
	u.mu.Lock()
	defer u.mu.Unlock()
	st := u.state
	st.Enabled = enabled
	if st.State == "" {
		st.State = "idle"
	}
	return st
}

// start begins an update unless one is running.
func (u *catalogUpdater) start() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.state.State == "running" {
		return false
	}
	started := u.now().UTC().Format(time.RFC3339)
	u.state = catalogUpdateJSON{State: "running", StartedAt: &started}
	u.done = make(chan struct{})
	go u.work(u.done)
	return true
}

func (u *catalogUpdater) work(done chan struct{}) {
	defer close(done)
	ctx, cancel := context.WithTimeout(context.Background(), catalogUpdateTimeout)
	defer cancel()
	res, err := u.run(ctx)
	finished := u.now().UTC().Format(time.RFC3339)
	u.mu.Lock()
	defer u.mu.Unlock()
	u.state.FinishedAt = &finished
	switch {
	case err != nil:
		msg := err.Error()
		u.state.State, u.state.Error = "failed", &msg
		u.log.Error("catalog update failed; keeping the previous catalog", "err", err)
	case res.Imported:
		u.state.State = "succeeded"
		u.log.Info("catalog updated online", "kba", res.KBA, "vehicles", res.Vehicles, "duration", res.Duration.Round(time.Millisecond))
	default:
		u.state.State = "unchanged"
		u.log.Info("catalog update: sources unchanged")
	}
}

// startCatalogUpdate starts an online update of the vehicle catalog
// (Constitution VII). Lookups keep using the current catalog until the new
// one is imported in a single transaction.
func (s *Server) startCatalogUpdate(w http.ResponseWriter, _ *http.Request) error {
	if !s.Config.CatalogOnlineUpdates {
		return apperr.Forbidden("catalog.updates_disabled")
	}
	if !s.catalogUpdate.start() {
		return apperr.Conflict("catalog.update_running", nil)
	}
	writeJSON(w, http.StatusAccepted, map[string]catalogUpdateJSON{"update": s.catalogUpdate.status(true)})
	return nil
}
