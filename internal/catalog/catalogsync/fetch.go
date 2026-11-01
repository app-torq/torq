package catalogsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/marekbeckmann/torq/internal/catalog"
	"github.com/marekbeckmann/torq/internal/db"
	"github.com/marekbeckmann/torq/internal/store"
)

// Upstream locations of the openly licensed sources (Constitution VII). SV 4.2
// is never fetched: it may only be redistributed with written KBA consent.
const (
	KBAPageURL = "https://www.kba.de/DE/Statistik/Fahrzeuge/Bestand/ListeHerstellerTypSchluesselnummern/typschluesselnummern_node.html"
	GitHubAPI  = "https://api.github.com"
	GitHubRaw  = "https://raw.githubusercontent.com"
	Gor3aRepo  = "gor3a/vehicle-makes-models"
)

// Download limits; the gor3a file is about 85 MB and is streamed, not held.
const (
	maxPageBytes  = 4 << 20
	maxFZ6Bytes   = 64 << 20
	maxGor3aBytes = 1 << 30
)

var fz6LinkRe = regexp.MustCompile(`href="([^"]*?fz6_(\d{4})\.xlsx[^"]*)"`)

// Fetcher downloads the current sources for an admin-triggered online
// update. The fields default to the public upstream locations; tests point
// them at a local server.
type Fetcher struct {
	Client    *http.Client
	UserAgent string
	KBAPage   string
	GitHubAPI string
	GitHubRaw string
	Repo      string
}

// NewFetcher returns a Fetcher for the public upstream sources.
func NewFetcher(userAgent string) *Fetcher {
	return &Fetcher{
		Client: &http.Client{Timeout: 10 * time.Minute}, UserAgent: userAgent,
		KBAPage: KBAPageURL, GitHubAPI: GitHubAPI, GitHubRaw: GitHubRaw, Repo: Gor3aRepo,
	}
}

// Update downloads the sources, builds a catalog and imports it as the
// online catalog. It does nothing when the sources did not change since the
// last import; Result.Imported tells which happened.
func (f *Fetcher) Update(ctx context.Context, d *db.DB, now time.Time) (catalog.Result, error) {
	fz6, stand, err := f.fz6(ctx)
	if err != nil {
		return catalog.Result{}, err
	}
	commit, err := f.gor3aCommit(ctx)
	if err != nil {
		return catalog.Result{}, err
	}
	sum := sha256.Sum256(fz6)
	hash := "online:" + hex.EncodeToString(sum[:8]) + ":" + commit
	meta, err := store.New(d.Reader).GetCatalogMeta(ctx)
	if err != nil {
		return catalog.Result{}, err
	}
	if meta.SeedSha256 == hash {
		return catalog.Result{}, nil
	}

	body, err := f.get(ctx, f.GitHubRaw+"/"+f.Repo+"/"+commit+"/data/json/all.json")
	if err != nil {
		return catalog.Result{}, err
	}
	defer func() { _ = body.Close() }()
	doc, _, err := Build(Inputs{
		FZ6: fz6, FZ6Stand: stand, Gor3a: io.LimitReader(body, maxGor3aBytes), Gor3aCommit: commit,
		Retrieved: now.UTC().Format(time.DateOnly), GeneratedAt: now.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return catalog.Result{}, err
	}
	res, err := catalog.Import(ctx, d, doc, hash, catalog.SourceOnline, now)
	// The build's garbage is large; hand it back to the OS right away.
	debug.FreeOSMemory()
	return res, err
}

// fz6 finds the newest FZ 6 workbook linked from the KBA page and downloads
// it. The reference date is 1 January of the year in the file name.
func (f *Fetcher) fz6(ctx context.Context) ([]byte, string, error) {
	page, err := f.read(ctx, f.KBAPage, maxPageBytes)
	if err != nil {
		return nil, "", err
	}
	var link string
	year := 0
	for _, m := range fz6LinkRe.FindAllStringSubmatch(string(page), -1) {
		if y, _ := strconv.Atoi(m[2]); y > year {
			link, year = html.UnescapeString(m[1]), y
		}
	}
	if link == "" {
		return nil, "", errors.New("catalog update: no FZ 6 workbook link on the KBA page")
	}
	base, err := url.Parse(f.KBAPage)
	if err != nil {
		return nil, "", err
	}
	ref, err := url.Parse(link)
	if err != nil {
		return nil, "", fmt.Errorf("catalog update: FZ 6 link: %w", err)
	}
	xlsx, err := f.read(ctx, base.ResolveReference(ref).String(), maxFZ6Bytes)
	if err != nil {
		return nil, "", err
	}
	return xlsx, fmt.Sprintf("%d-01-01", year), nil
}

// gor3aCommit returns the head commit of the gor3a default branch, so the
// data file and its attribution name the same commit.
func (f *Fetcher) gor3aCommit(ctx context.Context) (string, error) {
	b, err := f.read(ctx, f.GitHubAPI+"/repos/"+f.Repo+"/commits/HEAD", 1<<20)
	if err != nil {
		return "", err
	}
	var c struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(b, &c); err != nil || len(c.SHA) < 7 {
		return "", fmt.Errorf("catalog update: unexpected GitHub commit response: %v", err)
	}
	return c.SHA, nil
}

func (f *Fetcher) read(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	body, err := f.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()
	b, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("catalog update: %s: %w", rawURL, err)
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("catalog update: %s is larger than %d bytes", rawURL, limit)
	}
	return b, nil
}

func (f *Fetcher) get(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", f.UserAgent)
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("catalog update: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("catalog update: %s: HTTP %d", rawURL, resp.StatusCode)
	}
	return resp.Body, nil
}
