// Command torq is the self-hosted vehicle maintenance and DIY repair tracker.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // lets TZ work on distroless images without zoneinfo

	"github.com/marekbeckmann/torq/internal/clock"
	"github.com/marekbeckmann/torq/internal/config"
	"github.com/marekbeckmann/torq/internal/db"
	"github.com/marekbeckmann/torq/internal/httpapi"
	"github.com/marekbeckmann/torq/internal/scheduler"
	"github.com/marekbeckmann/torq/web"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "healthcheck":
			os.Exit(healthcheck())
		case "version":
			fmt.Println(version)
			return
		case "seed-demo":
			if err := seedDemo(os.Args[2:]); err != nil {
				fmt.Fprintln(os.Stderr, "torq seed-demo:", err)
				os.Exit(1)
			}
			return
		}
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "torq:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(nil)
	if err != nil {
		return err
	}
	log := newLogger(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	database, err := db.Open(cfg.DataDir)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()
	if err := db.Migrate(ctx, database, cfg.DataDir); err != nil {
		return err
	}

	server := httpapi.NewServer(httpapi.Deps{
		Config:  cfg,
		DB:      database,
		Clock:   clock.System{},
		Log:     log,
		Version: version,
		Dist:    web.Dist,
	})
	handler := server.Handler()

	sched := &scheduler.Scheduler{
		Interval: 15 * time.Minute,
		Log:      log,
		Tasks: []scheduler.Task{
			{Name: "evaluate-reminders", Run: server.Reminders().EvaluateAll},
			{Name: "enqueue-digests", Run: server.Notify().EnqueueDigests},
			{Name: "prune-deliveries", Run: server.Notify().Prune},
			{Name: "sweep-orphan-files", Run: func(ctx context.Context) error {
				n, err := server.Files().SweepOrphans(ctx, time.Hour)
				if n > 0 {
					log.Info("removed orphaned attachment files", "count", n)
				}
				return err
			}},
			{Name: "prune-sessions", Run: func(ctx context.Context) error {
				n, err := server.Sessions().PruneExpired(ctx)
				if n > 0 {
					log.Info("pruned expired sessions", "count", n)
				}
				return err
			}},
		},
	}
	go sched.Run(ctx)
	go server.Notify().Run(ctx)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("torq listening", "addr", cfg.ListenAddr, "version", version, "data_dir", cfg.DataDir)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
	}
	return nil
}

func newLogger(cfg config.Config) *slog.Logger {
	var level slog.Level
	_ = level.UnmarshalText([]byte(cfg.LogLevel))
	opts := &slog.HandlerOptions{Level: level}
	if cfg.LogFormat == "text" {
		return slog.New(slog.NewTextHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, opts))
}

// healthcheck GETs /healthz on the local listener; used by the container HEALTHCHECK.
func healthcheck() int {
	cfg, err := config.Load(nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_, port, err := net.SplitHostPort(cfg.ListenAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	client := http.Client{Timeout: 3 * time.Second}
	res, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "unhealthy:", res.Status)
		return 1
	}
	return 0
}
