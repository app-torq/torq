package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/marekbeckmann/torq/internal/catalog"
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
	Label          string            `json:"label"`
	FuelType       *string           `json:"fuelType"`
	DisplacementCc *float64          `json:"displacementCc"`
	PowerHp        *float64          `json:"powerHp"`
	Specs          map[string]string `json:"specs"`
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

func readGor3a(path string, rep *report) ([]catalog.VehicleRow, error) {
	b, err := os.ReadFile(path) //nolint:gosec // maintainer-supplied input file
	if err != nil {
		return nil, err
	}
	var groups []g3Group
	if err := json.Unmarshal(b, &groups); err != nil {
		return nil, fmt.Errorf("gor3a: %w", err)
	}
	seen := map[string]bool{}
	var rows []catalog.VehicleRow
	for _, g := range groups {
		for _, mk := range g.Makes {
			for _, mo := range mk.Models {
				model := cleanModel(mk.Name, mo.Name)
				for _, gen := range mo.Generations {
					label := generationLabel(mk.Name, mo.Name, gen.Name, gen.YearStart, gen.YearEnd)
					body := ""
					if gen.BodyType != nil {
						body = strings.TrimSpace(*gen.BodyType)
					}
					for _, e := range gen.Engines {
						row := catalog.VehicleRow{
							Make: mk.Name, Model: model, Generation: label, YearFrom: gen.YearStart, YearTo: gen.YearEnd,
							Body: body, Engine: cleanEngine(e.Label),
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

// generationLabel turns "MERCEDES BENZ C-Klasse Coupe (2016)" into
// "Coupe (2016–2018)" and a bare "(2000)" into "2000–2004".
func generationLabel(make, rawModel, name string, from, to *int64) string {
	desc := strings.TrimSpace(name)
	if len(desc) >= len(rawModel) && strings.EqualFold(desc[:len(rawModel)], rawModel) {
		desc = desc[len(rawModel):]
	} else {
		desc = cleanModel(make, desc)
		if c := cleanModel(make, rawModel); len(desc) >= len(c) && strings.EqualFold(desc[:len(c)], c) {
			desc = desc[len(c):]
		}
	}
	desc = strings.TrimSpace(yearSuffixRe.ReplaceAllString(desc, ""))
	years := ""
	switch {
	case from != nil && to != nil && *to != *from:
		years = fmt.Sprintf("%d–%d", *from, *to)
	case from != nil && to != nil:
		years = strconv.FormatInt(*from, 10)
	case from != nil:
		years = fmt.Sprintf("%d–", *from)
	}
	switch {
	case desc == "":
		return years
	case years == "":
		return desc
	default:
		return desc + " (" + years + ")"
	}
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
	if m := kwRe.FindStringSubmatch(e.Specs["ENGINE SPECS / Power"]); m != nil {
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

func fuel(e g3Engine, rep *report) *string {
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
