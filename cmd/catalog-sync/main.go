// Command catalog-sync builds torq's embedded vehicle catalog
// (internal/catalog/data/catalog_seed.json.gz) from downloaded open-data
// files. It is a maintainer tool: the Docker build and CI never run it, and
// the build never touches the network (specs/002-vehicle-catalog-lookup,
// research C7). Only --fetch-rdw downloads, as a separate step: the RDW data
// is a query result, not a file to download (specs/006-motorcycles, M2).
// The build itself lives in internal/catalog/catalogsync, which the server
// also uses for admin-triggered online updates.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/app-torq/torq/internal/catalog"
	"github.com/app-torq/torq/internal/catalog/catalogsync"
)

type options struct {
	fz6, fz6Stand, gor3a, gor3aCommit, sv42, sv42Consent, rdwAgg, rdwFuel, retrieved, out string
}

func main() {
	var o options
	flag.StringVar(&o.fz6, "fz6-xlsx", "", "KBA FZ 6 workbook (.xlsx)")
	flag.StringVar(&o.fz6Stand, "fz6-stand", "2024-01-01", "FZ 6 reference date (YYYY-MM-DD)")
	flag.StringVar(&o.gor3a, "gor3a-json", "", "gor3a/vehicle-makes-models data/json/all.json")
	flag.StringVar(&o.gor3aCommit, "gor3a-commit", "", "commit SHA the gor3a file was taken from")
	flag.StringVar(&o.sv42, "sv42-text", "", "optional: KBA SV 4.2 as `pdftotext -layout` text")
	flag.StringVar(&o.sv42Consent, "sv42-consent", "", `"yes" confirms written KBA consent to redistribute SV 4.2 data`)
	flag.StringVar(&o.rdwAgg, "rdw-aggregate", "", "RDW motorcycle aggregate (.csv, from --fetch-rdw)")
	flag.StringVar(&o.rdwFuel, "rdw-fuel", "", "RDW fuel and power of the sample registrations (.csv, from --fetch-rdw)")
	flag.StringVar(&o.retrieved, "retrieved", "", "retrieval date of the source files (YYYY-MM-DD)")
	flag.StringVar(&o.out, "out", "", "output seed path (.json.gz)")
	fetchRDW := flag.String("fetch-rdw", "", "only download the RDW data into `dir` (rdw-aggregate.csv, rdw-fuel.csv) and exit")
	flag.Parse()
	if *fetchRDW != "" {
		if err := fetch(*fetchRDW); err != nil {
			fmt.Fprintln(os.Stderr, "catalog-sync:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(o, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "catalog-sync:", err)
		os.Exit(1)
	}
}

func run(o options, stdout io.Writer) error {
	if o.fz6 == "" || o.gor3a == "" || o.gor3aCommit == "" || o.rdwAgg == "" || o.rdwFuel == "" || o.retrieved == "" || o.out == "" {
		return errors.New("--fz6-xlsx, --gor3a-json, --gor3a-commit, --rdw-aggregate, --rdw-fuel, --retrieved and --out are required")
	}
	if o.sv42 != "" && o.sv42Consent != "yes" {
		return errors.New(`SV 4.2 is "all rights reserved"; pass --sv42-consent=yes only with written KBA consent`)
	}
	fz6, err := os.ReadFile(o.fz6)
	if err != nil {
		return err
	}
	gor3a, err := os.Open(o.gor3a)
	if err != nil {
		return err
	}
	defer func() { _ = gor3a.Close() }()
	rdwAgg, err := os.Open(o.rdwAgg)
	if err != nil {
		return err
	}
	defer func() { _ = rdwAgg.Close() }()
	rdwFuel, err := os.Open(o.rdwFuel)
	if err != nil {
		return err
	}
	defer func() { _ = rdwFuel.Close() }()
	in := catalogsync.Inputs{
		FZ6: fz6, FZ6Stand: o.fz6Stand, Gor3a: gor3a, Gor3aCommit: o.gor3aCommit,
		RDWAggregate: rdwAgg, RDWFuel: rdwFuel,
		Retrieved: o.retrieved, SV42Consent: o.sv42Consent == "yes",
	}
	if o.sv42 != "" {
		f, err := os.Open(o.sv42)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		in.SV42 = f
	}
	doc, rep, err := catalogsync.Build(in)
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
	return rep.Print(stdout, doc, len(seed))
}

// fetch downloads the RDW data with the same queries as the online update.
func fetch(dir string) error {
	agg, fuel, err := catalogsync.NewFetcher("torq-catalog-sync").FetchRDW(context.Background())
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // maintainer download directory
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "rdw-aggregate.csv"), agg, 0o644); err != nil { //nolint:gosec // downloaded open data
		return err
	}
	return os.WriteFile(filepath.Join(dir, "rdw-fuel.csv"), fuel, 0o644) //nolint:gosec // downloaded open data
}
