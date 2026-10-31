package main

import (
	"cmp"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/marekbeckmann/torq/internal/catalog"
)

// report collects what the maintainer must review before committing a seed.
type report struct {
	unmapped    map[string]int64
	unknownFuel map[string]int
	rejected    int
	matched     int
	unmatched   int
}

func newReport() *report {
	return &report{unmapped: map[string]int64{}, unknownFuel: map[string]int{}}
}

func (r *report) print(w io.Writer, doc catalog.SeedDoc, size int) error {
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
	_, err := io.WriteString(w, b.String())
	return err
}
