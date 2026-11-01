package i18n

import (
	"sort"
	"testing"
)

func TestCatalogParity(t *testing.T) {
	en, de := Keys("en"), Keys("de")
	sort.Strings(en)
	sort.Strings(de)
	if len(en) != len(de) {
		t.Fatalf("en has %d keys, de has %d", len(en), len(de))
	}
	for i := range en {
		if en[i] != de[i] {
			t.Fatalf("key mismatch: %s vs %s", en[i], de[i])
		}
	}
}

func TestFormatting(t *testing.T) {
	de := Locale{Lang: "de", DateFormat: "DD.MM.YYYY", NumberFormat: "comma"}
	if got := de.Km(215000); got != "215.000 km" {
		t.Fatalf("km %q", got)
	}
	if got := de.Date("2026-10-15"); got != "15.10.2026" {
		t.Fatalf("date %q", got)
	}
	if got := Default.Km(215000); got != "215,000 km" {
		t.Fatalf("km %q", got)
	}
	if got := de.T("notify.overdue.title", map[string]string{"reminder": "Öl", "vehicle": "S203"}); got != "Überfällig: Öl — S203" {
		t.Fatalf("t %q", got)
	}
	if got := (Locale{Lang: "fr"}).T("notify.test.title", nil); got != "torq test notification" {
		t.Fatalf("fallback %q", got)
	}
}
