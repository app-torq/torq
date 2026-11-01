package catalog

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Key normalizes a catalog name for searching (research C5): accents and
// case are ignored, "ß" matches "ss", and spaces, hyphens, dots and slashes
// are dropped, so "mercedes benz" finds "Mercedes-Benz" and "citroen" finds
// "Citroën". The frontend mirrors this in web/src/lib/catalogKey.ts.
func Key(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range norm.NFKD.String(s) {
		switch {
		case unicode.Is(unicode.Mn, r), unicode.IsSpace(r), r == '-', r == '.', r == '/':
		case r == 'ß':
			b.WriteString("ss")
		default:
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}
