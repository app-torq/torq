package catalogsync

import (
	"cmp"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/app-torq/torq/internal/catalog"
)

// Report collects what the maintainer must review before committing a seed.
type Report struct {
	unmapped    map[string]int64
	unknownFuel map[string]int
	chassisHits map[string][2]int // make → generations with a chassis code, all generations
	rejected    int
	matched     int
	unmatched   int
	// RDW: models dropped by motorcycles.txt, and registrations per
	// make/model key that passed the make threshold.
	rdwDropped int
	rdwModels  []rdwModelStat
}

// rdwModelStat is one motorcycle model with its registrations.
type rdwModelStat struct {
	Make, Model string
	Count       int64
}

func newReport() *Report {
	return &Report{unmapped: map[string]int64{}, unknownFuel: map[string]int{}, chassisHits: map[string][2]int{}}
}

// chassis counts a source generation of a make chassis.txt covers.
func (r *Report) chassis(make string, found bool) {
	mk := catalog.Key(make)
	if a, ok := chassisMakeAliases[mk]; ok {
		mk = a
	}
	if !chassis.hasMake(mk) {
		return
	}
	c := r.chassisHits[make]
	if found {
		c[0]++
	}
	c[1]++
	r.chassisHits[make] = c
}

// Print writes the review summary for a seed of size bytes.
func (r *Report) Print(w io.Writer, doc catalog.SeedDoc, size int) error {
	var b strings.Builder
	makes := map[string]bool{}
	for _, v := range doc.Vehicles {
		makes[v.Make] = true
	}
	kbaMakes := map[string]bool{}
	for _, k := range doc.KBA {
		kbaMakes[k.Make] = true
	}
	fmt.Fprintf(&b, "seed: %d bytes, %d KBA rows (%d makes), %d taxonomy rows (%d makes)\n",
		size, len(doc.KBA), len(kbaMakes), len(doc.Vehicles), len(makes))
	fmt.Fprintf(&b, "KBA model match: %d unique, %d none\n", r.matched, r.unmatched)
	fmt.Fprintf(&b, "rejected source rows: %d\n", r.rejected)
	if len(r.chassisHits) > 0 {
		fmt.Fprintln(&b, "generations with a chassis code (extend chassis.txt for the rest):")
		for _, mk := range slices.Sorted(maps.Keys(r.chassisHits)) {
			fmt.Fprintf(&b, "  %-16s %d of %d\n", mk, r.chassisHits[mk][0], r.chassisHits[mk][1])
		}
	}
	if len(r.unknownFuel) > 0 {
		fmt.Fprintln(&b, "unknown fuel codes (mapped to other):")
		for _, k := range slices.Sorted(maps.Keys(r.unknownFuel)) {
			fmt.Fprintf(&b, "  %s: %d\n", k, r.unknownFuel[k])
		}
	}
	names := slices.SortedFunc(maps.Keys(r.unmapped), func(a, b string) int {
		return cmp.Or(cmp.Compare(r.unmapped[b], r.unmapped[a]), cmp.Compare(a, b))
	})
	if len(names) > 40 {
		names = names[:40]
	}
	fmt.Fprintln(&b, "largest unmapped KBA manufacturers (vehicle stock; extend makes.go if any is a car make):")
	for _, n := range names {
		fmt.Fprintf(&b, "  %-28s %d\n", n, r.unmapped[n])
	}
	r.printMotorcycles(&b, doc)
	_, err := io.WriteString(w, b.String())
	return err
}

// reportMakes are the makes issue #28 names; the report lists their models
// so the maintainer can check SC-001 (each spelled once).
var reportMakes = []string{"Harley-Davidson", "Honda", "Yamaha", "Suzuki", "Kawasaki", "BMW", "KTM", "Ducati"}

// codeLikeRe matches model names that are probably frame or type-approval
// codes rather than names ("RC46", "H3"); review them for motorcycles.txt.
var codeLikeRe = regexp.MustCompile(`^[A-Z]{1,3}\d{1,4}[A-Z]?$`)

func (r *Report) printMotorcycles(b *strings.Builder, doc catalog.SeedDoc) {
	models := slices.SortedFunc(slices.Values(r.rdwModels), func(x, y rdwModelStat) int {
		return cmp.Or(cmp.Compare(y.Count, x.Count), cmp.Compare(x.Make, y.Make), cmp.Compare(x.Model, y.Model))
	})
	fmt.Fprintf(b, "motorcycles: %d versions of %d models (%d left out by motorcycles.txt)\n",
		doc.Counts.Motorcycles, len(models), r.rdwDropped)
	for _, mk := range reportMakes {
		var top []string
		for _, m := range models {
			if m.Make == mk && len(top) < 15 {
				top = append(top, fmt.Sprintf("%s (%d)", m.Model, m.Count))
			}
		}
		fmt.Fprintf(b, "  %-16s %s\n", mk, strings.Join(top, ", "))
	}
	fmt.Fprintln(b, "most registered motorcycle names that look like codes (check the type approval, then motorcycles.txt):")
	n := 0
	for _, m := range models {
		if n < 30 && codeLikeRe.MatchString(m.Model) {
			fmt.Fprintf(b, "  %-16s %-12s %d\n", m.Make, m.Model, m.Count)
			n++
		}
	}
}
