package catalog

import (
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"time"

	"github.com/marekbeckmann/torq/internal/db"
	"github.com/marekbeckmann/torq/internal/store"
)

// SeedSchema identifies the seed document format (data-model §5). v2 added
// German names, chassis-code generations and body tokens.
const SeedSchema = "torq.catalog-seed/v2"

// Catalog sources recorded in catalog_meta.source.
const (
	SourceBundled = "bundled" // the seed embedded in this release
	SourceOnline  = "online"  // fetched by an admin-triggered update
)

// FuelTypes are the allowed fuel_type values (same enum as vehicles.fuel_type).
var FuelTypes = []string{"diesel", "petrol", "hybrid", "electric", "lpg", "cng", "other"}

// Bodies are the allowed body tokens of a vehicle row; the frontend
// translates them (vehicles.json "body.*").
var Bodies = []string{"sedan", "hatchback", "estate", "coupe", "cabriolet", "roadster", "sportback"}

var (
	hsnRe = regexp.MustCompile(`^[0-9]{4}$`)
	tsnRe = regexp.MustCompile(`^[0-9A-Z]{3}$`)
)

// Dataset is the attribution and provenance of one bundled source. The
// attribution text is kept verbatim as the license requires (research C13).
type Dataset struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Publisher   string `json:"publisher"`
	URI         string `json:"uri"`
	Stand       string `json:"stand,omitempty"`
	Retrieved   string `json:"retrieved"`
	Commit      string `json:"commit,omitempty"`
	License     string `json:"license"`
	LicenseID   string `json:"license_id"`
	LicenseURL  string `json:"license_url"`
	Attribution string `json:"attribution"`
	Modified    bool   `json:"modified"`
}

// Counts are the row counts of a seed.
type Counts struct {
	KBA      int `json:"kba"`
	Vehicles int `json:"vehicles"`
}

// Header is the seed metadata stored in catalog_meta.header_json.
type Header struct {
	Schema      string    `json:"schema"`
	GeneratedAt string    `json:"generated_at"`
	License     string    `json:"license"`
	Datasets    []Dataset `json:"datasets"`
	Counts      Counts    `json:"counts"`
}

// KBARow is one HSN/TSN variant.
type KBARow struct {
	HSN            string  `json:"hsn"`
	TSN            string  `json:"tsn"`
	Make           string  `json:"make"`
	Model          string  `json:"model"`
	ModelDE        string  `json:"model_de,omitempty"` // empty when equal to Model
	TradeName      string  `json:"trade_name"`
	Generation     string  `json:"generation,omitempty"`
	EngineCode     string  `json:"engine_code,omitempty"`
	DisplacementCC *int64  `json:"displacement_cc,omitempty"`
	PowerKW        *int64  `json:"power_kw,omitempty"`
	FuelType       *string `json:"fuel_type,omitempty"`
	BodyCode       string  `json:"body_code,omitempty"`
	YearFrom       *int64  `json:"year_from,omitempty"`
	Source         string  `json:"source"`
	Match          string  `json:"match"`
}

// VehicleRow is one engine variant of the make/model/generation taxonomy.
// Model and Generation are English; the *DE fields are empty when the German
// name is the same. Generation is a chassis code ("W203") where one is known.
type VehicleRow struct {
	Make           string  `json:"make"`
	Model          string  `json:"model"`
	ModelDE        string  `json:"model_de,omitempty"`
	Generation     string  `json:"generation"`
	GenerationDE   string  `json:"generation_de,omitempty"`
	YearFrom       *int64  `json:"year_from,omitempty"`
	YearTo         *int64  `json:"year_to,omitempty"`
	Body           string  `json:"body,omitempty"` // one of Bodies
	Engine         string  `json:"engine"`
	EngineCode     string  `json:"engine_code,omitempty"`
	DisplacementCC *int64  `json:"displacement_cc,omitempty"`
	PowerKW        *int64  `json:"power_kw,omitempty"`
	FuelType       *string `json:"fuel_type,omitempty"`
}

// SeedDoc is the complete seed document.
type SeedDoc struct {
	Header
	KBA      []KBARow     `json:"kba"`
	Vehicles []VehicleRow `json:"vehicles"`
}

// Result reports what Seed did.
type Result struct {
	Imported bool
	KBA      int
	Vehicles int
	Duration time.Duration
}

// Seed imports the embedded catalog at startup (research C6). It does nothing
// when this bundled seed was already handled. A changed bundled seed (a torq
// upgrade) replaces the catalog unless an online update imported newer data,
// so a restart never rolls an online update back. Any error leaves the
// previous catalog untouched.
func Seed(ctx context.Context, d *db.DB, seed []byte, now time.Time) (Result, error) {
	sum := sha256.Sum256(seed)
	hash := hex.EncodeToString(sum[:])
	meta, err := store.New(d.Reader).GetCatalogMeta(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("catalog meta: %w", err)
	}
	if meta.SeedSha256 != "" && meta.BundledSha256 == hash {
		return Result{}, nil
	}
	doc, err := DecodeSeed(seed)
	if err != nil {
		return Result{}, err
	}
	if meta.SeedSha256 != "" && meta.Source == SourceOnline && newerOnline(meta.HeaderJson, doc.GeneratedAt) {
		return Result{}, store.New(d.Writer).SetCatalogBundledSha(ctx, hash)
	}
	res, err := Import(ctx, d, doc, hash, SourceBundled, now)
	if err != nil {
		return Result{}, err
	}
	return res, store.New(d.Writer).SetCatalogBundledSha(ctx, hash)
}

// newerOnline reports whether the imported (online) catalog was generated
// after the bundled one.
func newerOnline(headerJSON, bundledGeneratedAt string) bool {
	var h Header
	if json.Unmarshal([]byte(headerJSON), &h) != nil {
		return false
	}
	online, err1 := time.Parse(time.RFC3339, h.GeneratedAt)
	bundled, err2 := time.Parse(time.RFC3339, bundledGeneratedAt)
	return err1 == nil && err2 == nil && online.After(bundled)
}

// Import validates doc and replaces the catalog with it in one write
// transaction, so readers see either the old or the new catalog. hash
// identifies the data; source is SourceBundled or SourceOnline.
func Import(ctx context.Context, d *db.DB, doc SeedDoc, hash, source string, now time.Time) (Result, error) {
	start := time.Now()
	if err := doc.Validate(); err != nil {
		return Result{}, err
	}
	header, err := json.Marshal(doc.Header)
	if err != nil {
		return Result{}, err
	}
	err = d.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if err := q.DeleteCatalogKBA(ctx); err != nil {
			return err
		}
		if err := q.DeleteCatalogVehicles(ctx); err != nil {
			return err
		}
		if err := insertKBA(ctx, tx, doc.KBA); err != nil {
			return err
		}
		if err := insertVehicles(ctx, tx, doc.Vehicles); err != nil {
			return err
		}
		return q.UpdateCatalogMeta(ctx, store.UpdateCatalogMetaParams{
			SeedSha256: hash, ImportedAt: now.UTC().Format(time.RFC3339), HeaderJson: string(header), Source: source,
		})
	})
	if err != nil {
		return Result{}, fmt.Errorf("catalog import: %w", err)
	}
	return Result{Imported: true, KBA: len(doc.KBA), Vehicles: len(doc.Vehicles), Duration: time.Since(start)}, nil
}

// DecodeSeed gunzips and decodes a seed document.
func DecodeSeed(seed []byte) (SeedDoc, error) {
	var doc SeedDoc
	zr, err := gzip.NewReader(bytes.NewReader(seed))
	if err != nil {
		return doc, fmt.Errorf("catalog seed: %w", err)
	}
	defer func() { _ = zr.Close() }()
	dec := json.NewDecoder(io.LimitReader(zr, 256<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return doc, fmt.Errorf("catalog seed: %w", err)
	}
	return doc, nil
}

// Validate checks every row against the catalog table constraints so a bad
// seed fails before touching the database.
func (doc SeedDoc) Validate() error {
	if doc.Schema != SeedSchema {
		return fmt.Errorf("catalog seed: unsupported schema %q", doc.Schema)
	}
	if doc.Counts.KBA != len(doc.KBA) || doc.Counts.Vehicles != len(doc.Vehicles) {
		return errors.New("catalog seed: counts do not match rows")
	}
	for i, r := range doc.KBA {
		if err := r.validate(); err != nil {
			return fmt.Errorf("catalog seed: kba[%d]: %w", i, err)
		}
	}
	for i, r := range doc.Vehicles {
		if err := r.validate(); err != nil {
			return fmt.Errorf("catalog seed: vehicles[%d]: %w", i, err)
		}
	}
	return nil
}

func (r KBARow) validate() error {
	switch {
	case !hsnRe.MatchString(r.HSN):
		return fmt.Errorf("hsn %q", r.HSN)
	case !tsnRe.MatchString(r.TSN):
		return fmt.Errorf("tsn %q", r.TSN)
	case r.Make == "" || r.TradeName == "":
		// Model may be empty: some keys name only the brand ("TOYOTA").
		return errors.New("make and trade_name are required")
	case r.Source != "sv42" && r.Source != "fz6":
		return fmt.Errorf("source %q", r.Source)
	case r.Match != "unique" && r.Match != "none":
		return fmt.Errorf("match %q", r.Match)
	}
	return validateTech(r.DisplacementCC, r.PowerKW, r.FuelType)
}

func (r VehicleRow) validate() error {
	if r.Make == "" || r.Model == "" || r.Engine == "" {
		return errors.New("make, model and engine are required")
	}
	if Key(r.Make) == "" || Key(r.Model) == "" || Key(r.Engine) == "" {
		return errors.New("make, model and engine need searchable characters")
	}
	if r.Body != "" && !slices.Contains(Bodies, r.Body) {
		return fmt.Errorf("body %q", r.Body)
	}
	return validateTech(r.DisplacementCC, r.PowerKW, r.FuelType)
}

func validateTech(cc, kw *int64, fuel *string) error {
	switch {
	case cc != nil && (*cc < 1 || *cc > 20000):
		return fmt.Errorf("displacement_cc %d", *cc)
	case kw != nil && (*kw < 1 || *kw > 2000):
		return fmt.Errorf("power_kw %d", *kw)
	case fuel != nil && !slices.Contains(FuelTypes, *fuel):
		return fmt.Errorf("fuel_type %q", *fuel)
	}
	return nil
}

func nInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func nStr(p *string) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *p, Valid: true}
}

func insertKBA(ctx context.Context, tx *sql.Tx, rows []KBARow) error {
	q := store.New(tx)
	for _, r := range rows {
		if err := q.InsertCatalogKBA(ctx, store.InsertCatalogKBAParams{
			Hsn: r.HSN, Tsn: r.TSN, Make: r.Make, Model: r.Model, ModelDe: cmp.Or(r.ModelDE, r.Model),
			TradeName: r.TradeName, Generation: r.Generation,
			EngineCode: r.EngineCode, DisplacementCc: nInt(r.DisplacementCC), PowerKw: nInt(r.PowerKW),
			FuelType: nStr(r.FuelType), BodyCode: r.BodyCode, YearFrom: nInt(r.YearFrom), Source: r.Source, Match: r.Match,
		}); err != nil {
			return err
		}
	}
	return nil
}

func insertVehicles(ctx context.Context, tx *sql.Tx, rows []VehicleRow) error {
	q := store.New(tx)
	for _, r := range rows {
		if err := q.InsertCatalogVehicle(ctx, store.InsertCatalogVehicleParams{
			Make: r.Make, MakeKey: Key(r.Make), Model: r.Model, ModelKey: Key(r.Model),
			ModelDe: cmp.Or(r.ModelDE, r.Model), ModelDeKey: Key(cmp.Or(r.ModelDE, r.Model)),
			Generation: r.Generation, GenerationKey: Key(r.Generation),
			GenerationDe: cmp.Or(r.GenerationDE, r.Generation), GenerationDeKey: Key(cmp.Or(r.GenerationDE, r.Generation)),
			YearFrom: nInt(r.YearFrom), YearTo: nInt(r.YearTo),
			Body: r.Body, Engine: r.Engine, EngineKey: Key(r.Engine), EngineCode: r.EngineCode,
			DisplacementCc: nInt(r.DisplacementCC), PowerKw: nInt(r.PowerKW), FuelType: nStr(r.FuelType),
		}); err != nil {
			return err
		}
	}
	return nil
}

// EncodeSeed writes doc as a gzipped seed with counts filled in. The output
// is deterministic for the same input (no gzip timestamp), so a regenerated
// seed only changes its hash when the data changes.
func EncodeSeed(doc SeedDoc) ([]byte, error) {
	doc.Schema = SeedSchema
	doc.Counts = Counts{KBA: len(doc.KBA), Vehicles: len(doc.Vehicles)}
	if doc.KBA == nil {
		doc.KBA = []KBARow{}
	}
	if doc.Vehicles == nil {
		doc.Vehicles = []VehicleRow{}
	}
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	enc := json.NewEncoder(zw)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
