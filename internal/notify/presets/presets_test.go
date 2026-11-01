package presets

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite golden files")

func i64(v int64) *int64   { return &v }
func str(s string) *string { return &s }

type locale struct {
	name, dueSoon, overdue, digest, test, testBody, more, vehicle, odometer string
	km                                                                      func(int64) string
}

var locales = []locale{
	{"en", "Due soon: Engine Oil & Filter — Mercedes-Benz C 220 CDI", "Overdue: Engine Oil & Filter — Mercedes-Benz C 220 CDI",
		"Weekly maintenance digest", "torq test notification", "This is a test from torq.", "+%d more", "Vehicle", "Odometer",
		func(n int64) string { return fmt.Sprintf("%d km", n) }},
	{"de", "Bald fällig: Motoröl & Filter — Mercedes-Benz C 220 CDI", "Überfällig: Motoröl & Filter — Mercedes-Benz C 220 CDI",
		"Wöchentliche Wartungsübersicht", "torq-Testbenachrichtigung", "Dies ist ein Test von torq.", "+%d weitere", "Fahrzeug", "Kilometerstand",
		func(n int64) string { return fmt.Sprintf("%d km", n) }},
}

func messages(l locale) map[string]Message {
	at := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	veh := &Vehicle{ID: 1, Name: "Mercedes-Benz C 220 CDI", LicensePlate: str("B-TQ 646"), CurrentOdometerKm: 225350}
	rem := func(state string, remaining int64) *Reminder {
		return &Reminder{ID: 7, Name: "Engine Oil & Filter", State: state, Trigger: str("km"), NextDueKm: i64(225000),
			NextDueDate: str("2027-10-15"), RemainingKm: i64(remaining), RemainingDays: i64(350)}
	}
	fields := []Field{{l.vehicle, veh.Name}, {l.odometer, l.km(225350)}}
	items := func(n int) []DigestItem {
		out := []DigestItem{}
		for i := 0; i < n; i++ {
			out = append(out, DigestItem{Reminder: *rem("overdue", -350), Vehicle: *veh, Line: "-350 km"})
		}
		return out
	}
	more := func(n int) string { return fmt.Sprintf(l.more, n) }
	return map[string]Message{
		"reminder_due_soon": {Event: "reminder_due_soon", Severity: SeverityWarning, Title: l.dueSoon, Body: "650 km", URL: "https://torq.example/vehicles/1/reminders", Reminder: rem("due_soon", 650), Vehicle: veh, Fields: fields, SentAt: at},
		"reminder_overdue":  {Event: "reminder_overdue", Severity: SeverityCritical, Title: l.overdue, Body: "-350 km", URL: "https://torq.example/vehicles/1/reminders", Reminder: rem("overdue", -350), Vehicle: veh, Fields: fields, SentAt: at},
		"digest_empty":      {Event: "digest_weekly", Severity: SeverityInfo, Title: l.digest, Body: "—", Digest: &Digest{Period: "weekly", Overdue: items(0), DueSoon: items(0)}, MoreText: more, SentAt: at},
		"digest_25":         {Event: "digest_weekly", Severity: SeverityInfo, Title: l.digest, Body: "25", URL: "https://torq.example/reminders", Digest: &Digest{Period: "weekly", Overdue: items(25), DueSoon: items(0)}, MoreText: more, SentAt: at},
		"test":              {Event: "test", Severity: SeverityInfo, Title: l.test, Body: l.testBody, Vehicle: veh, SentAt: at},
	}
}

func render(t *testing.T, preset string, m Message) []byte {
	t.Helper()
	r, err := Render(preset, m)
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(r.Headers))
	for k := range r.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for _, k := range keys {
		fmt.Fprintf(&buf, "%s: %s\n", k, r.Headers[k])
	}
	buf.WriteString("\n")
	if strings.HasPrefix(r.Headers["Content-Type"], "application/json") {
		var v any
		if err := json.Unmarshal(r.Body, &v); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		pretty, _ := json.MarshalIndent(v, "", "  ")
		buf.Write(pretty)
	} else {
		buf.Write(r.Body)
	}
	buf.WriteString("\n")
	return buf.Bytes()
}

func TestGolden(t *testing.T) {
	for _, l := range locales {
		for event, m := range messages(l) {
			for _, preset := range []string{Discord, Ntfy, Gotify, HomeAssistant, GenericJSON} {
				name := fmt.Sprintf("%s_%s_%s.golden", preset, event, l.name)
				got := render(t, preset, m)
				path := filepath.Join("testdata", name)
				if *update {
					if err := os.WriteFile(path, got, 0o600); err != nil {
						t.Fatal(err)
					}
					continue
				}
				want, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("%s: %v (run go test -update)", name, err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("%s differs:\n%s", name, got)
				}
			}
		}
	}
}

func TestPresetRules(t *testing.T) {
	m := messages(locales[1])
	// Discord: 20 digest fields max plus "+N more".
	r, _ := Render(Discord, m["digest_25"])
	var d struct {
		Embeds []struct {
			Description string
			Fields      []json.RawMessage
			Color       int
		}
	}
	_ = json.Unmarshal(r.Body, &d)
	if len(d.Embeds[0].Fields) != 20 || !strings.Contains(d.Embeds[0].Description, "+5 weitere") {
		t.Fatalf("discord digest: %d fields, desc %q", len(d.Embeds[0].Fields), d.Embeds[0].Description)
	}
	long := m["test"]
	long.Title = strings.Repeat("x", 400)
	long.Body = strings.Repeat("y", 5000)
	r, _ = Render(Discord, long)
	_ = json.Unmarshal(r.Body, &d)
	// ntfy: non-ASCII title is RFC 2047 encoded; Click only with a URL; priority by severity.
	n, _ := Render(Ntfy, m["reminder_overdue"])
	if !strings.HasPrefix(n.Headers["Title"], "=?UTF-8?b?") || n.Headers["Priority"] != "5" || n.Headers["Click"] == "" {
		t.Fatalf("ntfy headers %v", n.Headers)
	}
	n, _ = Render(Ntfy, m["test"])
	if _, ok := n.Headers["Click"]; ok || n.Headers["Priority"] != "3" {
		t.Fatalf("ntfy test headers %v", n.Headers)
	}
	// Gotify priorities.
	g, _ := Render(Gotify, m["reminder_due_soon"])
	if !bytes.Contains(g.Body, []byte(`"priority":5`)) {
		t.Fatalf("gotify %s", g.Body)
	}
	// Generic JSON schema and omitted URL.
	j, _ := Render(GenericJSON, m["test"])
	if !bytes.Contains(j.Body, []byte(`"schema":"torq.notification/v1"`)) || bytes.Contains(j.Body, []byte(`"url"`)) {
		t.Fatalf("generic %s", j.Body)
	}
}
