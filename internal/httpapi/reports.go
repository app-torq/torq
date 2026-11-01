package httpapi

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/i18n"
	"github.com/marekbeckmann/torq/internal/reports"
)

type costsJSON struct {
	TotalCents     int64            `json:"total_cents"`
	BySource       map[string]int64 `json:"by_source"`
	ByCostType     map[string]int64 `json:"by_cost_type"`
	DistanceKm     *int64           `json:"distance_km"`
	CostPerKmCents *float64         `json:"cost_per_km_cents"`
}

func toCostsJSON(c reports.Costs) costsJSON {
	return costsJSON{
		TotalCents: c.TotalCents,
		BySource:   map[string]int64{"routine_service_cents": c.RoutineServiceCents, "self_repair_cents": c.SelfRepairCents},
		ByCostType: map[string]int64{"parts_cents": c.PartsCents, "fluids_cents": c.FluidsCents, "other_cents": c.OtherCents},
		DistanceKm: c.DistanceKm, CostPerKmCents: c.CostPerKmCents(),
	}
}

func (s *Server) costReport(w http.ResponseWriter, r *http.Request) error {
	vehicleID, err := queryInt(r, "vehicle_id")
	if err != nil {
		return err
	}
	q := r.URL.Query()
	rep, err := s.reports.CostReport(r.Context(), vehicleID, q.Get("from"), q.Get("to"))
	if err != nil {
		return err
	}
	type vehicleCosts struct {
		VehicleID int64 `json:"vehicle_id"`
		costsJSON
	}
	body := struct {
		From string `json:"from"`
		To   string `json:"to"`
		costsJSON
		ByVehicle []vehicleCosts `json:"by_vehicle,omitempty"`
	}{From: rep.From, To: rep.To, costsJSON: toCostsJSON(rep.Costs)}
	if vehicleID == 0 {
		body.ByVehicle = []vehicleCosts{}
		for _, v := range rep.ByVehicle {
			body.ByVehicle = append(body.ByVehicle, vehicleCosts{VehicleID: v.VehicleID, costsJSON: toCostsJSON(v.Costs)})
		}
	}
	writeJSON(w, http.StatusOK, body)
	return nil
}

type timelineJSON struct {
	Kind        string  `json:"kind"`
	ID          int64   `json:"id"`
	Date        string  `json:"date"`
	OdometerKm  int64   `json:"odometer_km"`
	Title       string  `json:"title,omitempty"`
	CategoryIDs []int64 `json:"category_ids,omitempty"`
	TotalCents  int64   `json:"total_cents"`
}

func (s *Server) timeline(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	limit, offset, err := pagination(r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	items, total, err := s.reports.Timeline(r.Context(), id, q.Get("from"), q.Get("to"), limit, offset)
	if err != nil {
		return err
	}
	out := make([]timelineJSON, 0, len(items))
	for _, it := range items {
		out = append(out, timelineJSON{Kind: it.Kind, ID: it.ID, Date: it.Date, OdometerKm: it.OdometerKm, Title: it.Title, CategoryIDs: it.CategoryIDs, TotalCents: it.TotalCents})
	}
	writeJSON(w, http.StatusOK, newList(out, total))
	return nil
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	s = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss", "é", "e", "è", "e").Replace(strings.ToLower(s))
	return strings.Trim(slugRe.ReplaceAllString(s, "-"), "-")
}

func boolQuery(r *http.Request, key string, def bool) (bool, error) {
	switch r.URL.Query().Get(key) {
	case "":
		return def, nil
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	}
	return false, apperr.BadRequest("request.malformed")
}

func (s *Server) history(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	costs, err := boolQuery(r, "include_costs", true)
	if err != nil {
		return err
	}
	notes, err := boolQuery(r, "include_notes", true)
	if err != nil {
		return err
	}
	u := currentUser(r)
	h, err := s.reports.History(r.Context(), id, reports.ExportOptions{
		IncludeCosts: costs, IncludeNotes: notes,
		Locale: i18n.Locale{Lang: u.Prefs.Locale, DateFormat: u.Prefs.DateFormat, NumberFormat: u.Prefs.NumberFormat},
	})
	if err != nil {
		return err
	}
	base := "torq-" + slug(h.Vehicle.Make) + "-" + slug(h.Vehicle.Model) + "-" + s.Clock.Today()
	switch r.URL.Query().Get("format") {
	case "", "json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.URL.Query().Get("download") != "0" {
			w.Header().Set("Content-Disposition", `attachment; filename="`+base+`.json"`)
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(h)
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+base+`.csv"`)
		return reports.WriteCSV(w, h)
	default:
		return apperr.BadRequest("request.malformed")
	}
}
