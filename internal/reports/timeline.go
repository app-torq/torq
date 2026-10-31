package reports

import (
	"context"

	"github.com/marekbeckmann/torq/internal/store"
)

// Timeline kinds.
const (
	KindService = "service"
	KindRepair  = "repair"
)

// TimelineItem is one read-only entry: a service session or a completed repair (FR-046).
type TimelineItem struct {
	Kind        string
	ID          int64
	Date        string
	OdometerKm  int64
	Title       string  // repairs
	CategoryIDs []int64 // services
	TotalCents  int64
}

// Timeline merges service logs (by service_date) and completed repairs (by
// completed_on), newest first.
func (s *Service) Timeline(ctx context.Context, vehicleID int64, from, to string, limit, offset int64) ([]TimelineItem, int64, error) {
	lo, hi, err := bounds(from, to)
	if err != nil {
		return nil, 0, err
	}
	if _, err := s.Vehicles.Get(ctx, vehicleID); err != nil {
		return nil, 0, err
	}
	q := store.New(s.DB.Reader)
	logs, err := q.TimelineServiceLogs(ctx, store.TimelineServiceLogsParams{VehicleID: vehicleID, FromDate: lo, ToDate: hi})
	if err != nil {
		return nil, 0, err
	}
	reps, err := q.TimelineRepairs(ctx, store.TimelineRepairsParams{VehicleID: vehicleID, FromDate: sqlStr(lo), ToDate: sqlStr(hi)})
	if err != nil {
		return nil, 0, err
	}

	services := make([]TimelineItem, 0, len(logs))
	if len(logs) > 0 {
		ids := make([]int64, len(logs))
		for i, l := range logs {
			ids[i] = l.ID
		}
		items, err := q.ListServiceLogItemsForLogs(ctx, ids)
		if err != nil {
			return nil, 0, err
		}
		cats := map[int64][]int64{}
		totals := map[int64]int64{}
		for _, it := range items {
			cats[it.ServiceLogID] = append(cats[it.ServiceLogID], it.CategoryID)
			if it.CostCents.Valid {
				totals[it.ServiceLogID] += it.CostCents.Int64
			}
		}
		for _, l := range logs {
			services = append(services, TimelineItem{Kind: KindService, ID: l.ID, Date: l.ServiceDate, OdometerKm: l.OdometerKm,
				CategoryIDs: cats[l.ID], TotalCents: totals[l.ID] + l.ExtraCostCents})
		}
	}
	repairsOut := make([]TimelineItem, 0, len(reps))
	if len(reps) > 0 {
		ids := make([]int64, len(reps))
		for i, r := range reps {
			ids[i] = r.ID
		}
		totals, err := q.RepairTotals(ctx, ids)
		if err != nil {
			return nil, 0, err
		}
		byID := map[int64]int64{}
		for _, t := range totals {
			byID[t.RepairLogID] = t.TotalCents
		}
		for _, r := range reps {
			repairsOut = append(repairsOut, TimelineItem{Kind: KindRepair, ID: r.ID, Date: r.CompletedOn.String,
				OdometerKm: r.CompletionOdometerKm.Int64, Title: r.Title, TotalCents: byID[r.ID]})
		}
	}

	merged := make([]TimelineItem, 0, len(services)+len(repairsOut))
	i, j := 0, 0
	for i < len(services) || j < len(repairsOut) {
		if j >= len(repairsOut) || (i < len(services) && newer(services[i], repairsOut[j])) {
			merged = append(merged, services[i])
			i++
		} else {
			merged = append(merged, repairsOut[j])
			j++
		}
	}
	total := int64(len(merged))
	if offset >= total {
		return []TimelineItem{}, total, nil
	}
	end := min(offset+limit, total)
	return merged[offset:end], total, nil
}

func newer(a, b TimelineItem) bool {
	if a.Date != b.Date {
		return a.Date > b.Date
	}
	return a.OdometerKm >= b.OdometerKm
}
