package catalogsync

import (
	"slices"
	"strings"
	"testing"

	"github.com/app-torq/torq/internal/catalog"
)

// Rows as RDW exports them (research M2), with made-up counts.
const rdwAggregateCSV = `"merk","handelsbenaming","cilinderinhoud","vermogen_massarijklaar","y0","y1","n","k"
"DUCATI","MONSTER 821","821","0.39","20140918","20161117","300","07MNGR"
"DUCATI","MONSTER 821","821","0.37","20170710","20200921","140","02MRLH"
"DUCATI","MONSTER 821","821","0.17","20190409","20190409","8","03MVDJ"
"DUCATI","DUCATI SCRAMBLER","803","0.33","20150301","20240101","90","11AAAA"
"DUCATI","MULTISTRADA V4, MULTISTRADA V4 S","1158","0.47","20210101","20250101","40","12AAAA"
"DUCATI","MONSTER 696 ABS","696","0.35","20080101","20140101","60","13AAAA"
"KTM","1290 SUPERDUKE R (74 KW)","1301","0.36","20140101","20160101","20","21AAAA"
"KTM","1290 SUPER DUKE R","1301","0.83","20140101","20190101","50","22AAAA"
"KTM","350 EXC-F","349","0.40","20120101","20230101","30","23AAAA"
"HARLEY DAVIDSON","FLSTF FAT BOY","1584","0.20","20070101","20170101","70","31AAAA"
"HARLEY-DAVIDSON","FLHRCI ROAD KING CLASSIC","1450","0.17","19990101","20060101","40","32AAAA"
"HARLEY-DAVIDSON","HARLEY DAVIDSON FORTY-EIGHT","1202","0.24","20100101","20200101","20","33AAAA"
"BMW","R 1200 GS","1170","0.39","20130101","20180101","900","41AAAA"
"BMW","R 1200 GS","1170","0.34","20040101","20120101","800","33MBDB"
"BMW","RARE ONE","999","0.30","20000101","20000101","3","42AAAA"
"HONDA","CB650RA","649","0.31","20190101","20240101","70","51AAAA"
"HONDA","CB650R","649","0.31","20190101","20240101","10","52AAAA"
"HONDA","RC46","781","0.31","19980101","20010101","30","53AAAA"
"EIGENBOUW","CHOPPER","1000","0.10","19800101","19800101","900","61AAAA"
"TINYMAKE","MODEL","125","0.10","20000101","20000101","20","71AAAA"
"ZERO MOTORCYCLES","SR/F","","0.50","20200101","20240101","60","81AAAA"
"ZUENDAPP","KS 50","49","0.03","19700101","19800101","40","91AAAA"
"ZUNDAPP","KS 50","49","0.03","19700101","19800101","40","92AAAA"
`

const rdwFuelCSV = `"kenteken","brandstof_omschrijving","nettomaximumvermogen"
"07MNGR","Benzine","82.00"
"02MRLH","Benzine","82.00"
"03MVDJ","Benzine","35.00"
"11AAAA","Benzine","55.00"
"12AAAA","Benzine","125.00"
"13AAAA","Benzine","59.00"
"21AAAA","Benzine","74.00"
"22AAAA","Benzine","129.00"
"23AAAA","Benzine",""
"31AAAA","Benzine","59.00"
"32AAAA","Benzine","59.00"
"33AAAA","Benzine","49.00"
"41AAAA","Benzine","92.00"
"33MBDB","Benzine","81.00"
"51AAAA","Benzine","70.00"
"52AAAA","Benzine","70.00"
"53AAAA","Benzine","80.00"
"81AAAA","Elektriciteit","82.00"
"91AAAA","Benzine","2.00"
"92AAAA","Benzine","2.00"
`

const testOverrides = `
Honda | CB650RA | CB650R
Honda | RC46 | VFR 800
BMW | Rare One | -
`

func buildTestMotorcycles(t *testing.T) ([]catalog.VehicleRow, []rdwCandidate) {
	t.Helper()
	groups, err := readRDWAggregate(strings.NewReader(rdwAggregateCSV))
	if err != nil {
		t.Fatal(err)
	}
	fuel, err := readRDWFuel(strings.NewReader(rdwFuelCSV))
	if err != nil {
		t.Fatal(err)
	}
	ov, err := parseOverrides(testOverrides)
	if err != nil {
		t.Fatal(err)
	}
	cands := prepareMotorcycles(groups, ov, newReport())
	return buildMotorcycles(cands, fuel), cands
}

func TestRDWModelsAndMakes(t *testing.T) {
	rows, _ := buildTestMotorcycles(t)
	models := map[string][]string{}
	for _, r := range rows {
		if r.VehicleType != catalog.TypeMotorcycle {
			t.Fatalf("row without vehicle type: %+v", r)
		}
		if !slices.Contains(models[r.Make], r.Model) {
			models[r.Make] = append(models[r.Make], r.Model)
		}
	}
	for mk := range models {
		slices.Sort(models[mk])
	}
	want := map[string][]string{
		// Make prefix, comma list and trailing ABS removed; spellings merged.
		"Ducati": {"Monster 696", "Monster 821", "Multistrada V4", "Scrambler"},
		// Power suffix stripped, SUPERDUKE and SUPER DUKE merged (most registered spelling wins).
		"KTM": {"1290 Super Duke R", "350 EXC-F"},
		// Both spellings merged; codes upper case; make prefix in the trade name dropped.
		"Harley-Davidson": {"FLHRCI Road King Classic", "FLSTF Fat Boy", "Forty-Eight"},
		// "Rare One" dropped by an override.
		"BMW": {"R 1200 GS"},
		// Overrides merge CB650RA into CB650R and rename RC46.
		"Honda": {"CB650R", "VFR 800"},
		"Zero":  {"SR/F"},
		// ZUENDAPP and ZUNDAPP are one make: 80 registrations together.
		"Zündapp": {"KS 50"},
	}
	for mk, w := range want {
		if !slices.Equal(models[mk], w) {
			t.Errorf("%s: %v, want %v", mk, models[mk], w)
		}
	}
	// EIGENBOUW is self-built, TINYMAKE has fewer than 50 registrations.
	for _, mk := range []string{"Eigenbouw", "Tinymake"} {
		if _, ok := models[mk]; ok {
			t.Errorf("%s should be left out", mk)
		}
	}
	if len(models) != len(want) {
		t.Errorf("makes %v", models)
	}
}

func TestRDWVersions(t *testing.T) {
	rows, cands := buildTestMotorcycles(t)
	find := func(model string, kw int64) *catalog.VehicleRow {
		for i, r := range rows {
			if r.Model == model && r.PowerKW != nil && *r.PowerKW == kw {
				return &rows[i]
			}
		}
		return nil
	}
	// Two groups with 82 kW merge into one version spanning both year ranges.
	m := find("Monster 821", 82)
	if m == nil || *m.DisplacementCC != 821 || *m.YearFrom != 2014 || *m.YearTo != 2020 || *m.FuelType != "petrol" || m.Engine != "821 cm³ 82 kW" {
		t.Fatalf("Monster 821 82 kW: %+v", m)
	}
	// The A2 version is its own row.
	if find("Monster 821", 35) == nil {
		t.Error("restricted version missing")
	}
	// Restricted and full KTM versions of one model.
	if find("1290 Super Duke R", 74) == nil || find("1290 Super Duke R", 129) == nil {
		t.Error("KTM versions missing")
	}
	// No power in the fuel data: a version with displacement only.
	var exc *catalog.VehicleRow
	for i, r := range rows {
		if r.Model == "350 EXC-F" {
			exc = &rows[i]
		}
	}
	if exc == nil || exc.PowerKW != nil || exc.Engine != "349 cm³" {
		t.Fatalf("350 EXC-F: %+v", exc)
	}
	// Electric: no displacement.
	z := find("SR/F", 82)
	if z == nil || z.DisplacementCC != nil || *z.FuelType != "electric" || z.Engine != "82 kW" {
		t.Fatalf("Zero: %+v", z)
	}
	// Every row passes the seed validation.
	doc := catalog.SeedDoc{Header: catalog.Header{Schema: catalog.SeedSchema,
		Counts: catalog.Counts{Vehicles: len(rows), Motorcycles: len(rows)}}, Vehicles: rows}
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	// Samples: one per group with at least two registrations, no duplicates.
	s := rdwSamples(cands)
	if !slices.IsSorted(s) || slices.Contains(s, "61AAAA") || !slices.Contains(s, "03MVDJ") {
		t.Fatalf("samples %v", s)
	}
}

func TestRDWWords(t *testing.T) {
	for in, want := range map[string]string{
		"SUPER": "Super", "GS": "GS", "EXC-F": "EXC-F", "FAT": "Fat", "XSR": "XSR", "V-STROM": "V-Strom",
		"ZX-10R": "ZX-10R", "TÉNÉRÉ": "Ténéré", "ADV": "ADV", "TRANSALP": "Transalp",
	} {
		if got := rdwWord(in, false); got != want {
			t.Errorf("rdwWord(%q) = %q, want %q", in, got, want)
		}
	}
	if got := rdwWord("FLHRCI", true); got != "FLHRCI" {
		t.Errorf("Harley code: %q", got)
	}
	if got := rdwWord("FLHRCI", false); got != "Flhrci" {
		t.Errorf("non-Harley: %q", got)
	}
}

func TestParseOverrides(t *testing.T) {
	if _, err := parseOverrides("Honda | CB650RA"); err == nil {
		t.Error("two fields accepted")
	}
	if _, err := parseOverrides("Honda | A | B\nHONDA | a | C"); err == nil {
		t.Error("duplicate accepted")
	}
	if len(motorcycleRules) == 0 {
		t.Error("embedded motorcycles.txt is empty")
	}
}
