package specfill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DefaultTavilyURL is the Tavily Search API endpoint.
const DefaultTavilyURL = "https://api.tavily.com/search"

// SearchError is an error answer of the Tavily API: 401 for a bad key,
// 429 for too many requests, 432/433 when the plan's credits are used up.
type SearchError struct {
	Status  int
	Message string
}

func (e *SearchError) Error() string {
	return fmt.Sprintf("specfill: tavily: HTTP %d: %s", e.Status, e.Message)
}

// Source is one Tavily search result: a web page and the passage of it that
// matched the query.
type Source struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// search runs one Tavily query. "advanced" depth returns the passages most
// relevant to the query (2 credits per query on the free plan).
func (c *Client) search(ctx context.Context, query string) ([]Source, error) {
	body, err := json.Marshal(map[string]any{
		"query":             query,
		"search_depth":      "advanced",
		"chunks_per_source": 3,
		"max_results":       6,
		"include_answer":    false,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TavilyURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.TavilyKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("specfill: tavily: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("specfill: tavily: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Detail any `json:"detail"`
		}
		_ = json.Unmarshal(raw, &e)
		msg := fmt.Sprint(e.Detail)
		if e.Detail == nil {
			msg = strings.TrimSpace(string(raw))
		}
		return nil, &SearchError{Status: resp.StatusCode, Message: msg}
	}
	var out struct {
		Results []Source `json:"results"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("specfill: tavily: %w", err)
	}
	return out.Results, nil
}

// queries are the searches for one vehicle: fluids and capacities, then
// wheels and tyres. German terms, because most users and sources for these
// values (KBA vehicles, German manuals and shops) are German.
func queries(v Vehicle) []string {
	name := vehicleName(v)
	return []string{
		name + " Motoröl Freigabe Viskosität Füllmenge Getriebeöl Kühlmittel Bremsflüssigkeit",
		name + " Reifendruck Anzugsmoment Radschrauben Nm",
	}
}

// vehicleName is the vehicle as a search phrase, e.g. "Mercedes-Benz
// C-Klasse S203 2006 OM646".
func vehicleName(v Vehicle) string {
	parts := []string{v.Make, v.Model, v.ChassisCode}
	if v.ModelYear != 0 {
		parts = append(parts, fmt.Sprint(v.ModelYear))
	}
	parts = append(parts, v.EngineCode)
	var kept []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, " ")
}
