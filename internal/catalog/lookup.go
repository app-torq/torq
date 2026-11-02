package catalog

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/app-torq/torq/internal/store"
)

// KBA returns every catalog variant for an HSN/TSN key. TSN matching is
// case-insensitive (FR-003).
func KBA(ctx context.Context, q *store.Queries, hsn, tsn string) ([]store.CatalogKba, error) {
	return q.LookupKBA(ctx, store.LookupKBAParams{Hsn: hsn, Tsn: strings.ToUpper(tsn)})
}

// Makes suggests makes matching query (empty query lists the first 25).
func Makes(ctx context.Context, q *store.Queries, query string) ([]string, error) {
	return q.CatalogMakes(ctx, Key(query))
}

// Lang returns the catalog language for a UI locale: "de" or "en".
func Lang(locale string) string {
	if locale == "de" {
		return "de"
	}
	return "en"
}

// Models suggests models of make, named in lang. Parents are matched by Key
// in either language, so a parent typed with different case or spacing still
// finds its children; an unknown parent yields no rows (FR-011).
func Models(ctx context.Context, q *store.Queries, make, query, lang string) ([]store.CatalogModelsRow, error) {
	return q.CatalogModels(ctx, store.CatalogModelsParams{MakeKey: Key(make), Q: Key(query), Lang: Lang(lang)})
}

// Generations suggests generations of make + model, named in lang.
func Generations(ctx context.Context, q *store.Queries, make, model, query, lang string) ([]store.CatalogGenerationsRow, error) {
	return q.CatalogGenerations(ctx, store.CatalogGenerationsParams{
		MakeKey: Key(make), ModelKey: Key(model), Q: Key(query), Lang: Lang(lang),
	})
}

// Engines suggests engines of make + model + generation.
func Engines(ctx context.Context, q *store.Queries, make, model, generation, query string) ([]store.CatalogEnginesRow, error) {
	return q.CatalogEngines(ctx, store.CatalogEnginesParams{
		MakeKey: Key(make), ModelKey: Key(model), GenerationKey: Key(generation), Q: Key(query),
	})
}

// Meta describes the imported catalog; ImportedAt is empty before the first
// import.
type Meta struct {
	ImportedAt string
	Source     string
	Header     Header
}

// LoadMeta reads the imported catalog's header (datasets and counts).
func LoadMeta(ctx context.Context, q *store.Queries) (Meta, error) {
	row, err := q.GetCatalogMeta(ctx)
	if err != nil {
		return Meta{}, err
	}
	m := Meta{Header: Header{Datasets: []Dataset{}}}
	if row.SeedSha256 == "" {
		return m, nil
	}
	if err := json.Unmarshal([]byte(row.HeaderJson), &m.Header); err != nil {
		return Meta{}, err
	}
	if m.Header.Datasets == nil {
		m.Header.Datasets = []Dataset{}
	}
	m.ImportedAt = row.ImportedAt
	m.Source = row.Source
	return m, nil
}
