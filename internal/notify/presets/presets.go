package presets

import (
	"encoding/json"
	"fmt"
	"mime"
	"strings"
	"time"
	"unicode/utf8"
)

// Names of supported presets.
const (
	Discord       = "discord"
	Ntfy          = "ntfy"
	Gotify        = "gotify"
	HomeAssistant = "home_assistant"
	GenericJSON   = "generic_json"
)

// Valid reports whether name is a supported preset.
func Valid(name string) bool {
	switch name {
	case Discord, Ntfy, Gotify, HomeAssistant, GenericJSON:
		return true
	}
	return false
}

// Render produces the request for preset.
func Render(preset string, m Message) (Rendered, error) {
	switch preset {
	case Discord:
		return discord(m)
	case Ntfy:
		return ntfy(m), nil
	case Gotify:
		return gotify(m)
	case HomeAssistant, GenericJSON:
		return generic(m)
	default:
		return Rendered{}, fmt.Errorf("unknown preset %q", preset)
	}
}

func jsonRendered(v any) (Rendered, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return Rendered{}, err
	}
	return Rendered{Headers: map[string]string{"Content-Type": "application/json"}, Body: b}, nil
}

func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max-1]) + "…"
}

// --- Generic JSON / Home Assistant ---------------------------------------

type genericPayload struct {
	Schema   string    `json:"schema"`
	Event    string    `json:"event"`
	Severity string    `json:"severity"`
	SentAt   string    `json:"sent_at"`
	Title    string    `json:"title"`
	Message  string    `json:"message"`
	URL      string    `json:"url,omitempty"`
	Reminder *Reminder `json:"reminder,omitempty"`
	Vehicle  *Vehicle  `json:"vehicle,omitempty"`
	Digest   *Digest   `json:"digest,omitempty"`
}

func generic(m Message) (Rendered, error) {
	return jsonRendered(genericPayload{
		Schema: "torq.notification/v1", Event: m.Event, Severity: m.Severity,
		SentAt: m.SentAt.UTC().Format(time.RFC3339), Title: m.Title, Message: m.Body, URL: m.URL,
		Reminder: m.Reminder, Vehicle: m.Vehicle, Digest: m.Digest,
	})
}

// --- Discord ---------------------------------------------------------------

var discordColors = map[string]int{SeverityCritical: 15680580, SeverityWarning: 16096779, SeverityInfo: 7434609}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type discordEmbed struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	URL         string         `json:"url,omitempty"`
	Color       int            `json:"color"`
	Fields      []discordField `json:"fields,omitempty"`
	Timestamp   string         `json:"timestamp"`
}

func discord(m Message) (Rendered, error) {
	e := discordEmbed{
		Title: truncate(m.Title, 256), URL: m.URL, Color: discordColors[m.Severity],
		Timestamp: m.SentAt.UTC().Format(time.RFC3339),
	}
	desc := m.Body
	for _, f := range m.Fields {
		e.Fields = append(e.Fields, discordField{Name: truncate(f.Name, 256), Value: truncate(f.Value, 1024), Inline: true})
	}
	if m.Digest != nil {
		items := append(append([]DigestItem{}, m.Digest.Overdue...), m.Digest.DueSoon...)
		const maxFields = 20
		for i, it := range items {
			if i == maxFields {
				if m.MoreText != nil {
					desc += "\n" + m.MoreText(len(items)-maxFields)
				}
				break
			}
			e.Fields = append(e.Fields, discordField{Name: truncate(it.Vehicle.Name+" · "+it.Name, 256), Value: truncate(it.Line, 1024)})
		}
	}
	if len(e.Fields) > 25 {
		e.Fields = e.Fields[:25]
	}
	e.Description = truncate(desc, 4096)
	return jsonRendered(map[string]any{"username": "torq", "embeds": []discordEmbed{e}})
}

// --- ntfy --------------------------------------------------------------------

var ntfyPriority = map[string]string{SeverityInfo: "3", SeverityWarning: "4", SeverityCritical: "5"}
var ntfyTags = map[string]string{SeverityInfo: "wrench", SeverityWarning: "warning", SeverityCritical: "rotating_light"}

// headerText RFC 2047-encodes non-ASCII values so they survive HTTP headers.
func headerText(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 || s[i] < 0x20 {
			return mime.BEncoding.Encode("UTF-8", s)
		}
	}
	return s
}

func ntfy(m Message) Rendered {
	body := m.Body
	if m.Digest != nil {
		var lines []string
		for _, it := range append(append([]DigestItem{}, m.Digest.Overdue...), m.Digest.DueSoon...) {
			lines = append(lines, "• "+it.Vehicle.Name+" · "+it.Name+": "+it.Line)
		}
		if len(lines) > 0 {
			body += "\n" + strings.Join(lines, "\n")
		}
	}
	h := map[string]string{
		"Content-Type": "text/plain; charset=utf-8",
		"Title":        headerText(m.Title),
		"Priority":     ntfyPriority[m.Severity],
		"Tags":         ntfyTags[m.Severity],
	}
	if m.URL != "" {
		h["Click"] = m.URL
	}
	return Rendered{Headers: h, Body: []byte(body)}
}

// --- Gotify ------------------------------------------------------------------

var gotifyPriority = map[string]int{SeverityInfo: 2, SeverityWarning: 5, SeverityCritical: 8}

func gotify(m Message) (Rendered, error) {
	body := m.Body
	if m.Digest != nil {
		for _, it := range append(append([]DigestItem{}, m.Digest.Overdue...), m.Digest.DueSoon...) {
			body += "\n- **" + it.Vehicle.Name + " · " + it.Name + "**: " + it.Line
		}
	}
	extras := map[string]any{"client::display": map[string]string{"contentType": "text/markdown"}}
	if m.URL != "" {
		extras["client::notification"] = map[string]any{"click": map[string]string{"url": m.URL}}
	}
	return jsonRendered(map[string]any{"title": m.Title, "message": body, "priority": gotifyPriority[m.Severity], "extras": extras})
}
