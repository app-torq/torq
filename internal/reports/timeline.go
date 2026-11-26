package reports

import (
	"context"
	"database/sql"

	"github.com/app-torq/torq/internal/store"
	"github.com/app-torq/torq/internal/vehicles"
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
	// OdometerRecorded is false for entries of a vehicle without an odometer.
	OdometerRecorded bool
	MeterReadings    []vehicles.HoursReading
}

// Timeline merges service logs (by service_date) and completed repairs (by
// completed_on), newest first.
//
// Kinds the reader's role doesn't see are left out before paging (#49).
func (s *Service) Timeline(ctx context.Context, vehicleID int64, from, to string, limit, offset int64, kinds TimelineKinds) ([]TimelineItem, int64, error) {
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
	if !kinds.Services {
		logs = nil
	}
	if !kinds.Repairs {
		reps = nil
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
				CategoryIDs: cats[l.ID], TotalCents: totals[l.ID] + l.ExtraCostCents, OdometerRecorded: l.OdometerRecorded == 1})
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
				OdometerKm: r.CompletionOdometerKm.Int64, Title: r.Title, TotalCents: byID[r.ID], OdometerRecorded: r.CompletionOdometerRecorded == 1})
		}
	}

	if err := attachTimelineReadings(ctx, q, services, repairsOut); err != nil {
		return nil, 0, err
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

// TimelineKinds says which kinds of entries the timeline contains.
type TimelineKinds struct{ Services, Repairs bool }

// AllKinds is a timeline with every kind of entry.
var AllKinds = TimelineKinds{Services: true, Repairs: true}

func newer(a, b TimelineItem) bool {
	if a.Date != b.Date {
		return a.Date > b.Date
	}
	return a.OdometerKm >= b.OdometerKm
}

// attachTimelineReadings adds the hour meter readings of the entries.
func attachTimelineReadings(ctx context.Context, q *store.Queries, services, repairs []TimelineItem) error {
	ids := func(items []TimelineItem) []sql.NullInt64 {
		out := make([]sql.NullInt64, len(items))
		for i, it := range items {
			out[i] = sql.NullInt64{Int64: it.ID, Valid: true}
		}
		return out
	}
	if len(services) > 0 {
		rows, err := q.ListServiceLogMeterReadings(ctx, ids(services))
		if err != nil {
			return err
		}
		by := map[int64][]vehicles.HoursReading{}
		for _, r := range rows {
			by[r.ServiceLogID.Int64] = append(by[r.ServiceLogID.Int64], vehicles.HoursReading{MeterID: r.HourMeterID, HoursTenths: r.HoursTenths})
		}
		for i := range services {
			services[i].MeterReadings = by[services[i].ID]
		}
	}
	if len(repairs) > 0 {
		rows, err := q.ListRepairMeterReadings(ctx, ids(repairs))
		if err != nil {
			return err
		}
		by := map[int64][]vehicles.HoursReading{}
		for _, r := range rows {
			by[r.RepairLogID.Int64] = append(by[r.RepairLogID.Int64], vehicles.HoursReading{MeterID: r.HourMeterID, HoursTenths: r.HoursTenths})
		}
		for i := range repairs {
			repairs[i].MeterReadings = by[repairs[i].ID]
		}
	}
	return nil
}
