// Command catalog-sync builds torq's embedded vehicle catalog
// (internal/catalog/data/catalog_seed.json.gz) from downloaded open-data
// files. It is a maintainer tool: the server, Docker build and CI never run
// it, and it never touches the network (specs/002-vehicle-catalog-lookup,
// research C7).
package main

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/marekbeckmann/torq/internal/catalog"
)

type options struct {
	fz6, gor3a, gor3aCommit, sv42, sv42Consent, retrieved, out string
}

func main() {
	var o options
	flag.StringVar(&o.fz6, "fz6-xlsx", "", "KBA FZ 6 workbook (.xlsx)")
	flag.StringVar(&o.gor3a, "gor3a-json", "", "gor3a/vehicle-makes-models data/json/all.json")
	flag.StringVar(&o.gor3aCommit, "gor3a-commit", "", "commit SHA the gor3a file was taken from")
	flag.StringVar(&o.sv42, "sv42-text", "", "optional: KBA SV 4.2 as `pdftotext -layout` text")
	flag.StringVar(&o.sv42Consent, "sv42-consent", "", `"yes" confirms written KBA consent to redistribute SV 4.2 data`)
	flag.StringVar(&o.retrieved, "retrieved", "", "retrieval date of the source files (YYYY-MM-DD)")
	flag.StringVar(&o.out, "out", "", "output seed path (.json.gz)")
	flag.Parse()
	if err := run(o, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "catalog-sync:", err)
		os.Exit(1)
	}
}

func run(o options, stdout io.Writer) error {
	if o.fz6 == "" || o.gor3a == "" || o.gor3aCommit == "" || o.retrieved == "" || o.out == "" {
		return errors.New("--fz6-xlsx, --gor3a-json, --gor3a-commit, --retrieved and --out are required")
	}
	if o.sv42 != "" && o.sv42Consent != "yes" {
		return errors.New(`SV 4.2 is "all rights reserved"; pass --sv42-consent=yes only with written KBA consent`)
	}
	doc, rep, err := build(o)
	if err != nil {
		return err
	}
	seed, err := catalog.EncodeSeed(doc)
	if err != nil {
		return err
	}
	if err := os.WriteFile(o.out, seed, 0o644); err != nil { //nolint:gosec // generated data file committed to the repo
		return err
	}
	return rep.print(stdout, doc, len(seed))
}

func build(o options) (catalog.SeedDoc, *report, error) {
	rep := newReport()
	vehicles, err := readGor3a(o.gor3a, rep)
	if err != nil {
		return catalog.SeedDoc{}, nil, err
	}
	idx := newModelIndex(vehicles)
	fz6, err := readFZ6(o.fz6)
	if err != nil {
		return catalog.SeedDoc{}, nil, err
	}
	var sv42 []sv42Row
	if o.sv42 != "" {
		if sv42, err = readSV42(o.sv42, rep); err != nil {
			return catalog.SeedDoc{}, nil, err
		}
	}
	kba := mergeKBA(fz6, sv42, idx, rep)

	sort.SliceStable(vehicles, func(i, j int) bool {
		a, b := vehicles[i], vehicles[j]
		return cmp.Or(
			cmp.Compare(catalog.Key(a.Make), catalog.Key(b.Make)), cmp.Compare(catalog.Key(a.Model), catalog.Key(b.Model)),
			cmp.Compare(catalog.Key(a.Generation), catalog.Key(b.Generation)), cmp.Compare(catalog.Key(a.Engine), catalog.Key(b.Engine)),
			cmp.Compare(ptr(a.PowerKW), ptr(b.PowerKW)),
		) < 0
	})
	doc := catalog.SeedDoc{
		Header: catalog.Header{
			GeneratedAt: o.retrieved + "T00:00:00Z",
			License:     "ODbL-1.0",
			Datasets:    datasets(o),
		},
		KBA:      kba,
		Vehicles: vehicles,
	}
	doc.Schema = catalog.SeedSchema
	doc.Counts = catalog.Counts{KBA: len(kba), Vehicles: len(vehicles)}
	if err := doc.Validate(); err != nil {
		return doc, nil, err
	}
	return doc, rep, nil
}

// mergeKBA turns FZ 6 (and, with consent, SV 4.2) rows into catalog rows.
// SV 4.2 rows win for keys present in both; FZ 6 only adds missing keys.
func mergeKBA(fz6 []fz6Row, sv42 []sv42Row, idx modelIndex, rep *report) []catalog.KBARow {
	var rows []catalog.KBARow
	fromSV42 := map[string]bool{}
	for _, r := range sv42 {
		base, ok := manufacturers[r.Manufacturer]
		if !ok {
			rep.unmapped[r.Manufacturer]++
			continue
		}
		fromSV42[r.HSN+r.TSN] = true
		var fuelType *string
		if r.FuelCode != "" {
			f, known := sv42Fuel[r.FuelCode]
			if !known {
				rep.unknownFuel["SV42 "+r.FuelCode]++
				f = "other"
			}
			fuelType = &f
		}
		for _, trade := range splitTradeNames(r.TradeName) {
			mk := brandFor(r.Manufacturer, base, trade)
			model, matched := idx.modelFor(mk, trade)
			rows = append(rows, kbaRow(r.HSN, r.TSN, mk, model, trade, matched, "sv42", func(k *catalog.KBARow) {
				k.DisplacementCC, k.PowerKW, k.FuelType, k.BodyCode, k.YearFrom = r.DisplacementCC, r.PowerKW, fuelType, r.Body, r.Year
				if fuelType != nil && *fuelType == "electric" {
					k.DisplacementCC = nil
				}
			}))
		}
	}
	for _, r := range fz6 {
		base, ok := manufacturers[r.Manufacturer]
		if !ok {
			rep.unmapped[r.Manufacturer] += r.Count
			continue
		}
		if fromSV42[r.HSN+r.TSN] {
			continue
		}
		for _, trade := range splitTradeNames(r.TradeName) {
			mk := brandFor(r.Manufacturer, base, trade)
			model, matched := idx.modelFor(mk, trade)
			rows = append(rows, kbaRow(r.HSN, r.TSN, mk, model, trade, matched, "fz6", nil))
		}
	}
	seen := map[string]bool{}
	out := rows[:0]
	for _, r := range rows {
		k := r.HSN + r.TSN + "|" + catalog.Key(r.TradeName)
		if seen[k] {
			continue
		}
		seen[k] = true
		if r.Match == "unique" {
			rep.matched++
		} else {
			rep.unmatched++
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		return cmp.Or(cmp.Compare(a.HSN, b.HSN), cmp.Compare(a.TSN, b.TSN), cmp.Compare(a.TradeName, b.TradeName)) < 0
	})
	return out
}

func kbaRow(hsn, tsn, mk, model, trade string, matched bool, source string, extra func(*catalog.KBARow)) catalog.KBARow {
	r := catalog.KBARow{HSN: hsn, TSN: tsn, Make: mk, Model: model, TradeName: trade, Source: source, Match: "none"}
	if matched {
		r.Match = "unique"
	}
	if extra != nil {
		extra(&r)
	}
	return r
}

// splitTradeNames splits KBA's combined trade names ("C 220 CDI;CLC 220 CDI")
// into variants, dropping purely numeric segments ("202 022"), placeholder
// segments ("BT....") and "UNBEKANNT".
func splitTradeNames(s string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ';' || r == ',' }) {
		p := strings.Join(strings.Fields(part), " ")
		if p == "" || p == "UNBEKANNT" || strings.Contains(p, "..") || !strings.ContainsFunc(p, unicode.IsLetter) {
			continue
		}
		if len([]rune(p)) > 64 {
			continue
		}
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

func datasets(o options) []catalog.Dataset {
	ds := []catalog.Dataset{
		{
			ID: "kba-fz6", Name: "Bestand an Kraftfahrzeugen und Kraftfahrzeuganhängern nach Herstellern und Typen (FZ 6)",
			Publisher: "Kraftfahrt-Bundesamt, Flensburg",
			URI:       "https://www.kba.de/DE/Statistik/Fahrzeuge/Bestand/ListeHerstellerTypSchluesselnummern/typschluesselnummern_node.html",
			Stand:     "2024-01-01", Retrieved: o.retrieved,
			License: "Datenlizenz Deutschland – Namensnennung – Version 2.0", LicenseID: "dl-de/by-2-0",
			LicenseURL:  "https://www.govdata.de/dl-de/by-2-0",
			Attribution: "© Kraftfahrt-Bundesamt, Flensburg. Datenquelle: Kraftfahrt-Bundesamt, FZ 6, " + o.retrieved + "; Datenlizenz by-2-0; Daten geändert",
			Modified:    true,
		},
		{
			ID: "gor3a", Name: "vehicle-makes-models (data/json/all.json)",
			Publisher: "vehicle-makes-models project (gor3a), data compiled from autoevolution.com",
			URI:       "https://github.com/gor3a/vehicle-makes-models", Retrieved: o.retrieved, Commit: o.gor3aCommit,
			License: "Open Database License (ODbL) 1.0", LicenseID: "ODbL-1.0",
			LicenseURL:  "https://opendatacommons.org/licenses/odbl/1-0/",
			Attribution: "Contains data from vehicle-makes-models (https://github.com/gor3a/vehicle-makes-models) and autoevolution.com, licensed under ODbL 1.0; changes were made.",
			Modified:    true,
		},
	}
	if o.sv42 != "" {
		ds = append(ds, catalog.Dataset{
			ID: "kba-sv42", Name: "Verzeichnis der Hersteller und Typen (SV 4.2)", Publisher: "Kraftfahrt-Bundesamt, Flensburg",
			URI:       "https://www.kba.de/DE/Themen/Typgenehmigung/CoC_Daten_Fahrzeugtypdaten/Veroeffentlichungen/SV4.html",
			Retrieved: o.retrieved, License: "Redistributed with written consent of the Kraftfahrt-Bundesamt", LicenseID: "KBA-consent",
			LicenseURL:  "https://www.kba.de/DE/Themen/Typgenehmigung/CoC_Daten_Fahrzeugtypdaten/Veroeffentlichungen/SV4.html",
			Attribution: "© Kraftfahrt-Bundesamt, Flensburg. Datenquelle: Kraftfahrt-Bundesamt, SV 4.2, " + o.retrieved + "; Daten geändert",
			Modified:    true,
		})
	}
	return ds
}
