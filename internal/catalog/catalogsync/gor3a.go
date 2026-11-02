package catalogsync

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/app-torq/torq/internal/catalog"
)

// gor3a/vehicle-makes-models data/json/all.json (research C2).
type g3Group struct {
	Makes []g3Make `json:"makes"`
}

type g3Make struct {
	Name   string    `json:"name"`
	Models []g3Model `json:"models"`
}

type g3Model struct {
	Name        string `json:"name"`
	Generations []struct {
		Name      string     `json:"name"`
		YearStart *int64     `json:"yearStart"`
		YearEnd   *int64     `json:"yearEnd"`
		BodyType  *string    `json:"bodyType"`
		Engines   []g3Engine `json:"engines"`
	} `json:"generations"`
}

type g3Engine struct {
	Label          string   `json:"label"`
	FuelType       *string  `json:"fuelType"`
	DisplacementCc *float64 `json:"displacementCc"`
	PowerHp        *float64 `json:"powerHp"`
	// Specs holds only the field torq reads; the source's other specs are
	// skipped while decoding so a full update stays small in memory.
	Specs struct {
		Power string `json:"ENGINE SPECS / Power"`
	} `json:"specs"`
}

var (
	// hpRe matches horsepower figures anywhere in a label ("(143 HP)", "140 HP)",
	// "(1,500 HP)"); torq shows power only as kW (metric only).
	hpRe         = regexp.MustCompile(`\s*\(?\s*\d[\d.,]*\s*(?i:b?hp)\b\s*\)*`)
	hpPrefixRe   = regexp.MustCompile(`\s*\(\s*(?i:b?hp)(\s*\d[\d.,]*)?\s*\)`) // "(HP 68)", "(HP)"
	spacesRe     = regexp.MustCompile(`\s{2,}`)
	yearSuffixRe = regexp.MustCompile(`\s*\(\d{4}\)\s*$`)
	kwRe         = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*KW\b`)
)

// gor3aFuel maps the source fuel names to torq's fuel types.
var gor3aFuel = map[string]string{
	"Gasoline": "petrol", "Diesel": "diesel", "Electric": "electric",
	"Hybrid": "hybrid", "Mild Hybrid": "hybrid", "Hybrid Gasoline": "hybrid", "Plug-in Hybrid": "hybrid",
	"Mild Hybrid Diesel": "hybrid", "Hybrid Diesel": "hybrid",
	"Natural Gas": "cng", "Liquefied Petroleum Gas (LPG)": "lpg",
	"Ethanol": "other", "Hydrogen Fuel Cell": "other",
}

// readGor3a streams the source array one make group at a time, so the
// download never has to be held in memory as a whole.
func readGor3a(r io.Reader, rep *Report) ([]catalog.VehicleRow, error) {
	dec := json.NewDecoder(r)
	if tok, err := dec.Token(); err != nil || tok != json.Delim('[') {
		return nil, fmt.Errorf("gor3a: expected a JSON array: %v", err)
	}
	seen := map[string]bool{}
	var rows []catalog.VehicleRow
	for dec.More() {
		var g g3Group
		if err := dec.Decode(&g); err != nil {
			return nil, fmt.Errorf("gor3a: %w", err)
		}
		for _, mk := range g.Makes {
			for _, mo := range mk.Models {
				for _, gen := range mo.Generations {
					tax := classify(mk.Name, mo.Name, gen.Name, gen.YearStart, gen.YearEnd, rep)
					for _, e := range gen.Engines {
						row := catalog.VehicleRow{
							Make: mk.Name, Model: tax.model.en, ModelDE: deOnly(tax.model), Generation: tax.generation.en,
							GenerationDE: deOnly(tax.generation), YearFrom: gen.YearStart, YearTo: gen.YearEnd,
							Body: tax.body, Engine: cleanEngine(e.Label),
							DisplacementCC: displacement(e), PowerKW: powerKW(e), FuelType: fuel(e, rep),
						}
						if row.FuelType != nil && *row.FuelType == "electric" {
							row.DisplacementCC = nil
						}
						if catalog.Key(row.Model) == "" || catalog.Key(row.Engine) == "" {
							rep.rejected++
							continue
						}
						k := fmt.Sprint(catalog.Key(row.Make), "|", catalog.Key(row.Model), "|", catalog.Key(row.Generation), "|",
							catalog.Key(row.Engine), "|", ptr(row.DisplacementCC), "|", ptr(row.PowerKW), "|", sptr(row.FuelType))
						if seen[k] {
							continue
						}
						seen[k] = true
						rows = append(rows, row)
					}
				}
			}
		}
	}
	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("gor3a: %w", err)
	}
	return rows, nil
}

// cleanEngine removes horsepower figures from an engine label.
func cleanEngine(label string) string {
	s := hpPrefixRe.ReplaceAllString(hpRe.ReplaceAllString(label, " "), " ")
	return strings.TrimSpace(spacesRe.ReplaceAllString(s, " "))
}

// cleanModel removes the make prefix some makes carry ("MERCEDES BENZ
// C-Klasse") and the "and predecessors" suffix.
func cleanModel(make, model string) string {
	m := strings.TrimSpace(model)
	for _, p := range []string{make, strings.ReplaceAll(make, "-", " ")} {
		if len(m) > len(p) && strings.EqualFold(m[:len(p)], p) && m[len(p)] == ' ' {
			m = strings.TrimSpace(m[len(p):])
			break
		}
	}
	m = strings.TrimSpace(strings.TrimSuffix(m, " and predecessors"))
	return m
}

func displacement(e g3Engine) *int64 {
	if e.DisplacementCc == nil || *e.DisplacementCc < 1 || *e.DisplacementCc > 20000 {
		return nil
	}
	v := int64(math.Round(*e.DisplacementCc))
	return &v
}

// powerKW prefers the source's kW figure and falls back to its metric
// horsepower (PS) value; only whole kW are kept (research C2, metric only).
func powerKW(e g3Engine) *int64 {
	var kw float64
	if m := kwRe.FindStringSubmatch(e.Specs.Power); m != nil {
		kw, _ = strconv.ParseFloat(m[1], 64)
	} else if e.PowerHp != nil {
		kw = *e.PowerHp * 0.73549875
	}
	v := int64(math.Round(kw))
	if v < 1 || v > 2000 {
		return nil
	}
	return &v
}

func fuel(e g3Engine, rep *Report) *string {
	if e.FuelType == nil {
		return nil
	}
	f, ok := gor3aFuel[*e.FuelType]
	if !ok {
		rep.unknownFuel[*e.FuelType]++
		f = "other"
	}
	return &f
}

func ptr(p *int64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(*p, 10)
}

func sptr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
