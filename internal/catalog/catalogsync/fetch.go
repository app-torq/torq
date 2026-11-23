package catalogsync

import (
	"bytes"
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
	"strings"
	"time"

	"github.com/app-torq/torq/internal/catalog"
	"github.com/app-torq/torq/internal/db"
	"github.com/app-torq/torq/internal/store"
)

// Upstream locations of the openly licensed sources (Constitution VII). SV 4.2
// is never fetched: it may only be redistributed with written KBA consent.
const (
	KBAPageURL = "https://www.kba.de/DE/Statistik/Fahrzeuge/Bestand/ListeHerstellerTypSchluesselnummern/typschluesselnummern_node.html"
	GitHubAPI  = "https://api.github.com"
	GitHubRaw  = "https://raw.githubusercontent.com"
	Gor3aRepo  = "gor3a/vehicle-makes-models"
	RDWBase    = "https://opendata.rdw.nl"
)

// Download limits; the gor3a file is about 85 MB and is streamed, not held.
const (
	maxPageBytes  = 4 << 20
	maxFZ6Bytes   = 64 << 20
	maxGor3aBytes = 1 << 30
	maxRDWBytes   = 64 << 20 // one page of the RDW aggregate or fuel export
)

// RDW paging: the aggregate is read in pages of rdwPageRows; sample
// registrations are looked up rdwFuelBatch at a time (URLs stay short).
const (
	rdwPageRows  = 50000
	rdwFuelBatch = 400
)

var kentekenRe = regexp.MustCompile(`^[A-Z0-9]{1,10}$`)

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
	RDW       string
}

// NewFetcher returns a Fetcher for the public upstream sources.
func NewFetcher(userAgent string) *Fetcher {
	return &Fetcher{
		Client: &http.Client{Timeout: 10 * time.Minute}, UserAgent: userAgent,
		KBAPage: KBAPageURL, GitHubAPI: GitHubAPI, GitHubRaw: GitHubRaw, Repo: Gor3aRepo, RDW: RDWBase,
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
	rdwUpdated, err := f.RDWUpdated(ctx)
	if err != nil {
		return catalog.Result{}, err
	}
	sum := sha256.Sum256(fz6)
	hash := "online:" + hex.EncodeToString(sum[:8]) + ":" + commit + ":" + rdwUpdated
	meta, err := store.New(d.Reader).GetCatalogMeta(ctx)
	if err != nil {
		return catalog.Result{}, err
	}
	if meta.SeedSha256 == hash {
		return catalog.Result{}, nil
	}

	agg, fuel, err := f.FetchRDW(ctx)
	if err != nil {
		return catalog.Result{}, err
	}
	body, err := f.get(ctx, f.GitHubRaw+"/"+f.Repo+"/"+commit+"/data/json/all.json")
	if err != nil {
		return catalog.Result{}, err
	}
	defer func() { _ = body.Close() }()
	doc, _, err := Build(Inputs{
		FZ6: fz6, FZ6Stand: stand, Gor3a: io.LimitReader(body, maxGor3aBytes), Gor3aCommit: commit,
		RDWAggregate: bytes.NewReader(agg), RDWFuel: bytes.NewReader(fuel),
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

// RDWUpdated returns when RDW last changed the register, so an online update
// can skip unchanged sources.
func (f *Fetcher) RDWUpdated(ctx context.Context) (string, error) {
	b, err := f.read(ctx, f.RDW+"/api/views/"+RDWVehiclesDataset+".json", maxPageBytes)
	if err != nil {
		return "", err
	}
	var v struct {
		RowsUpdatedAt int64 `json:"rowsUpdatedAt"`
	}
	if err := json.Unmarshal(b, &v); err != nil || v.RowsUpdatedAt == 0 {
		return "", fmt.Errorf("catalog update: unexpected RDW dataset response: %v", err)
	}
	return strconv.FormatInt(v.RowsUpdatedAt, 10), nil
}

// FetchRDW downloads the motorcycle aggregate and the fuel and power of its
// sample registrations as CSV (research M2): about 2 aggregate pages and 50
// small lookups, never the full register.
func (f *Fetcher) FetchRDW(ctx context.Context) (aggregate, fuel []byte, err error) {
	for offset := 0; ; offset += rdwPageRows {
		page, err := f.read(ctx, f.soda(RDWVehiclesDataset, url.Values{
			"$select": {RDWAggregateSelect}, "$where": {RDWAggregateWhere}, "$group": {RDWAggregateGroup},
			"$order": {RDWAggregateGroup}, "$limit": {strconv.Itoa(rdwPageRows)}, "$offset": {strconv.Itoa(offset)},
		}), maxRDWBytes)
		if err != nil {
			return nil, nil, err
		}
		aggregate = append(aggregate, page...)
		rows, err := readRDWAggregate(bytes.NewReader(page))
		if err != nil {
			return nil, nil, err
		}
		if len(rows) < rdwPageRows {
			break
		}
		if len(aggregate) > 4*maxRDWBytes {
			return nil, nil, errors.New("catalog update: RDW aggregate unexpectedly large")
		}
	}
	groups, err := readRDWAggregate(bytes.NewReader(aggregate))
	if err != nil {
		return nil, nil, err
	}
	samples := rdwSamples(prepareMotorcycles(groups, motorcycleRules, newReport()))
	for len(samples) > 0 {
		n := min(rdwFuelBatch, len(samples))
		quoted := make([]string, 0, n)
		for _, k := range samples[:n] {
			if kentekenRe.MatchString(k) {
				quoted = append(quoted, "'"+k+"'")
			}
		}
		samples = samples[n:]
		if len(quoted) == 0 {
			continue
		}
		page, err := f.read(ctx, f.soda(RDWFuelDataset, url.Values{
			"$select": {RDWFuelSelect}, "$where": {"kenteken in(" + strings.Join(quoted, ",") + ")"},
			"$limit": {strconv.Itoa(rdwPageRows)},
		}), maxRDWBytes)
		if err != nil {
			return nil, nil, err
		}
		fuel = append(fuel, page...)
	}
	if len(fuel) == 0 {
		return nil, nil, errors.New("catalog update: no RDW fuel data")
	}
	return aggregate, fuel, nil
}

// soda is the CSV export URL of an RDW dataset with query parameters.
func (f *Fetcher) soda(dataset string, q url.Values) string {
	return f.RDW + "/resource/" + dataset + ".csv?" + q.Encode()
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
