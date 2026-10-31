package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/marekbeckmann/torq/internal/catalog"
)

// modelIndex maps make key → model key → display name from the taxonomy, so
// KBA trade names can be matched to taxonomy models (research C3).
type modelIndex map[string]map[string]string

func newModelIndex(rows []catalog.VehicleRow) modelIndex {
	idx := modelIndex{}
	for _, r := range rows {
		mk := catalog.Key(r.Make)
		if idx[mk] == nil {
			idx[mk] = map[string]string{}
		}
		if _, ok := idx[mk][catalog.Key(r.Model)]; !ok {
			idx[mk][catalog.Key(r.Model)] = r.Model
		}
	}
	return idx
}

var (
	bmwSeriesRe     = regexp.MustCompile(`^([1-8])(ER\b|\d{2}[A-Z]{0,2}\b)`)
	mercedesClassRe = regexp.MustCompile(`^([A-Z]{1,3}) ?\d{2,3}\b`)
)

// modelFor derives the model for a trade-name variant. It returns a taxonomy
// model name when exactly one model matches the variant's leading words, and
// otherwise the trade name without a leading brand word. An empty model means
// the KBA entry names only the brand (e.g. "TOYOTA").
func (idx modelIndex) modelFor(make, tradeName string) (model string, matched bool) {
	words := strings.Fields(tradeName)
	if len(words) > 0 {
		w0 := catalog.Key(words[0])
		if w0 == catalog.Key(make) || w0 == catalog.Key(strings.Fields(make)[0]) {
			words = words[1:]
		}
	}
	rest := strings.Join(words, " ")
	if rest == "" {
		return "", false
	}
	models := idx[catalog.Key(make)]
	switch make {
	case "BMW":
		if m := bmwSeriesRe.FindStringSubmatch(rest); m != nil {
			if name, ok := models[catalog.Key(m[1]+" Series")]; ok {
				return name, true
			}
		}
	case "Mercedes-Benz", "Mercedes-AMG":
		if m := mercedesClassRe.FindStringSubmatch(rest); m != nil {
			for _, cand := range []string{m[1] + "-Klasse", m[1] + "-Class", m[1]} {
				if name, ok := models[catalog.Key(cand)]; ok {
					return name, true
				}
			}
		}
	}
	if len(words) >= 2 {
		if name, ok := models[catalog.Key(words[0]+" "+words[1])]; ok {
			return name, true
		}
	}
	if name, ok := models[catalog.Key(words[0])]; ok {
		return name, true
	}
	return rest, false
}

func (idx modelIndex) String() string { return fmt.Sprintf("%d makes", len(idx)) }
