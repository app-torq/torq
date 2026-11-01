package catalogsync

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/marekbeckmann/torq/internal/catalog"
)

// All fixtures are synthetic (HSN 99xx); no KBA or autoevolution content is
// stored in the repository.

const gor3aFixture = `[
 {"group": "mercedes-benz", "makes": [{"name": "Mercedes-Benz", "models": [
  {"name": "MERCEDES BENZ C-Klasse and predecessors", "yearStart": 1993, "yearEnd": null, "generations": [
   {"name": "MERCEDES BENZ C-Klasse and predecessors (2000)", "yearStart": 2000, "yearEnd": 2004, "bodyType": null, "engines": [
    {"label": "220 CDI 5AT RWD (143 HP)", "fuelType": "Diesel", "displacementCc": 2148, "powerHp": 143,
     "specs": {"ENGINE SPECS / Power": "105 KW @ 4200 RPM143 HP @ 4200 RPM141 BHP @ 4200 RPM"}},
    {"label": "220 CDI 6MT RWD (143 HP)", "fuelType": "Diesel", "displacementCc": 2148, "powerHp": 143, "specs": {}},
    {"label": "220 CDI 6MT RWD (143 HP)", "fuelType": "Diesel", "displacementCc": 2148, "powerHp": 143, "specs": {}}
   ]},
   {"name": "MERCEDES BENZ C-Klasse and predecessors Coupe (2016)", "yearStart": 2016, "yearEnd": 2018, "bodyType": "Coupe", "engines": [
    {"label": "C 300 h (231 HP)", "fuelType": "Hybrid Diesel", "displacementCc": 2143, "powerHp": null,
     "specs": {"ENGINE SPECS / Power": "170 KW @ 3800 RPM"}}
   ]}
  ]}
 ]}]},
 {"group": "volkswagen", "makes": [{"name": "Volkswagen", "models": [
  {"name": "Golf", "yearStart": 1974, "yearEnd": null, "generations": [
   {"name": "Golf 5 Doors (2024)", "yearStart": 2024, "yearEnd": null, "bodyType": "Hatchback", "engines": [
    {"label": "e-Golf (136 HP)", "fuelType": "Electric", "displacementCc": 0, "powerHp": 136, "specs": {}},
    {"label": "1.5 TSI 7DSG FWD (150 HP)", "fuelType": "Gasoline", "displacementCc": 1498, "powerHp": 150, "specs": {}}
   ]}
  ]}
 ]}]},
 {"group": "bmw", "makes": [{"name": "BMW", "models": [
  {"name": "3 Series", "yearStart": 1975, "yearEnd": null, "generations": [
   {"name": "3 Series (1998)", "yearStart": 1998, "yearEnd": 2005, "bodyType": null, "engines": [
    {"label": "320d 6MT RWD (150 HP)", "fuelType": "Diesel", "displacementCc": 1995, "powerHp": 150, "specs": {}}
   ]}
  ]}
 ]}]},
 {"group": "toyota", "makes": [{"name": "Toyota", "models": [
  {"name": "Corolla", "yearStart": 1966, "yearEnd": null, "generations": [
   {"name": "Corolla (2019)", "yearStart": 2019, "yearEnd": null, "bodyType": null, "engines": [
    {"label": "1.8 Hybrid (122 HP)", "fuelType": "Hybrid", "displacementCc": 1798, "powerHp": 122, "specs": {}}
   ]}
  ]}
 ]}, {"name": "Lexus", "models": [
  {"name": "IS", "yearStart": 1999, "yearEnd": null, "generations": [
   {"name": "IS (2005)", "yearStart": 2005, "yearEnd": 2013, "bodyType": null, "engines": [
    {"label": "IS 220d (177 HP)", "fuelType": "Diesel", "displacementCc": 2231, "powerHp": 177, "specs": {}}
   ]}
  ]}
 ]}]}
]`

// fz6Rows are written into a minimal FZ 6 workbook: a cover sheet without a
// header, then the data sheet with inline strings and shared strings.
var fz6Rows = [][]string{
	{"9901", "DAIMLERCHRYSLER (D)", "AAA", "C 220 CDI;CLC 220 CDI", "1440"},
	{"9901", "DAIMLERCHRYSLER (D)", "AAB", "C 220;202 022", "12"},
	{"9902", "VOLKSWAGEN-VW", "abc", "GOLF", "100"},
	{"9902", "VOLKSWAGEN-VW", "ABD", "TRANSPORTER", "7"},
	{"9903", "BAYER.MOT.WERKE-BMW", "AAA", "320D", "50"},
	{"9903", "BAYER.MOT.WERKE-BMW", "AAB", "COOPER", "20"},
	{"9904", "TOYOTA EUROPE (B)", "AAA", "TOYOTA COROLLA", "30"},
	{"9904", "TOYOTA EUROPE (B)", "AAB", "LEXUS IS 220D", "3"},
	{"9904", "TOYOTA EUROPE (B)", "AAC", "TOYOTA", "900"},
	{"9905", "SCHMITZ CARGOBULL", "AAA", "S01", "5000"},
	{"9906", "VOLKSWAGEN-VW", "ZZZ", "UNBEKANNT", "1"},
	{"9907", "PSA AUTOMOBILES (F)", "AAA", "BERLINGO,PARTNER,RIFTER", "9"},
	{"9908", "VOLKSWAGEN-VW", "AAA", "PASSAT", "4"},
}

func xlsx(t *testing.T, rows [][]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, body string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	const ns = `xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"`
	add("xl/sharedStrings.xml", `<sst `+ns+`><si><t>Hersteller-
schlüssel-
nummer</t></si><si><r><t>Hersteller</t></r><r><t>klartext</t></r></si></sst>`)
	add("xl/worksheets/sheet1.xml", `<worksheet `+ns+`><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>Deckblatt</t></is></c></row></sheetData></worksheet>`)
	var sb strings.Builder
	sb.WriteString(`<worksheet ` + ns + `><sheetData>`)
	sb.WriteString(`<row r="7"><c r="A7" t="s"><v>0</v></c><c r="B7" t="s"><v>1</v></c>` +
		`<c r="C7" t="inlineStr"><is><t>Typ-schlüssel-nummer</t></is></c><c r="D7" t="inlineStr"><is><t>Handelsname</t></is></c>` +
		`<c r="E7" t="inlineStr"><is><t>Anzahl</t></is></c></row>`)
	for i, r := range rows {
		n := i + 8
		sb.WriteString(`<row>`)
		for j, v := range r {
			ref := string(rune('A' + j))
			if j == 4 {
				sb.WriteString(`<c r="` + ref + itoa(n) + `"><v>` + v + `</v></c>`)
				continue
			}
			sb.WriteString(`<c r="` + ref + itoa(n) + `" t="inlineStr"><is><t>` + v + `</t></is></c>`)
		}
		sb.WriteString(`</row>`)
	}
	sb.WriteString(`<row><c r="A99" t="inlineStr"><is><t>© Kraftfahrt-Bundesamt, Flensburg</t></is></c></row></sheetData></worksheet>`)
	add("xl/worksheets/sheet2.xml", sb.String())
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

// sv42Fixture mimics `pdftotext -layout` output: a column-number line
// followed by one record per line.
const sv42Fixture = `                 Verzeichnis der Hersteller und Typen (synthetic test fixture)
  1     2     3                       4                    5                         6           7     8     9      10     11
  9908  AAA   VOLKSWAGEN-VW           VW                   PASSAT VARIANT            01.02.2011  M1    AC    0002   103    1968
  9909  ABC   TESLA (USA)             TESLA                MODEL 3                   15.06.2019  M1    AA    0004   324
  9909  ABD   TESLA (USA)             TESLA                MODEL Y                   01.01.2021  M1    AF    0099   220
`

func fixtures(t *testing.T) Inputs {
	t.Helper()
	return Inputs{
		FZ6: xlsx(t, fz6Rows), FZ6Stand: "2024-01-01", Gor3a: strings.NewReader(gor3aFixture), Gor3aCommit: "0123abc",
		Retrieved: "2026-10-31",
	}
}

func kbaVariants(doc catalog.SeedDoc, hsn, tsn string) []catalog.KBARow {
	var out []catalog.KBARow
	for _, r := range doc.KBA {
		if r.HSN == hsn && r.TSN == tsn {
			out = append(out, r)
		}
	}
	return out
}

func TestBuildFromFZ6AndGor3a(t *testing.T) {
	doc, rep, err := Build(fixtures(t))
	if err != nil {
		t.Fatal(err)
	}

	// Taxonomy cleanup: make prefix, "and predecessors", localized model,
	// chassis codes, HP suffix, kW from the source kW figure, duplicate
	// engines removed.
	var mb []catalog.VehicleRow
	for _, v := range doc.Vehicles {
		if v.Make == "Mercedes-Benz" {
			mb = append(mb, v)
		}
	}
	if len(mb) != 3 {
		t.Fatalf("mercedes rows %+v", mb)
	}
	if mb[0].Model != "C-Class" || mb[0].ModelDE != "C-Klasse" || mb[0].Generation != "C205" || mb[0].Body != "coupe" ||
		*mb[0].PowerKW != 170 || *mb[0].FuelType != "hybrid" {
		t.Errorf("coupe row %+v", mb[0])
	}
	if mb[1].Generation != "W203" || mb[1].GenerationDE != "" || mb[1].Body != "" || mb[1].Engine != "220 CDI 5AT RWD" ||
		*mb[1].PowerKW != 105 || *mb[1].DisplacementCC != 2148 || *mb[1].FuelType != "diesel" {
		t.Errorf("mercedes row %+v", mb[1])
	}
	for _, v := range doc.Vehicles {
		if v.Engine == "e-Golf" && (v.DisplacementCC != nil || *v.PowerKW != 100 || v.Generation != "Golf VIII (CD)" || v.Body != "hatchback") {
			t.Errorf("electric row %+v", v)
		}
		if v.Engine == "320d 6MT RWD" && (*v.PowerKW != 110 || v.Generation != "E46" || v.ModelDE != "3er") { // 150 PS × 0.7355
			t.Errorf("BMW row %+v", v)
		}
		if v.Make == "Toyota" && (v.Generation != "2019–" || v.ModelDE != "") {
			t.Errorf("uncovered make keeps the year label %+v", v)
		}
	}

	// KBA: manufacturer normalization, variant split, model matching.
	if got := kbaVariants(doc, "9901", "AAA"); len(got) != 2 || got[0].Make != "Mercedes-Benz" || got[0].Model != "C-Class" ||
		got[0].ModelDE != "C-Klasse" || got[0].TradeName != "C 220 CDI" || got[0].Match != "unique" || got[1].TradeName != "CLC 220 CDI" {
		t.Errorf("9901/AAA %+v", got)
	}
	if got := kbaVariants(doc, "9901", "AAB"); len(got) != 1 || got[0].TradeName != "C 220" {
		t.Errorf("numeric segment not dropped: %+v", got)
	}
	if got := kbaVariants(doc, "9902", "ABC"); len(got) != 1 || got[0].Make != "Volkswagen" || got[0].Model != "Golf" || got[0].ModelDE != "" {
		t.Errorf("lower-case TSN / VW %+v", got)
	}
	if got := kbaVariants(doc, "9902", "ABD"); len(got) != 1 || got[0].Model != "TRANSPORTER" || got[0].Match != "none" {
		t.Errorf("unmatched model %+v", got)
	}
	if got := kbaVariants(doc, "9903", "AAA"); len(got) != 1 || got[0].Model != "3 Series" || got[0].ModelDE != "3er" {
		t.Errorf("BMW series rule %+v", got)
	}
	if got := kbaVariants(doc, "9903", "AAB"); len(got) != 1 || got[0].Make != "MINI" {
		t.Errorf("MINI brand %+v", got)
	}
	if got := kbaVariants(doc, "9904", "AAA"); len(got) != 1 || got[0].Model != "Corolla" {
		t.Errorf("brand word stripped %+v", got)
	}
	if got := kbaVariants(doc, "9904", "AAB"); len(got) != 1 || got[0].Make != "Lexus" || got[0].Model != "IS" {
		t.Errorf("Lexus %+v", got)
	}
	if got := kbaVariants(doc, "9904", "AAC"); len(got) != 1 || got[0].Model != "" || got[0].Make != "Toyota" {
		t.Errorf("brand-only %+v", got)
	}
	if got := kbaVariants(doc, "9907", "AAA"); len(got) != 3 || got[0].Make != "Citroën" || got[1].Make != "Peugeot" {
		t.Errorf("PSA split %+v", got)
	}
	if len(kbaVariants(doc, "9905", "AAA")) != 0 || len(kbaVariants(doc, "9906", "ZZZ")) != 0 {
		t.Error("trailer or UNBEKANNT row kept")
	}

	if len(doc.Datasets) != 2 || doc.Datasets[0].LicenseID != "dl-de/by-2-0" || doc.Datasets[0].Stand != "2024-01-01" ||
		doc.Datasets[1].Commit != "0123abc" || !strings.Contains(doc.Datasets[0].Attribution, "Kraftfahrt-Bundesamt") ||
		doc.GeneratedAt != "2026-10-31T00:00:00Z" {
		t.Errorf("datasets %+v", doc.Header)
	}
	var out bytes.Buffer
	if err := rep.Print(&out, doc, 1); err != nil {
		t.Fatal(err)
	}
	if r := out.String(); !strings.Contains(r, "SCHMITZ CARGOBULL") || !strings.Contains(r, "KBA model match:") ||
		!strings.Contains(r, "Mercedes-Benz    2 of 2") {
		t.Errorf("report:\n%s", r)
	}

	// Deterministic output: same inputs, same bytes.
	first, err := catalog.EncodeSeed(doc)
	if err != nil {
		t.Fatal(err)
	}
	again, _, err := Build(fixtures(t))
	if err != nil {
		t.Fatal(err)
	}
	if second, _ := catalog.EncodeSeed(again); !bytes.Equal(first, second) {
		t.Error("output is not deterministic")
	}
}

func TestSV42RequiresConsentAndWins(t *testing.T) {
	in := fixtures(t)
	in.SV42 = strings.NewReader(sv42Fixture)
	if _, _, err := Build(in); err == nil || !strings.Contains(err.Error(), "consent") {
		t.Fatalf("missing consent accepted: %v", err)
	}
	in = fixtures(t)
	in.SV42, in.SV42Consent = strings.NewReader(sv42Fixture), true
	doc, rep, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	_ = rep.Print(&out, doc, 1)
	got := kbaVariants(doc, "9908", "AAA")
	if len(got) != 1 || got[0].Source != "sv42" || got[0].TradeName != "PASSAT VARIANT" || *got[0].PowerKW != 103 ||
		*got[0].DisplacementCC != 1968 || *got[0].FuelType != "diesel" || got[0].BodyCode != "AC" || *got[0].YearFrom != 2011 {
		t.Errorf("SV 4.2 row must replace FZ 6 row: %+v", got)
	}
	if tesla := kbaVariants(doc, "9909", "ABC"); len(tesla) != 1 || tesla[0].DisplacementCC != nil || *tesla[0].FuelType != "electric" {
		t.Errorf("electric %+v", tesla)
	}
	if y := kbaVariants(doc, "9909", "ABD"); len(y) != 1 || *y[0].FuelType != "other" || !strings.Contains(out.String(), "SV42 0099") {
		t.Errorf("unknown fuel code %+v\n%s", y, out.String())
	}
	if len(doc.Datasets) != 3 || doc.Datasets[2].ID != "kba-sv42" {
		t.Errorf("datasets %+v", doc.Datasets)
	}
}

func TestCleanEngine(t *testing.T) {
	for in, want := range map[string]string{
		"220 CDI 5AT RWD (143 HP)":                        "220 CDI 5AT RWD",
		"1.8L 5MT FWD 140 HP)":                            "1.8L 5MT FWD",
		"93.4 KWh (475 HP))":                              "93.4 KWh",
		"45 TDI V6 MHEV quattro 8AT AWD (231 HP) 7 seats": "45 TDI V6 MHEV quattro 8AT AWD 7 seats",
		"M40i xDrive 8AT AWD (382 HP) (US)":               "M40i xDrive 8AT AWD (US)",
		"8.0 W16 (1,500 HP)":                              "8.0 W16",
		"2.0 TDI (150 BHP)":                               "2.0 TDI",
		"HPX 2.0":                                         "HPX 2.0",
	} {
		if got := cleanEngine(in); got != want {
			t.Errorf("cleanEngine(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildRequiresInputs(t *testing.T) {
	if _, _, err := Build(Inputs{}); err == nil {
		t.Fatal("missing inputs accepted")
	}
}
