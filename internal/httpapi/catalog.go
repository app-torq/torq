package httpapi

import (
	"net/http"
	"regexp"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/catalog"
	"github.com/marekbeckmann/torq/internal/store"
)

// Catalog endpoints (specs/002-vehicle-catalog-lookup/contracts/rest-api.md).
// They read only the embedded catalog and never call external hosts. Query
// failures are logged and answered with an empty list so the vehicle form
// stays usable (FR-018).

var (
	catalogHSNRe = regexp.MustCompile(`^[0-9]{4}$`)
	catalogTSNRe = regexp.MustCompile(`^[0-9A-Za-z]{3}$`)
)

type kbaVariantJSON struct {
	Make           string  `json:"make"`
	Model          string  `json:"model"`
	TradeName      string  `json:"trade_name"`
	Generation     string  `json:"generation"`
	EngineCode     string  `json:"engine_code"`
	DisplacementCC *int64  `json:"displacement_cc"`
	PowerKW        *int64  `json:"power_kw"`
	FuelType       *string `json:"fuel_type"`
	BodyCode       string  `json:"body_code"`
	YearFrom       *int64  `json:"year_from"`
	Match          string  `json:"match"`
}

type catalogNameJSON struct {
	Name string `json:"name"`
}

type catalogModelJSON struct {
	Name     string `json:"name"`
	YearFrom *int64 `json:"year_from"`
	YearTo   *int64 `json:"year_to"`
}

type catalogGenerationJSON struct {
	Name     string `json:"name"`
	YearFrom *int64 `json:"year_from"`
	YearTo   *int64 `json:"year_to"`
	Body     string `json:"body"`
}

type catalogEngineJSON struct {
	Name           string  `json:"name"`
	EngineCode     string  `json:"engine_code"`
	DisplacementCC *int64  `json:"displacement_cc"`
	PowerKW        *int64  `json:"power_kw"`
	FuelType       *string `json:"fuel_type"`
}

type catalogMetaJSON struct {
	ImportedAt *string           `json:"imported_at"`
	Datasets   []catalog.Dataset `json:"datasets"`
	Counts     catalog.Counts    `json:"counts"`
}

// yearPtr maps the 0 the aggregate queries use for "unknown" to null.
func yearPtr(y int64) *int64 {
	if y == 0 {
		return nil
	}
	return &y
}

// writeCatalog sends a catalog answer with the long private cache lifetime;
// catalog data only changes when torq is upgraded.
func writeCatalog[T any](w http.ResponseWriter, items []T) {
	w.Header().Set("Cache-Control", "private, max-age=86400")
	writeJSON(w, http.StatusOK, map[string]any{"items": orEmpty(items)})
}

func orEmpty[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// catalogFailed logs a lookup failure and answers with an empty list.
func catalogFailed[T any](s *Server, w http.ResponseWriter, r *http.Request, err error) error {
	s.Log.ErrorContext(r.Context(), "catalog lookup failed", "path", r.URL.Path, "err", err)
	writeCatalog[T](w, nil)
	return nil
}

// requireParams validates the required cascade parents.
func requireParams(r *http.Request, names ...string) error {
	var v apperr.Validation
	for _, n := range names {
		v.Check(r.URL.Query().Get(n) == "", n, "validation.required")
	}
	return v.Err()
}

func (s *Server) catalogQueries() *store.Queries { return store.New(s.DB.Reader) }

func (s *Server) getCatalogKBA(w http.ResponseWriter, r *http.Request) error {
	hsn, tsn := r.URL.Query().Get("hsn"), r.URL.Query().Get("tsn")
	var v apperr.Validation
	v.Check(!catalogHSNRe.MatchString(hsn), "hsn", "catalog.hsn_invalid")
	v.Check(!catalogTSNRe.MatchString(tsn), "tsn", "catalog.tsn_invalid")
	if err := v.Err(); err != nil {
		return err
	}
	rows, err := catalog.KBA(r.Context(), s.catalogQueries(), hsn, tsn)
	if err != nil {
		return catalogFailed[kbaVariantJSON](s, w, r, err)
	}
	items := make([]kbaVariantJSON, 0, len(rows))
	for _, k := range rows {
		items = append(items, kbaVariantJSON{
			Make: k.Make, Model: k.Model, TradeName: k.TradeName, Generation: k.Generation, EngineCode: k.EngineCode,
			DisplacementCC: intPtr(k.DisplacementCc), PowerKW: intPtr(k.PowerKw), FuelType: strPtr(k.FuelType),
			BodyCode: k.BodyCode, YearFrom: intPtr(k.YearFrom), Match: k.Match,
		})
	}
	writeCatalog(w, items)
	return nil
}

func (s *Server) getCatalogMakes(w http.ResponseWriter, r *http.Request) error {
	names, err := catalog.Makes(r.Context(), s.catalogQueries(), r.URL.Query().Get("q"))
	if err != nil {
		return catalogFailed[catalogNameJSON](s, w, r, err)
	}
	items := make([]catalogNameJSON, 0, len(names))
	for _, n := range names {
		items = append(items, catalogNameJSON{Name: n})
	}
	writeCatalog(w, items)
	return nil
}

func (s *Server) getCatalogModels(w http.ResponseWriter, r *http.Request) error {
	if err := requireParams(r, "make"); err != nil {
		return err
	}
	q := r.URL.Query()
	rows, err := catalog.Models(r.Context(), s.catalogQueries(), q.Get("make"), q.Get("q"))
	if err != nil {
		return catalogFailed[catalogModelJSON](s, w, r, err)
	}
	items := make([]catalogModelJSON, 0, len(rows))
	for _, m := range rows {
		items = append(items, catalogModelJSON{Name: m.Name, YearFrom: yearPtr(m.YearFrom), YearTo: yearPtr(m.YearTo)})
	}
	writeCatalog(w, items)
	return nil
}

func (s *Server) getCatalogGenerations(w http.ResponseWriter, r *http.Request) error {
	if err := requireParams(r, "make", "model"); err != nil {
		return err
	}
	q := r.URL.Query()
	rows, err := catalog.Generations(r.Context(), s.catalogQueries(), q.Get("make"), q.Get("model"), q.Get("q"))
	if err != nil {
		return catalogFailed[catalogGenerationJSON](s, w, r, err)
	}
	items := make([]catalogGenerationJSON, 0, len(rows))
	for _, g := range rows {
		items = append(items, catalogGenerationJSON{Name: g.Name, YearFrom: yearPtr(g.YearFrom), YearTo: yearPtr(g.YearTo), Body: g.Body})
	}
	writeCatalog(w, items)
	return nil
}

func (s *Server) getCatalogEngines(w http.ResponseWriter, r *http.Request) error {
	if err := requireParams(r, "make", "model", "generation"); err != nil {
		return err
	}
	q := r.URL.Query()
	rows, err := catalog.Engines(r.Context(), s.catalogQueries(), q.Get("make"), q.Get("model"), q.Get("generation"), q.Get("q"))
	if err != nil {
		return catalogFailed[catalogEngineJSON](s, w, r, err)
	}
	items := make([]catalogEngineJSON, 0, len(rows))
	for _, e := range rows {
		items = append(items, catalogEngineJSON{
			Name: e.Name, EngineCode: e.EngineCode, DisplacementCC: intPtr(e.DisplacementCc),
			PowerKW: intPtr(e.PowerKw), FuelType: strPtr(e.FuelType),
		})
	}
	writeCatalog(w, items)
	return nil
}

func (s *Server) getCatalogMeta(w http.ResponseWriter, r *http.Request) error {
	m, err := catalog.LoadMeta(r.Context(), s.catalogQueries())
	if err != nil {
		s.Log.ErrorContext(r.Context(), "catalog meta failed", "err", err)
		m = catalog.Meta{Header: catalog.Header{Datasets: []catalog.Dataset{}}}
	}
	out := catalogMetaJSON{Datasets: m.Header.Datasets, Counts: m.Header.Counts}
	if m.ImportedAt != "" {
		out.ImportedAt = &m.ImportedAt
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}
