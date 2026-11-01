// Package reports aggregates spending, builds the vehicle timeline and exports
// the vehicle history. It is the only package that reads both the Service
// Logbook and the Self-Repair Log (Constitution II), and it never writes.
package reports

import (
	"context"
	"sort"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/db"
	"github.com/marekbeckmann/torq/internal/repairs"
	"github.com/marekbeckmann/torq/internal/servicelog"
	"github.com/marekbeckmann/torq/internal/store"
	"github.com/marekbeckmann/torq/internal/vehicles"
)

// Service computes reports.
type Service struct {
	DB         *db.DB
	Vehicles   *vehicles.Service
	ServiceLog *servicelog.Service
	Repairs    *repairs.Service
}

// Range bounds; empty means open-ended.
const (
	minDate = "0000-01-01"
	maxDate = "9999-12-31"
)

func bounds(from, to string) (string, string, error) {
	var v apperr.Validation
	if from != "" {
		if _, ok := vehicles.ParseDate(from); !ok {
			v.Add("from", "validation.date")
		}
	} else {
		from = minDate
	}
	if to != "" {
		if _, ok := vehicles.ParseDate(to); !ok {
			v.Add("to", "validation.date")
		}
	} else {
		to = maxDate
	}
	if err := v.Err(); err != nil {
		return "", "", err
	}
	if from > to {
		return "", "", apperr.Unprocessable("report.invalid_range", nil)
	}
	return from, to, nil
}

// Costs is a spending breakdown in integer cents (FR-044, FR-045).
type Costs struct {
	TotalCents          int64
	RoutineServiceCents int64
	SelfRepairCents     int64
	PartsCents          int64
	FluidsCents         int64
	OtherCents          int64
	// DistanceKm is nil when fewer than two readings exist or the distance is zero.
	DistanceKm *int64
}

// CostPerKmCents returns fractional cents per km, or nil without distance.
func (c Costs) CostPerKmCents() *float64 {
	if c.DistanceKm == nil {
		return nil
	}
	v := float64(c.TotalCents) / float64(*c.DistanceKm)
	return &v
}

// VehicleCosts is one vehicle's share of a garage-wide report.
type VehicleCosts struct {
	VehicleID int64
	Costs
}

// CostReport is the result for a vehicle or the whole garage.
type CostReport struct {
	From, To string
	Costs
	ByVehicle []VehicleCosts // garage-wide only
}

// CostReport sums Routine Service (items + extra costs) and completed
// Self-Repair projects (quantity × unit cost) in [from, to]. vehicleID 0 means
// the whole garage; its distance is the sum of per-vehicle distances.
func (s *Service) CostReport(ctx context.Context, vehicleID int64, from, to string) (CostReport, error) {
	lo, hi, err := bounds(from, to)
	if err != nil {
		return CostReport{}, err
	}
	if vehicleID != 0 {
		if _, err := s.Vehicles.Get(ctx, vehicleID); err != nil {
			return CostReport{}, err
		}
	}
	q := store.New(s.DB.Reader)
	per := map[int64]*Costs{}
	get := func(id int64) *Costs {
		if per[id] == nil {
			per[id] = &Costs{}
		}
		return per[id]
	}
	addType := func(c *Costs, costType string, cents int64) {
		if costType == "fluids" {
			c.FluidsCents += cents
		} else {
			c.PartsCents += cents
		}
	}

	svc, err := q.ServiceCostsByType(ctx, store.ServiceCostsByTypeParams{VehicleID: vehicleID, FromDate: lo, ToDate: hi})
	if err != nil {
		return CostReport{}, err
	}
	for _, r := range svc {
		c := get(r.VehicleID)
		c.RoutineServiceCents += r.Cents
		addType(c, r.CostType, r.Cents)
	}
	extra, err := q.ServiceExtraCosts(ctx, store.ServiceExtraCostsParams{VehicleID: vehicleID, FromDate: lo, ToDate: hi})
	if err != nil {
		return CostReport{}, err
	}
	for _, r := range extra {
		c := get(r.VehicleID)
		c.RoutineServiceCents += r.Cents
		c.OtherCents += r.Cents
	}
	rep, err := q.RepairCostsByType(ctx, store.RepairCostsByTypeParams{VehicleID: vehicleID, FromDate: sqlStr(lo), ToDate: sqlStr(hi)})
	if err != nil {
		return CostReport{}, err
	}
	for _, r := range rep {
		c := get(r.VehicleID)
		c.SelfRepairCents += r.Cents
		addType(c, r.CostType, r.Cents)
	}
	ranges, err := q.ReadingRange(ctx, store.ReadingRangeParams{VehicleID: vehicleID, FromDate: lo, ToDate: hi})
	if err != nil {
		return CostReport{}, err
	}
	for _, r := range ranges {
		if r.Readings >= 2 && r.MaxKm > r.MinKm {
			d := r.MaxKm - r.MinKm
			get(r.VehicleID).DistanceKm = &d
		}
	}

	out := CostReport{From: from, To: to}
	ids := make([]int64, 0, len(per))
	for id := range per {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var distance int64
	hasDistance := false
	for _, id := range ids {
		c := per[id]
		c.TotalCents = c.RoutineServiceCents + c.SelfRepairCents
		out.TotalCents += c.TotalCents
		out.RoutineServiceCents += c.RoutineServiceCents
		out.SelfRepairCents += c.SelfRepairCents
		out.PartsCents += c.PartsCents
		out.FluidsCents += c.FluidsCents
		out.OtherCents += c.OtherCents
		if c.DistanceKm != nil {
			distance += *c.DistanceKm
			hasDistance = true
		}
		if vehicleID == 0 {
			out.ByVehicle = append(out.ByVehicle, VehicleCosts{VehicleID: id, Costs: *c})
		}
	}
	if hasDistance {
		out.DistanceKm = &distance
	}
	return out, nil
}
