package catalogsync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/app-torq/torq/internal/catalog"
	"github.com/app-torq/torq/internal/store"
	"github.com/app-torq/torq/internal/testutil"
)

// upstream fakes the KBA page and GitHub with the synthetic fixtures.
func upstream(t *testing.T, fail *bool) *Fetcher {
	t.Helper()
	workbook := xlsx(t, fz6Rows)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /kba/page.html", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<a href="/dl/fz6_2023.xlsx?__blob=publicationFile&amp;v=3">2023</a>
			<a href="/dl/fz6_2024.xlsx?__blob=publicationFile&amp;v=2">2024</a>`))
	})
	mux.HandleFunc("GET /dl/fz6_2024.xlsx", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("v") != "2" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(workbook)
	})
	mux.HandleFunc("GET /api/repos/gor3a/vehicle-makes-models/commits/HEAD", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "torq/test" {
			http.Error(w, "user agent required", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"sha": "abcdef0123456789"}`))
	})
	mux.HandleFunc("GET /raw/gor3a/vehicle-makes-models/abcdef0123456789/data/json/all.json", func(w http.ResponseWriter, _ *http.Request) {
		if *fail {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(gor3aFixture))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	f := NewFetcher("torq/test")
	f.KBAPage, f.GitHubAPI, f.GitHubRaw = srv.URL+"/kba/page.html", srv.URL+"/api", srv.URL+"/raw"
	return f
}

func TestFetcherUpdate(t *testing.T) {
	ctx := context.Background()
	d := testutil.NewDB(t)
	fail := true
	f := upstream(t, &fail)
	now := time.Date(2026, 11, 2, 9, 30, 0, 0, time.UTC)

	if _, err := f.Update(ctx, d, now); err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("failed download: %v", err)
	}
	if m, _ := store.New(d.Reader).GetCatalogMeta(ctx); m.SeedSha256 != "" {
		t.Fatalf("failed update changed the catalog: %+v", m)
	}

	fail = false
	res, err := f.Update(ctx, d, now)
	if err != nil || !res.Imported || res.KBA == 0 || res.Vehicles == 0 {
		t.Fatalf("update %+v %v", res, err)
	}
	meta, err := catalog.LoadMeta(ctx, store.New(d.Reader))
	if err != nil {
		t.Fatal(err)
	}
	if meta.Source != catalog.SourceOnline || meta.Header.GeneratedAt != "2026-11-02T09:30:00Z" ||
		meta.Header.Datasets[0].Stand != "2024-01-01" || meta.Header.Datasets[0].Retrieved != "2026-11-02" ||
		meta.Header.Datasets[1].Commit != "abcdef0123456789" {
		t.Fatalf("meta %+v", meta)
	}

	// Unchanged sources: nothing to import.
	if res, err := f.Update(ctx, d, now.Add(time.Hour)); err != nil || res.Imported {
		t.Fatalf("second update %+v %v", res, err)
	}
}
