package repairs

import "strings"

// Search is a parsed project search: tags the projects must all have, and
// free text matched against titles, diagnostics, fault codes and parts.
type Search struct {
	Tags []string
	Text string
}

// ParseSearch splits a search box query such as
// `tags:brake-fluid,"brake discs" weak braking` into its parts. Only known
// keys (tags:, tag:) are taken out, so other words with a colon stay text and
// more keys can be added later. A key followed by a space takes the next word
// (`tags: abs`); double quotes keep spaces inside a tag or a phrase.
func ParseSearch(q string) Search {
	var out Search
	var text []string
	tokens := splitOutsideQuotes(q, isSpace)
	for i := 0; i < len(tokens); i++ {
		key, value, ok := strings.Cut(tokens[i], ":")
		if !ok || !isTagKey(key) {
			text = append(text, unquote(tokens[i]))
			continue
		}
		if value == "" && i+1 < len(tokens) {
			i++
			value = tokens[i]
		}
		for _, tag := range splitOutsideQuotes(value, func(r rune) bool { return r == ',' }) {
			if tag = normalizeTag(unquote(tag)); tag != "" {
				out.Tags = append(out.Tags, tag)
			}
		}
	}
	out.Text = strings.Join(text, " ")
	return out
}

func isTagKey(key string) bool {
	return strings.EqualFold(key, "tags") || strings.EqualFold(key, "tag")
}

func isSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }

// splitOutsideQuotes splits s at runes for which sep is true, except inside
// double quotes. Quotes stay in the parts; empty parts are dropped.
func splitOutsideQuotes(s string, sep func(rune) bool) []string {
	var parts []string
	var cur strings.Builder
	quoted := false
	for _, r := range s {
		switch {
		case r == '"':
			quoted = !quoted
			cur.WriteRune(r)
		case !quoted && sep(r):
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

func unquote(s string) string { return strings.ReplaceAll(s, `"`, "") }
