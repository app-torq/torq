package catalogsync

import (
	"cmp"
	_ "embed"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/app-torq/torq/internal/catalog"
)

// Motorcycles come from the RDW open data (Public Domain, Constitution VII):
// an aggregate of the register of registered vehicles grouped by make, trade
// name, displacement and power-to-mass ratio, and the exact power and fuel of
// one sample registration per group from the fuel dataset (research M2).
// Both are read as CSV: RDW's JSON output has invalid escapes in some names.

// Thresholds of the clean-up (research M4).
const (
	rdwMinMakeRegistrations  = 50 // makes with fewer registrations are left out
	rdwMinModelRegistrations = 5  // models with fewer registrations are left out
	rdwMinGroupRegistrations = 2  // single registrations make no version
)

// rdwGroup is one row of the aggregate query.
type rdwGroup struct {
	Make, TradeName string
	CC              *int64
	Ratio           string // power-to-mass ratio; only separates versions
	FromYear        int64
	ToYear          int64
	Count           int64
	Sample          string // registration (kenteken) of one vehicle of the group
}

// rdwFuel is the fuel and power of one sample registration.
type rdwFuel struct {
	Fuel string // catalog.FuelTypes value
	KW   *int64
}

// rdwCandidate is a group after the make/model clean-up.
type rdwCandidate struct {
	rdwGroup
	MakeName, ModelName string // display names
	makeKey, modelKey   string
}

// RDWAggregateQuery and RDWFuelQuery are the SODA queries used by the build
// and the online update; the fuel query gets a "kenteken in (...)" filter.
const (
	RDWVehiclesDataset = "m9d7-ebf2"
	RDWFuelDataset     = "8ys7-d773"
	RDWAggregateSelect = "merk,handelsbenaming,cilinderinhoud,vermogen_massarijklaar," +
		"min(datum_eerste_toelating) as y0,max(datum_eerste_toelating) as y1,count(*) as n,min(kenteken) as k"
	RDWAggregateWhere = "voertuigsoort='Motorfiets'"
	RDWAggregateGroup = "merk,handelsbenaming,cilinderinhoud,vermogen_massarijklaar"
	RDWFuelSelect     = "kenteken,brandstof_omschrijving,nettomaximumvermogen"
)

// readCSV reads a SODA CSV export into maps keyed by column name; want lists
// the columns that must be present.
func readCSV(r io.Reader, want ...string) ([]map[string]string, error) {
	cr := csv.NewReader(r)
	cr.ReuseRecord = false
	head, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("RDW csv header: %w", err)
	}
	for _, w := range want {
		if !slices.Contains(head, w) {
			return nil, fmt.Errorf("RDW csv: column %q missing", w)
		}
	}
	var out []map[string]string
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("RDW csv: %w", err)
		}
		row := make(map[string]string, len(head))
		for i, h := range head {
			if i < len(rec) {
				row[h] = strings.TrimSpace(rec[i])
			}
		}
		out = append(out, row)
	}
}

// readRDWAggregate parses the aggregate export; it may be several pages
// concatenated, each with its header.
func readRDWAggregate(r io.Reader) ([]rdwGroup, error) {
	rows, err := readCSV(r, "merk", "handelsbenaming", "cilinderinhoud", "vermogen_massarijklaar", "y0", "y1", "n", "k")
	if err != nil {
		return nil, err
	}
	out := make([]rdwGroup, 0, len(rows))
	for _, row := range rows {
		if row["merk"] == "merk" { // header of a following page
			continue
		}
		n, err := strconv.ParseInt(row["n"], 10, 64)
		if err != nil || n < 1 {
			continue
		}
		g := rdwGroup{Make: row["merk"], TradeName: row["handelsbenaming"], Ratio: row["vermogen_massarijklaar"],
			FromYear: rdwYear(row["y0"]), ToYear: rdwYear(row["y1"]), Count: n, Sample: row["k"]}
		if cc, err := strconv.ParseInt(row["cilinderinhoud"], 10, 64); err == nil && cc > 0 && cc <= 20000 {
			g.CC = &cc
		}
		out = append(out, g)
	}
	return out, nil
}

// rdwYear takes the year of a YYYYMMDD date; 0 when unknown.
func rdwYear(s string) int64 {
	if len(s) < 4 {
		return 0
	}
	y, err := strconv.ParseInt(s[:4], 10, 64)
	if err != nil || y < 1885 || y > 2200 {
		return 0
	}
	return y
}

var rdwFuelTypes = map[string]string{
	"Benzine": "petrol", "Diesel": "diesel", "Elektriciteit": "electric", "LPG": "lpg", "CNG": "cng",
}

// readRDWFuel parses the fuel export. A registration with petrol and
// electricity is a hybrid; its power is the larger value.
func readRDWFuel(r io.Reader) (map[string]rdwFuel, error) {
	rows, err := readCSV(r, "kenteken", "brandstof_omschrijving", "nettomaximumvermogen")
	if err != nil {
		return nil, err
	}
	out := map[string]rdwFuel{}
	for _, row := range rows {
		k := row["kenteken"]
		if k == "" || k == "kenteken" {
			continue
		}
		fuel := cmp.Or(rdwFuelTypes[row["brandstof_omschrijving"]], "other")
		cur, seen := out[k]
		switch {
		case !seen:
			cur.Fuel = fuel
		case cur.Fuel != fuel && (cur.Fuel == "electric" || fuel == "electric"):
			cur.Fuel = "hybrid"
		}
		if f, err := strconv.ParseFloat(row["nettomaximumvermogen"], 64); err == nil && f >= 1 && f <= 2000 {
			kw := int64(math.Round(f))
			if cur.KW == nil || kw > *cur.KW {
				cur.KW = &kw
			}
		}
		out[k] = cur
	}
	return out, nil
}

// --- Make and model clean-up -------------------------------------------------

// rdwMakeAliases maps make keys (catalog.Key of RDW spellings) to the key of
// the make they belong to.
var rdwMakeAliases = map[string]string{
	"zuendapp": "zundapp", "indianmotorcycle": "indian", "zeromotorcycles": "zero", "enfield": "royalenfield",
	"motoguzziofpiaggio&c": "motoguzzi", "motoguzzidipiaggio&c": "motoguzzi", "hyosungmotors&machinery": "hyosung",
	"victorymotorcyclesdivis": "victory", "fanticmotor": "fantic", "aermacchihdavidson": "aermacchi",
}

// rdwMakeNames are display names where the default capitalization is wrong.
var rdwMakeNames = map[string]string{
	"harleydavidson": "Harley-Davidson", "motoguzzi": "Moto Guzzi", "mvagusta": "MV Agusta", "zundapp": "Zündapp",
	"cfmoto": "CFMOTO", "kymco": "KYMCO", "qjmotor": "QJMOTOR", "gasgas": "GASGAS", "dkw": "DKW", "nsu": "NSU",
	"ajs": "AJS", "swm": "SWM", "fn": "FN", "twn": "TWN", "tmracing": "TM Racing", "ktm": "KTM", "bmw": "BMW",
	"bsa": "BSA", "sym": "SYM", "niu": "NIU", "mbk": "MBK", "ajp": "AJP", "livewire": "LiveWire", "surron": "Sur-Ron",
	"supersoco": "Super Soco", "royalenfield": "Royal Enfield", "zero": "Zero", "indian": "Indian",
}

// rdwSkipMakes are not makes: self-built vehicles.
var rdwSkipMakes = map[string]bool{"eigenbouw": true, "zelfbouw": true, "onbekend": true}

func rdwMakeKey(raw string) string {
	k := catalog.Key(raw)
	if a, ok := rdwMakeAliases[k]; ok {
		return a
	}
	return k
}

// rdwTitleWords are short words written as words, not as abbreviations.
var rdwTitleWords = map[string]bool{
	"FAT": true, "BOY": true, "BOB": true, "LOW": true, "ROD": true, "RUN": true, "BIG": true, "DOG": true,
	"RED": true, "SKY": true, "PAN": true, "AIR": true, "ONE": true, "TWO": true, "SIX": true, "NEW": true,
	"OLD": true, "PRO": true, "EVO": true, "ACE": true, "CUP": true, "VAN": true, "MAX": true, "SUN": true,
}

// harleyCodeRe matches Harley-Davidson model family codes (FLSTF, FXDB, XL).
var harleyCodeRe = regexp.MustCompile(`^(FL|FX|XL|XG|XR|VRSC|RA|RH)[A-Z]{0,5}$`)

// rdwWord capitalizes one word of a model name (research M4).
func rdwWord(w string, harley bool) string {
	var parts []string
	for p := range strings.SplitSeq(w, "-") {
		parts = append(parts, rdwPart(p, harley))
	}
	return strings.Join(parts, "-")
}

func rdwPart(p string, harley bool) string {
	upper := strings.ToUpper(p)
	letters, vowels, digits := 0, 0, 0
	for _, r := range upper {
		switch {
		case unicode.IsDigit(r):
			digits++
		case unicode.IsLetter(r):
			letters++
			if strings.ContainsRune("AEIOUYÄÖÜÉ", r) {
				vowels++
			}
		}
	}
	switch {
	case letters == 0, digits > 0, vowels == 0, harley && harleyCodeRe.MatchString(upper):
		return upper
	case letters <= 3 && !rdwTitleWords[upper]:
		return upper
	}
	r := []rune(strings.ToLower(p))
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

var (
	rdwPowerSuffixRe = regexp.MustCompile(`(?i)\(\s*\d+(?:[.,]\d+)?\s*kw\s*\)`)
	rdwSpacesRe      = regexp.MustCompile(`\s+`)
)

// rdwTidyModel cleans a trade name: no make prefix, no power suffix, the
// first of a comma list, no trailing ABS, capitalized words.
func rdwTidyModel(makeKey, rawMake, trade string) string {
	t := rdwPowerSuffixRe.ReplaceAllString(trade, " ")
	if i := strings.IndexByte(t, ','); i >= 0 {
		t = t[:i]
	}
	words := strings.Fields(rdwSpacesRe.ReplaceAllString(t, " "))
	// Drop leading words that spell the make ("HARLEY DAVIDSON FAT BOY").
	for _, mk := range []string{makeKey, catalog.Key(rawMake)} {
		acc := ""
		for i, w := range words {
			acc += catalog.Key(w)
			if acc == mk {
				words = words[i+1:]
				break
			}
			if !strings.HasPrefix(mk, acc) {
				break
			}
		}
	}
	for len(words) > 0 && strings.EqualFold(words[len(words)-1], "ABS") {
		words = words[:len(words)-1]
	}
	harley := makeKey == "harleydavidson"
	for i, w := range words {
		words[i] = rdwWord(strings.Trim(w, "-"), harley)
	}
	return strings.TrimSpace(strings.Join(words, " "))
}

// rdwMakeName is the display name of a make key, from the most registered
// raw spelling unless rdwMakeNames knows better.
func rdwMakeName(key, raw string) string {
	if n, ok := rdwMakeNames[key]; ok {
		return n
	}
	var words []string
	for w := range strings.FieldsSeq(strings.ReplaceAll(raw, ".", "")) {
		words = append(words, rdwWord(w, false))
	}
	return strings.Join(words, " ")
}

// prepareMotorcycles applies the make and model clean-up and the thresholds
// to the aggregate. Every group of a surviving model is kept, so the
// registration counts stay complete; rdwSamples picks the ones that become
// versions.
func prepareMotorcycles(groups []rdwGroup, ov motorcycleOverrides, rep *Report) []rdwCandidate {
	makeCount := map[string]int64{}
	makeSpelling := map[string]map[string]int64{}
	for _, g := range groups {
		k := rdwMakeKey(g.Make)
		makeCount[k] += g.Count
		if makeSpelling[k] == nil {
			makeSpelling[k] = map[string]int64{}
		}
		makeSpelling[k][g.Make] += g.Count
	}
	var cands []rdwCandidate
	modelCount := map[[2]string]int64{}
	modelSpelling := map[[2]string]map[string]int64{}
	for _, g := range groups {
		mk := rdwMakeKey(g.Make)
		if rdwSkipMakes[mk] || makeCount[mk] < rdwMinMakeRegistrations {
			continue
		}
		model := rdwTidyModel(mk, g.Make, g.TradeName)
		if o, ok := ov.lookup(mk, model); ok {
			if o == "" {
				rep.rdwDropped++
				continue
			}
			model = o
		}
		if catalog.Key(model) == "" {
			continue
		}
		c := rdwCandidate{rdwGroup: g, makeKey: mk, modelKey: catalog.Key(model), ModelName: model}
		key := [2]string{mk, c.modelKey}
		modelCount[key] += g.Count
		if modelSpelling[key] == nil {
			modelSpelling[key] = map[string]int64{}
		}
		modelSpelling[key][model] += g.Count
		cands = append(cands, c)
	}
	out := cands[:0]
	reported := map[[2]string]bool{}
	for _, c := range cands {
		key := [2]string{c.makeKey, c.modelKey}
		if modelCount[key] < rdwMinModelRegistrations {
			continue
		}
		c.MakeName = rdwMakeName(c.makeKey, mostCommon(makeSpelling[c.makeKey]))
		c.ModelName = mostCommon(modelSpelling[key])
		if !reported[key] {
			reported[key] = true
			rep.rdwModels = append(rep.rdwModels, rdwModelStat{Make: c.MakeName, Model: c.ModelName, Count: modelCount[key]})
		}
		out = append(out, c)
	}
	return out
}

// mostCommon returns the key with the highest count (ties: alphabetical).
func mostCommon(m map[string]int64) string {
	best, n := "", int64(-1)
	for _, k := range slices.Sorted(maps.Keys(m)) {
		if m[k] > n {
			best, n = k, m[k]
		}
	}
	return best
}

// rdwSamples lists the sample registrations whose power and fuel the build
// needs: one per group with at least rdwMinGroupRegistrations vehicles.
func rdwSamples(cands []rdwCandidate) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range cands {
		if c.Count >= rdwMinGroupRegistrations && c.Sample != "" && !seen[c.Sample] {
			seen[c.Sample] = true
			out = append(out, c.Sample)
		}
	}
	slices.Sort(out)
	return out
}

// buildMotorcycles merges the candidate groups into versions: one row per
// make, model, displacement, power and fuel, with the years of first
// registration of all its groups (research M5).
func buildMotorcycles(cands []rdwCandidate, fuel map[string]rdwFuel) []catalog.VehicleRow {
	type versionKey struct {
		makeKey, modelKey string
		cc, kw            int64
		fuel              string
	}
	type version struct {
		row   catalog.VehicleRow
		count int64
	}
	versions := map[versionKey]*version{}
	var order []versionKey
	for _, c := range cands {
		if c.Count < rdwMinGroupRegistrations {
			continue
		}
		f := fuel[c.Sample]
		k := versionKey{makeKey: c.makeKey, modelKey: c.modelKey, cc: ptrOr(c.CC), kw: ptrOr(f.KW), fuel: f.Fuel}
		v, ok := versions[k]
		if !ok {
			row := catalog.VehicleRow{Make: c.MakeName, Model: c.ModelName, VehicleType: catalog.TypeMotorcycle,
				DisplacementCC: c.CC, PowerKW: f.KW}
			if f.Fuel != "" {
				fuelType := f.Fuel
				row.FuelType = &fuelType
			}
			if row.DisplacementCC != nil && f.Fuel == "electric" {
				row.DisplacementCC = nil
			}
			row.Engine = versionLabel(row)
			v = &version{row: row}
			versions[k] = v
			order = append(order, k)
		}
		v.count += c.Count
		v.row.YearFrom = minYear(v.row.YearFrom, c.FromYear)
		v.row.YearTo = maxYear(v.row.YearTo, c.ToYear)
	}
	out := make([]catalog.VehicleRow, 0, len(order))
	for _, k := range order {
		out = append(out, versions[k].row)
	}
	return out
}

// versionLabel is the searchable engine name of a version, e.g.
// "821 cm³ 82 kW"; the web app shows the numbers in the user's units.
func versionLabel(r catalog.VehicleRow) string {
	var parts []string
	if r.DisplacementCC != nil {
		parts = append(parts, fmt.Sprintf("%d cm³", *r.DisplacementCC))
	}
	if r.PowerKW != nil {
		parts = append(parts, fmt.Sprintf("%d kW", *r.PowerKW))
	}
	if len(parts) == 0 {
		return r.Model
	}
	return strings.Join(parts, " ")
}

func ptrOr(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func minYear(cur *int64, y int64) *int64 {
	if y == 0 || (cur != nil && *cur <= y) {
		return cur
	}
	return &y
}

func maxYear(cur *int64, y int64) *int64 {
	if y == 0 || (cur != nil && *cur >= y) {
		return cur
	}
	return &y
}

// --- Curated overrides (motorcycles.txt) ---------------------------------

//go:embed motorcycles.txt
var motorcyclesTxt string

var motorcycleRules = mustParseOverrides(motorcyclesTxt)

// motorcycleOverrides maps make key → tidied model key → display name; an
// empty name drops the model.
type motorcycleOverrides map[string]map[string]string

func (o motorcycleOverrides) lookup(makeKey, model string) (string, bool) {
	n, ok := o[makeKey][catalog.Key(model)]
	return n, ok
}

func mustParseOverrides(src string) motorcycleOverrides {
	o, err := parseOverrides(src)
	if err != nil {
		panic(err)
	}
	return o
}

// parseOverrides reads "Make | model | display name" lines; "-" as the
// display name drops the model. # starts a comment.
func parseOverrides(src string) (motorcycleOverrides, error) {
	o := motorcycleOverrides{}
	for i, line := range strings.Split(src, "\n") {
		if j := strings.IndexByte(line, '#'); j >= 0 {
			line = line[:j]
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(line, "|")
		if len(f) != 3 {
			return nil, fmt.Errorf("motorcycles.txt:%d: want 3 fields", i+1)
		}
		mk, model, name := rdwMakeKey(strings.TrimSpace(f[0])), catalog.Key(strings.TrimSpace(f[1])), strings.TrimSpace(f[2])
		if mk == "" || model == "" || name == "" {
			return nil, fmt.Errorf("motorcycles.txt:%d: empty field", i+1)
		}
		if name == "-" {
			name = ""
		}
		if o[mk] == nil {
			o[mk] = map[string]string{}
		}
		if _, dup := o[mk][model]; dup {
			return nil, fmt.Errorf("motorcycles.txt:%d: duplicate %s %s", i+1, f[0], f[1])
		}
		o[mk][model] = name
	}
	return o, nil
}
