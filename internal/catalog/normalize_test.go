package catalog_test

import (
	"testing"

	"github.com/marekbeckmann/torq/internal/catalog"
)

func TestKey(t *testing.T) {
	cases := map[string]string{
		"Mercedes-Benz":     "mercedesbenz",
		"mercedes benz":     "mercedesbenz",
		"Škoda":             "skoda",
		"Citroën":           "citroen",
		"Straße":            "strasse",
		"W203 / S203":       "w203s203",
		"C 220 CDI":         "c220cdi",
		"2.0 TDI":           "20tdi",
		"  ":                "",
		"":                  "",
		"Coupe (2016–2018)": "coupe(2016–2018)",
	}
	for in, want := range cases {
		if got := catalog.Key(in); got != want {
			t.Errorf("Key(%q) = %q, want %q", in, got, want)
		}
	}
}
