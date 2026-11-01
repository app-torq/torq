package main

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/marekbeckmann/torq/internal/catalog"
	"github.com/marekbeckmann/torq/internal/db"
)

// importCatalog loads the embedded vehicle catalog when it changed. A failure
// is logged and startup continues: the catalog only speeds up data entry and
// must never block torq (FR-018, research C6).
func importCatalog(ctx context.Context, log *slog.Logger, database *db.DB) {
	res, err := catalog.Seed(ctx, database, catalog.EmbeddedSeed, time.Now())
	switch {
	case err != nil:
		log.Error("catalog import failed; continuing with the previous catalog", "err", err)
	case res.Imported:
		// The decoded seed is garbage now; hand its memory back to the OS so
		// the first start after an upgrade stays within the footprint budget.
		debug.FreeOSMemory()
		log.Info("catalog imported", "kba", res.KBA, "vehicles", res.Vehicles, "duration", res.Duration.Round(time.Millisecond))
	default:
		log.Info("catalog up to date")
	}
}
