// Package i18n renders the few server-side strings (webhook notification text)
// in the user's language and number/date format (Constitution V, research R16).
package i18n

import (
	"embed"
	"encoding/json"
	"strings"
	"time"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
)

//go:embed en.json de.json
var files embed.FS

var catalogs = map[string]map[string]string{}

func init() {
	for _, lang := range []string{"en", "de"} {
		b, err := files.ReadFile(lang + ".json")
		if err != nil {
			panic(err)
		}
		m := map[string]string{}
		if err := json.Unmarshal(b, &m); err != nil {
			panic(err)
		}
		catalogs[lang] = m
	}
}

// Keys returns all keys of a locale (for tests).
func Keys(locale string) []string {
	out := make([]string, 0, len(catalogs[locale]))
	for k := range catalogs[locale] {
		out = append(out, k)
	}
	return out
}

// Locale formats text, numbers and dates for one user.
type Locale struct {
	Lang         string // "en" or "de"
	DateFormat   string // "DD.MM.YYYY" or "YYYY-MM-DD"
	NumberFormat string // "point" or "comma"
}

// Default is English with ISO dates and point decimals, used when no owner is known.
var Default = Locale{Lang: "en", DateFormat: "YYYY-MM-DD", NumberFormat: "point"}

// T translates key, substituting {name} placeholders. Missing keys fall back
// to English, then to the key itself.
func (l Locale) T(key string, params map[string]string) string {
	s, ok := catalogs[l.Lang][key]
	if !ok {
		if s, ok = catalogs["en"][key]; !ok {
			s = key
		}
	}
	for k, v := range params {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

func (l Locale) printer() *message.Printer {
	if l.NumberFormat == "comma" {
		return message.NewPrinter(language.German)
	}
	return message.NewPrinter(language.English)
}

// Number formats an integer with locale-aware grouping.
func (l Locale) Number(n int64) string {
	return l.printer().Sprint(number.Decimal(n))
}

// Km formats a distance, e.g. "215,000 km" or "215.000 km".
func (l Locale) Km(n int64) string { return l.Number(n) + " km" }

// Date formats a YYYY-MM-DD date per the chosen pattern.
func (l Locale) Date(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	if l.DateFormat == "DD.MM.YYYY" {
		return t.Format("02.01.2006")
	}
	return t.Format("2006-01-02")
}
