package specfill

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakes serves Tavily under /search and Gemini under /models/… and records
// what torq sent.
type fakes struct {
	mu           sync.Mutex
	queries      []string
	tavilyAuth   string
	gemini       map[string]any
	tavilyStatus int
	results      map[string][]Source // by query substring
	answer       string
	rawQueries   []string // searches that asked for whole pages
	overloaded   int      // Gemini answers 503 this many times first
	geminiCalls  int      // extraction requests
	names        []string // the engine-name lookup's answer
}

func newFakes(t *testing.T, f *fakes) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case r.URL.Path == "/search":
			f.tavilyAuth = r.Header.Get("Authorization")
			var in struct {
				Query string `json:"query"`
				Raw   string `json:"include_raw_content"`
			}
			_ = json.Unmarshal(b, &in)
			f.queries = append(f.queries, in.Query)
			if in.Raw == "markdown" {
				f.rawQueries = append(f.rawQueries, in.Query)
			}
			if f.tavilyStatus != 0 {
				w.WriteHeader(f.tavilyStatus)
				_, _ = w.Write([]byte(`{"detail":{"error":"This request exceeds your plan's set usage limit."}}`))
				return
			}
			var results []Source
			for sub, rs := range f.results {
				if strings.Contains(in.Query, sub) {
					results = append(results, rs...)
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"query": in.Query, "results": results})
		case r.URL.Path == "/models/"+DefaultModel+":generateContent" && r.Header.Get("x-goog-api-key") == "gemini-key" &&
			strings.Contains(string(b), "You help search the web"):
			_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []map[string]any{{
				"content": map[string]any{"parts": []map[string]string{{"text": `{"names": ` + mustJSON(f.names) + `}`}}},
			}}})
		case r.URL.Path == "/models/"+DefaultModel+":generateContent" && r.Header.Get("x-goog-api-key") == "gemini-key":
			f.geminiCalls++
			if f.geminiCalls <= f.overloaded {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":{"code":503,"message":"This model is currently experiencing high demand."}}`))
				return
			}
			_ = json.Unmarshal(b, &f.gemini)
			_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []map[string]any{{
				"content": map[string]any{"parts": []map[string]string{{"text": f.answer}}},
			}}})
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":404,"message":"not found"}}`))
		}
	}))
	t.Cleanup(srv.Close)
	c := New("gemini-key", "tavily-key", "")
	c.BaseURL, c.TavilyURL = srv.URL, srv.URL+"/search"
	c.RetryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	return c
}

var w203 = Vehicle{Make: "Mercedes-Benz", Model: "C-Class", ChassisCode: "S203", ModelYear: 2006, EngineCode: "OM646", HSN: "0710", TSN: "472"}

func TestLookupExtractsOnlyStatedValuesFromSearchResults(t *testing.T) {
	f := &fakes{
		results: map[string][]Source{
			"Motoröl": {
				{Title: "W203 Ölwechsel", URL: "https://www.carwiki.de/mercedes/w203/oel", Content: "OM646: 6,5 Liter Motoröl SAE 5W30 nach MB-Freigabe 229.51. Bremsflüssigkeit DOT 4 plus."},
				{Title: "Kühlmittel", URL: "https://example.org/kuehlmittel", Content: "Kühlmittel nach MB 325.0, Füllmenge 8,5 l."},
			},
			"Reifendruck": {
				{Title: "Reifendruck C-Klasse", URL: "https://www.adac.de/reifendruck/c-klasse/", Content: "Vorne 2,3 bar, hinten 2,5 bar. Radschrauben mit 110 Nm anziehen."},
				// Duplicates across both searches are sent to Gemini once.
				{Title: "W203 Ölwechsel", URL: "https://www.carwiki.de/mercedes/w203/oel", Content: "OM646: 6,5 Liter Motoröl SAE 5W30 nach MB-Freigabe 229.51. Bremsflüssigkeit DOT 4 plus."},
			},
		},
		answer: `{
		  "engine_oil_viscosity": {"value": "5W-30", "source_url": "https://www.carwiki.de/mercedes/w203/oel", "source_name": "carwiki.de"},
		  "engine_oil_spec": {"value": "MB 229.51", "source_url": "https://www.carwiki.de/mercedes/w203/oel", "source_name": "carwiki.de"},
		  "engine_oil_capacity_l": {"value": 6.5, "source_url": "https://www.carwiki.de/mercedes/w203/oel", "source_name": "carwiki.de"},
		  "transmission_fluid_spec": {"value": "MB 236.14", "source_url": "https://www.carwiki.de/mercedes/w203/oel", "source_name": "carwiki.de"},
		  "transmission_fluid_capacity_l": {"value": 7.5, "source_url": "https://www.carwiki.de/mercedes/w203/getriebe", "source_name": "carwiki.de"},
		  "differential_fluid_spec": {"value": null, "source_url": null, "source_name": null},
		  "brake_fluid_spec": {"value": "DOT 4 plus", "source_url": "https://www.carwiki.de/mercedes/w203/oel", "source_name": "carwiki.de"},
		  "coolant_spec": {"value": "MB 325.0", "source_url": "https://example.org/kuehlmittel", "source_name": "example.org"},
		  "coolant_capacity_l": {"value": 85, "source_url": "https://example.org/kuehlmittel", "source_name": "example.org"},
		  "wheel_lug_torque_nm": {"value": 110, "source_url": "https://www.adac.de/reifendruck/c-klasse", "source_name": "adac.de"},
		  "tire_pressure_front_bar": {"value": "2,3 bar", "source_url": "https://www.adac.de/reifendruck/c-klasse/", "source_name": "adac.de"},
		  "tire_pressure_rear_bar": {"value": 2.8, "source_url": "https://www.adac.de/reifendruck/c-klasse/", "source_name": "adac.de"}
		}`,
	}
	c := newFakes(t, f)

	got, err := c.Lookup(context.Background(), w203)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(fields) {
		t.Fatalf("want every key, got %d", len(got))
	}
	check := func(key string, value any, url, name string) {
		t.Helper()
		g := got[key]
		if g.Value != value || g.SourceURL == nil || *g.SourceURL != url || *g.SourceName != name {
			t.Errorf("%s = %#v %v %v", key, g.Value, deref(g.SourceURL), deref(g.SourceName))
		}
	}
	const oil, adac = "https://www.carwiki.de/mercedes/w203/oel", "https://www.adac.de/reifendruck/c-klasse/"
	check("engine_oil_viscosity", "5W-30", oil, "carwiki.de") // "5W30" in the text
	check("engine_oil_spec", "MB 229.51", oil, "carwiki.de")  // "MB-Freigabe 229.51"
	check("engine_oil_capacity_l", 6.5, oil, "carwiki.de")    // "6,5 Liter"
	check("brake_fluid_spec", "DOT 4 plus", oil, "carwiki.de")
	check("coolant_spec", "MB 325.0", "https://example.org/kuehlmittel", "example.org")
	check("wheel_lug_torque_nm", 110.0, adac, "adac.de")   // cited without the trailing slash
	check("tire_pressure_front_bar", 2.3, adac, "adac.de") // unit stripped
	for key, why := range map[string]string{
		"transmission_fluid_spec":       "is not written in the cited result",
		"transmission_fluid_capacity_l": "cites a URL that was no search result",
		"differential_fluid_spec":       "was not found",
		"coolant_capacity_l":            "is out of range",
		"tire_pressure_rear_bar":        "differs from the cited result (2,5 bar)",
		"brake_fluid_capacity_l":        "is missing from the answer",
	} {
		if g := got[key]; g.Value != nil || g.SourceURL != nil {
			t.Errorf("%s %s but was kept: %#v", key, why, g)
		}
	}

	// One Tavily search per topic, with the vehicle and the bearer key.
	all := strings.Join(f.queries, " | ")
	if len(f.queries) != 4 || f.tavilyAuth != "Bearer tavily-key" || strings.Count(all, "Mercedes-Benz C-Class S203 2006 OM646") != 4 {
		t.Fatalf("searches %q auth %q", f.queries, f.tavilyAuth)
	}
	for _, topic := range []string{"Motoröl", "Getriebeöl", "Kühlmittel", "Reifendruck"} {
		if !strings.Contains(all, topic) {
			t.Errorf("no search for %s: %q", topic, f.queries)
		}
	}
	// Gemini gets the results in the prompt, no search tool, and a JSON schema.
	if _, ok := f.gemini["tools"]; ok {
		t.Error("Gemini request must not use tools")
	}
	cfg, _ := json.Marshal(f.gemini["generationConfig"])
	contents, _ := json.Marshal(f.gemini["contents"])
	if !strings.Contains(string(cfg), `"responseMimeType":"application/json"`) || !strings.Contains(string(cfg), `"tire_pressure_rear_bar"`) {
		t.Errorf("generation config %s", cfg)
	}
	prompt := string(contents)
	if !strings.Contains(prompt, "URL: https://www.carwiki.de/mercedes/w203/oel") || !strings.Contains(prompt, "Radschrauben mit 110 Nm") ||
		strings.Count(prompt, "URL: https://www.carwiki.de/mercedes/w203/oel") != 1 || !strings.Contains(prompt, "0710/472") {
		t.Errorf("prompt %s", prompt)
	}
}

func TestLookupErrors(t *testing.T) {
	ctx := context.Background()

	c := newFakes(t, &fakes{results: map[string][]Source{}})
	if _, err := c.Lookup(ctx, w203); !errors.Is(err, ErrNoSources) {
		t.Fatalf("no results: %v", err)
	}

	c = newFakes(t, &fakes{tavilyStatus: 432})
	var search *SearchError
	if _, err := c.Lookup(ctx, w203); !errors.As(err, &search) || search.Status != 432 || !strings.Contains(search.Message, "usage limit") {
		t.Fatalf("tavily limit: %v", err)
	}

	src := map[string][]Source{"Motoröl": {{Title: "x", URL: "https://carwiki.de/x", Content: "5W-30"}}}
	c = newFakes(t, &fakes{results: src, answer: "Sorry, nothing found."})
	if _, err := c.Lookup(ctx, w203); err == nil || !strings.Contains(err.Error(), "no JSON") {
		t.Fatalf("prose answer: %v", err)
	}

	c = newFakes(t, &fakes{results: src, answer: "{}"})
	c.Model = "gemini-2.5-flash"
	var api *APIError
	if _, err := c.Lookup(ctx, w203); !errors.As(err, &api) || api.Status != http.StatusNotFound {
		t.Fatalf("retired model: %v", err)
	}
	if New("g", "t", "").Model != DefaultModel || New("g", "t", "gemini-9").Model != "gemini-9" {
		t.Fatal("model default or override")
	}
}

func TestLookupTimeout(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer slow.Close()
	c := New("g", "t", "")
	c.TavilyURL, c.Timeout = slow.URL, 50*time.Millisecond
	if _, err := c.Lookup(context.Background(), w203); !errors.Is(err, ErrTimeout) {
		t.Fatalf("slow upstream: %v", err)
	}
	if New("g", "t", "").Timeout != DefaultTimeout {
		t.Fatal("default timeout")
	}
}

func TestTyrePressureRanges(t *testing.T) {
	f := &fakes{
		results: map[string][]Source{"Reifendruck": {
			{Title: "C 220 CDI (phase II) 110 kW Sedan 2004-", URL: "https://www.tpressure.com/w203", Content: "195/65R 15 91V | 2.2-2.4 bar 32-35 psi | 2.2-2.9 bar 32-42 psi"},
			{Title: "Reifendruck", URL: "https://carwiki.de/c", Content: "Vorne 2,3 bar."},
		}},
		answer: `{
		  "tire_pressure_front_bar": {"value": 2.2, "value_max": 2.4, "source_url": "https://www.tpressure.com/w203", "source_name": "tpressure.com"},
		  "tire_pressure_rear_bar": {"value": 2.2, "value_max": 3.1, "source_url": "https://www.tpressure.com/w203", "source_name": "tpressure.com"}
		}`,
	}
	got, err := newFakes(t, f).Lookup(context.Background(), w203)
	if err != nil {
		t.Fatal(err)
	}
	if front := got["tire_pressure_front_bar"]; front.Value != 2.2 || front.ValueMax == nil || *front.ValueMax != 2.4 {
		t.Errorf("stated range %+v", front)
	}
	// 3.1 is not the range the source writes: only the stated 2.2 bar is kept.
	if rear := got["tire_pressure_rear_bar"]; rear.Value != 2.2 || rear.ValueMax != nil {
		t.Errorf("invented maximum %+v", rear)
	}
	schema, _ := json.Marshal(f.gemini["generationConfig"])
	if !strings.Contains(string(schema), `"value_max"`) {
		t.Errorf("schema without value_max: %s", schema)
	}

	for _, c := range []struct {
		text   string
		lo, hi float64
		want   bool
	}{
		{"2.2-2.4 bar 32-35 psi", 2.2, 2.4, true},
		{"vorne 2,2 – 2,4 bar", 2.2, 2.4, true},
		{"2,2 bis 2,4 bar", 2.2, 2.4, true},
		{"2.2-2.9 bar", 2.2, 2.4, false},
		{"32-35 psi", 32, 35, false},
		{"2.2 bar vorne, 2.4 bar hinten", 2.2, 2.4, false},
	} {
		if got := statesRange(c.text, c.lo, c.hi, bars); got != c.want {
			t.Errorf("statesRange(%q, %v, %v) = %v", c.text, c.lo, c.hi, got)
		}
	}
}

func TestGeminiOverloadRetries(t *testing.T) {
	src := map[string][]Source{"Motoröl": {{Title: "x", URL: "https://carwiki.de/x", Content: "SAE 5W-30"}}}
	answer := `{"engine_oil_viscosity": {"value": "5W-30", "source_url": "https://carwiki.de/x", "source_name": "carwiki.de"}}`

	f := &fakes{results: src, answer: answer, overloaded: 2}
	got, err := newFakes(t, f).Lookup(context.Background(), w203)
	if err != nil || got["engine_oil_viscosity"].Value != "5W-30" || f.geminiCalls != 3 {
		t.Fatalf("two 503s then success: %v, %d calls", err, f.geminiCalls)
	}

	f = &fakes{results: src, answer: answer, overloaded: 99}
	var api *APIError
	if _, err := newFakes(t, f).Lookup(context.Background(), w203); !errors.As(err, &api) || api.Status != http.StatusServiceUnavailable || f.geminiCalls != 3 {
		t.Fatalf("still overloaded: %v, %d calls", err, f.geminiCalls)
	}
}

func TestTyreSearchUsesWholePages(t *testing.T) {
	block := "Mercedes-Benz C 220 CDI (phase II) 110 kW Sedan 2004- | 195/65R 15 91V | 2.2-2.4 bar | 2.2-2.9 bar"
	f := &fakes{
		names: []string{"C 220 CDI", "220 CDI"},
		results: map[string][]Source{"Reifendruck": {
			{Title: "W203 tyre pressure", URL: "https://www.tpressure.com/w203", Content: "C 200 Kompressor (phase II) 2.1-2.9 bar", RawContent: "# W203\n" + strings.Repeat("other variants | ", 2000) + block + strings.Repeat(" | more variants", 2000)},
		}},
		answer: `{"tire_pressure_front_bar": {"value": 2.2, "value_max": 2.4, "source_url": "https://www.tpressure.com/w203", "source_name": "tpressure.com"}}`,
	}
	got, err := newFakes(t, f).Lookup(context.Background(), w203)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.rawQueries) != 1 || !strings.Contains(f.rawQueries[0], "Reifendruck") {
		t.Fatalf("whole pages requested for %q", f.rawQueries)
	}
	// The engine name aims every search; only the passage naming it reaches the prompt.
	for _, q := range f.queries {
		if !strings.Contains(q, "C 220 CDI") {
			t.Errorf("search without engine name: %q", q)
		}
	}
	contents, _ := json.Marshal(f.gemini["contents"])
	if !strings.Contains(string(contents), "C 220 CDI (phase II)") || strings.Count(string(contents), "other variants") > 100 {
		t.Errorf("prompt has %d chars, block present: %v", len(contents), strings.Contains(string(contents), "C 220 CDI (phase II)"))
	}
	// The range is verified against the whole page, not the excerpt.
	if p := got["tire_pressure_front_bar"]; p.Value != 2.2 || p.ValueMax == nil || *p.ValueMax != 2.4 {
		t.Errorf("range %+v", p)
	}
}

func TestStates(t *testing.T) {
	for _, c := range []struct {
		text  string
		value any
		unit  *regexp.Regexp
		want  bool
	}{
		{"Füllmenge 6,5 l", 6.5, litres, true},
		{"Füllmenge 6.5 Liter", 6.5, litres, true},
		{"Inhalt [Liter]: 8; SAE 5W-30", 8.0, litres, true},
		{"Füllmenge 6,8 l", 6.5, litres, false},
		{"Ölwechsel nach 6.5 Jahren", 6.5, litres, false},
		{"| 2.1-2.3 bar 30-33 psi |", 2.1, bars, true},
		{"vorne 2,5 und hinten 3,1", 2.5, bars, false},
		{"Bauteil 2.3 im Schema", 2.3, bars, false},
		{"mit 110 Nm anziehen", 110.0, newton, true},
		{"Radschrauben 110nm", 110.0, newton, true},
		{"110 PS Diesel", 110.0, newton, false},
		{"SAE 5W30", "5W-30", nil, true},
		{"MB-Freigabe 229.51", "MB 229.51", nil, true},
		{"MB 229.31", "MB 229.51", nil, false},
		{"DOT4", "DOT 4", nil, true},
		{"G13", "G 12", nil, false},
	} {
		if got := states(c.text, c.value, c.unit); got != c.want {
			t.Errorf("states(%q, %v) = %v", c.text, c.value, got)
		}
	}
}

func mustJSON(v any) string {
	if v == nil {
		return "[]"
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func deref(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}
