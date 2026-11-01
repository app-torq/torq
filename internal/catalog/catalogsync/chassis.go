package catalogsync

import (
	"cmp"
	_ "embed"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/marekbeckmann/torq/internal/catalog"
)

// chassisTxt maps make + model (+ body) + production years to chassis codes.
// The sources carry no chassis codes, only year ranges; see the file header
// for the format.
//
//go:embed chassis.txt
var chassisTxt string

var chassis = mustParseChassis(chassisTxt)

type chassisGen struct {
	code     string
	from, to int64 // to 0: still in production
}

// chassisTable is keyed by "makeKey|modelKey", then by body token ("" is the
// default for every body without its own line).
type chassisTable map[string]map[string][]chassisGen

// chassisMakeAliases share another make's lines.
var chassisMakeAliases = map[string]string{catalog.Key("Mercedes-AMG"): catalog.Key("Mercedes-Benz")}

var chassisGenRe = regexp.MustCompile(`^(.*\S)\s+(\d{4})-(\d{4})?$`)

func mustParseChassis(src string) chassisTable {
	t, err := parseChassis(src)
	if err != nil {
		panic(err)
	}
	return t
}

func parseChassis(src string) (chassisTable, error) {
	t := chassisTable{}
	for i, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "|")
		if len(f) != 4 {
			return nil, fmt.Errorf("chassis.txt:%d: want 4 fields", i+1)
		}
		make, body := strings.TrimSpace(f[0]), strings.TrimSpace(f[2])
		if body != "" && !slices.Contains(catalog.Bodies, body) {
			return nil, fmt.Errorf("chassis.txt:%d: unknown body %q", i+1, body)
		}
		var gens []chassisGen
		if g := strings.TrimSpace(f[3]); g != "-" {
			for _, entry := range strings.Split(g, ",") {
				m := chassisGenRe.FindStringSubmatch(strings.TrimSpace(entry))
				if m == nil {
					return nil, fmt.Errorf("chassis.txt:%d: bad generation %q", i+1, entry)
				}
				from, _ := strconv.ParseInt(m[2], 10, 64)
				to, _ := strconv.ParseInt(cmp.Or(m[3], "0"), 10, 64)
				if to != 0 && to < from {
					return nil, fmt.Errorf("chassis.txt:%d: %q ends before it starts", i+1, entry)
				}
				gens = append(gens, chassisGen{code: m[1], from: from, to: to})
			}
		}
		for _, model := range strings.Split(f[1], ",") {
			k := catalog.Key(make) + "|" + catalog.Key(strings.TrimSpace(model))
			if t[k] == nil {
				t[k] = map[string][]chassisGen{}
			}
			if _, dup := t[k][body]; dup {
				return nil, fmt.Errorf("chassis.txt:%d: duplicate %s %q", i+1, model, body)
			}
			t[k][body] = gens
		}
	}
	return t, nil
}

// lookup returns the chassis code of a generation that started in year. The
// longest leading part of the model name with lines wins, so "911 Carrera
// 4S" uses the "911 Carrera" line and "Golf GTD Variant" the "Golf" estate
// line. Within a line the generation that started last, but not after year,
// is chosen, provided it was still built in year.
func (t chassisTable) lookup(make, model, body string, year *int64) (string, bool) {
	if year == nil {
		return "", false
	}
	mk := catalog.Key(make)
	if a, ok := chassisMakeAliases[mk]; ok {
		mk = a
	}
	words := strings.Fields(model)
	for n := len(words); n > 0; n-- {
		bodies, ok := t[mk+"|"+catalog.Key(strings.Join(words[:n], " "))]
		if !ok {
			continue
		}
		gens, ok := bodies[body]
		if !ok {
			gens = bodies[""]
		}
		return pickChassis(gens, *year)
	}
	return "", false
}

// hasMake reports whether any line covers the make key.
func (t chassisTable) hasMake(makeKey string) bool {
	for k := range t {
		if strings.HasPrefix(k, makeKey+"|") {
			return true
		}
	}
	return false
}

func pickChassis(gens []chassisGen, year int64) (string, bool) {
	best := -1
	for i, g := range gens {
		if g.from <= year && (best < 0 || g.from > gens[best].from) {
			best = i
		}
	}
	if best < 0 || (gens[best].to != 0 && year > gens[best].to) {
		return "", false
	}
	return gens[best].code, true
}
